import { chatBaseUrl } from '$lib/api';

export interface PRDChatMessage {
  type: string;
  text?: string;
  error?: string;
  final_content?: string;
  final_title?: string;
  final_prd_path?: string;
}

export interface PRDChatState {
  messages: PRDChatMessage[];
  finalContent: string | null;
  finalTitle: string | null;
  busy: boolean;
  starting: boolean;
}

export interface PRDChatSocket {
  send(payload: string): void;
  close(): void;
}

export interface PRDChatOptions {
  initialPrompt?: string;
  context?: Record<string, string>;
}

const INITIAL_PROMPT = 'I want to draft a new document. Help me refine the requirements.';
const SESSION_TYPE = 'prd-refinement';

/** Opens the PRD chat session and emits parsed state changes to the caller. */
export function connectPRDChat(
  onState: (next: Partial<PRDChatState>) => void,
  options: PRDChatOptions = {}
): PRDChatSocket {
  let sessionId: string | null = null;
  let closed = false;
  let activeStream: EventSource | null = null;
  let streamInFlight = false;

  const pendingPayloads: string[] = [];
  const pendingMessages: string[] = [];

  const processQueue = async () => {
    if (closed || sessionId === null || streamInFlight) {
      return;
    }

    const nextMessage = pendingMessages.shift();
    if (!nextMessage) {
      return;
    }

    streamInFlight = true;
    onState({ busy: true });

    try {
      const assistantText = await postAndStreamResponse(sessionId, nextMessage, (stream) => {
        activeStream = stream;
      });

      if (assistantText.trim() !== '') {
        onState({
          messages: [{ type: 'agent', text: assistantText }],
          finalContent: assistantText,
          finalTitle: 'Drafted Document'
        });
      }
    } catch (err) {
      onState({ messages: [{ type: 'error', error: messageForError(err) }] });
    } finally {
      streamInFlight = false;
      activeStream = null;
      onState({ busy: false });
      void processQueue();
    }
  };

  const enqueueUserMessage = (text: string) => {
    if (closed) {
      return;
    }
    const trimmed = text.trim();
    if (trimmed === '') {
      return;
    }
    pendingMessages.push(trimmed);
    void processQueue();
  };

  const applyPayload = (payload: string) => {
    if (closed) {
      return;
    }

    if (sessionId === null) {
      pendingPayloads.push(payload);
      return;
    }

    try {
      const decoded = JSON.parse(payload) as { type?: string; text?: string };
      if (decoded.type === 'user' && typeof decoded.text === 'string') {
        enqueueUserMessage(decoded.text);
      }
    } catch {
      onState({ messages: [{ type: 'error', error: 'Invalid chat payload' }] });
    }
  };

  const socket: PRDChatSocket = {
    send(payload: string) {
      applyPayload(payload);
    },
    close() {
      closed = true;
      activeStream?.close();
      activeStream = null;
      pendingPayloads.length = 0;
      pendingMessages.length = 0;
      onState({ starting: false, busy: false });
    }
  };

  onState({
    messages: [],
    finalContent: null,
    finalTitle: null,
    starting: true,
    busy: false
  });

  void (async () => {
    try {
      const createResponse = await fetch(`${chatBaseUrl}/v1/chat/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ type: SESSION_TYPE, context: options.context || {} })
      });
      if (!createResponse.ok) {
        throw new Error(await readError(createResponse));
      }

      const payload = await createResponse.json() as { sessionId?: string };
      if (!payload.sessionId) {
        throw new Error('chat session creation returned no sessionId');
      }

      sessionId = payload.sessionId;
      onState({ starting: false });

      const firstPrompt = options.initialPrompt && options.initialPrompt.trim() !== ''
        ? options.initialPrompt
        : INITIAL_PROMPT;
      applyPayload(JSON.stringify({ type: 'user', text: firstPrompt }));
      while (pendingPayloads.length > 0) {
        const queued = pendingPayloads.shift();
        if (queued) {
          applyPayload(queued);
        }
      }
    } catch (err) {
      onState({
        starting: false,
        busy: false,
        messages: [{ type: 'error', error: messageForError(err) }]
      });
    }
  })();

  return socket;
}

/** Sends a user-authored message through the active PRD chat connection. */
export function sendPRDChatMessage(socket: PRDChatSocket | null, text: string) {
  if (!socket || !text) {
    return false;
  }
  socket.send(JSON.stringify({ type: 'user', text }));
  return true;
}

function streamResponse(
  sessionId: string,
  onOpen: (stream: EventSource) => void
): Promise<string> {
  return new Promise((resolve, reject) => {
    const stream = new EventSource(`${chatBaseUrl}/v1/chat/sessions/${sessionId}/stream`);
    onOpen(stream);

    let assistantText = '';
    let settled = false;

    const done = (result: { value?: string; error?: Error }) => {
      if (settled) {
        return;
      }
      settled = true;
      stream.close();
      if (result.error) {
        reject(result.error);
        return;
      }
      resolve(result.value ?? assistantText);
    };

    stream.addEventListener('message.delta', (event: Event) => {
      const msg = event as MessageEvent;
      try {
        const payload = JSON.parse(msg.data) as { delta?: string };
        if (typeof payload.delta === 'string') {
          assistantText += payload.delta;
        }
      } catch {
        // Ignore malformed chunks and continue streaming.
      }
    });

    stream.addEventListener('message.completed', () => {
      done({ value: assistantText });
    });

    stream.addEventListener('error', (event: Event) => {
      const maybeMessage = event as MessageEvent;
      if (typeof maybeMessage.data === 'string' && maybeMessage.data.trim() !== '') {
        try {
          const payload = JSON.parse(maybeMessage.data) as { message?: string; error?: string };
          const msg = payload.message || payload.error;
          if (msg) {
            done({ error: new Error(msg) });
            return;
          }
        } catch {
          // Fall through to default error.
        }
      }
      done({ error: new Error('Streaming error') });
    });
  });
}

async function postAndStreamResponse(
  sessionId: string,
  message: string,
  onOpen: (stream: EventSource) => void
): Promise<string> {
  const postResponse = await fetch(`${chatBaseUrl}/v1/chat/sessions/${sessionId}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message })
  });

  if (postResponse.ok) {
    return streamResponse(sessionId, onOpen);
  }

  if (postResponse.status === 409) {
    return streamResponse(sessionId, onOpen);
  }

  throw new Error(await readError(postResponse));
}

async function readError(response: Response): Promise<string> {
  try {
    const payload = await response.json() as { error?: string };
    if (payload.error) {
      return payload.error;
    }
  } catch {
    // Fall through to status-based fallback.
  }
  return `HTTP ${response.status}`;
}

function messageForError(err: unknown): string {
  if (err instanceof Error && err.message) {
    return err.message;
  }
  return 'Unexpected chat error';
}
