import { chatBaseUrl } from '$lib/api';

export interface PRDChatMessage {
  type: string;
  text?: string;
  error?: string;
  final_content?: string;
  final_title?: string;
  final_prd_path?: string;
}

export interface DocumentPatchOperation {
  op: string;
  content?: string;
  [key: string]: unknown;
}

export interface DocumentPatchProposal {
  type: 'document_patch_proposal';
  operations: DocumentPatchOperation[];
  title?: string;
  [key: string]: unknown;
}

export interface PRDChatState {
  messages: PRDChatMessage[];
  finalContent: string | null;
  finalTitle: string | null;
  assistantDraft: string | null;
  busy: boolean;
  starting: boolean;
  readinessStatus: string | null;
  sessionIntent: string | null;
  documentVersion: string | null;
  documentTitle: string | null;
  contextDrift: boolean;
  patchProposal: DocumentPatchProposal | null;
}

export interface PRDChatSocket {
  send(payload: string): void;
  updateContext(update: PRDChatContextUpdate): void;
  close(): void;
}

export interface PRDChatContextUpdate {
  type?: 'ui.context.updated';
  context?: Record<string, string>;
  focusContext?: Record<string, unknown>;
}

export interface PRDChatOptions {
  initialPrompt?: string;
  context?: Record<string, string>;
  sessionIntent?: string;
}

const INITIAL_PROMPT = 'I want to draft a new document. Help me refine the requirements.';
const SESSION_TYPE = 'prd-refinement';

/** Opens the PRD chat session and emits parsed state changes to the caller. */
export function connectPRDChat(
  onState: (next: Partial<PRDChatState>) => void,
  options: PRDChatOptions = {}
): PRDChatSocket {
  const sessionIntent = (options.sessionIntent || '').trim() || 'document_refinement';
  let sessionId: string | null = null;
  let closed = false;
  let activeStream: EventSource | null = null;
  let streamInFlight = false;
  let contextUpdateInFlight = false;

  const pendingPayloads: string[] = [];
  const pendingMessages: string[] = [];
  const pendingContextUpdates: PRDChatContextUpdate[] = [];

  const processQueue = async () => {
    if (closed || sessionId === null || streamInFlight) {
      return;
    }

    const nextMessage = pendingMessages.shift();
    if (!nextMessage) {
      return;
    }

    streamInFlight = true;
    onState({ busy: true, assistantDraft: '' });
    let patchProposed = false;

    try {
      const assistantText = await postAndStreamResponse(
        sessionId,
        nextMessage,
        (stream) => {
          activeStream = stream;
        },
        (draft) => {
          onState({ assistantDraft: draft });
        },
        (eventName, payload) => {
          if (eventName === 'context.loaded') {
            const drift = payload?.versionDrift === true || payload?.contextDrift === true;
            onState({
              sessionIntent: asString(payload?.sessionIntent),
              documentVersion: asString(payload?.documentVersion),
              documentTitle: asString(payload?.documentTitle),
              contextDrift: drift
            });
            return;
          }

          if (eventName === 'readiness.updated') {
            onState({ readinessStatus: asString(payload?.status) });
            return;
          }

          if (eventName === 'document.patch.proposed' && isPatchProposal(payload)) {
            patchProposed = true;
            const replacement = replacementContentFromPatch(payload);
            onState({
              patchProposal: payload,
              finalContent: replacement || null,
              finalTitle: asString(payload.title) || 'Drafted Document'
            });
          }
        }
      );

      if (assistantText.trim() !== '') {
        if (patchProposed) {
          onState({
            messages: [{ type: 'agent', text: assistantText }],
            assistantDraft: null
          });
        } else {
          onState({
            messages: [{ type: 'agent', text: assistantText }],
            finalContent: assistantText,
            finalTitle: 'Drafted Document',
            assistantDraft: null
          });
        }
      } else {
        onState({ assistantDraft: null });
      }
    } catch (err) {
      onState({ messages: [{ type: 'error', error: messageForError(err) }], assistantDraft: null });
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

  const processContextQueue = async () => {
    if (closed || sessionId === null || contextUpdateInFlight || pendingContextUpdates.length === 0) {
      return;
    }

    const next = pendingContextUpdates.shift();
    if (!next) {
      return;
    }

    contextUpdateInFlight = true;
    try {
      const response = await fetch(`${chatBaseUrl}/v1/chat/sessions/${sessionId}/context`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: next.type || 'ui.context.updated',
          context: next.context || {},
          focusContext: next.focusContext || {}
        })
      });
      if (!response.ok) {
        throw new Error(await readError(response));
      }
    } catch (err) {
      onState({ messages: [{ type: 'error', error: `Context sync failed: ${messageForError(err)}` }] });
    } finally {
      contextUpdateInFlight = false;
      void processContextQueue();
    }
  };

  const enqueueContextUpdate = (update: PRDChatContextUpdate) => {
    if (closed) {
      return;
    }
    pendingContextUpdates.push(update);
    void processContextQueue();
  };

  const socket: PRDChatSocket = {
    send(payload: string) {
      applyPayload(payload);
    },
    updateContext(update: PRDChatContextUpdate) {
      enqueueContextUpdate(update);
    },
    close() {
      closed = true;
      activeStream?.close();
      activeStream = null;
      pendingPayloads.length = 0;
      pendingMessages.length = 0;
      pendingContextUpdates.length = 0;
      onState({ starting: false, busy: false, assistantDraft: null });
    }
  };

  onState({
    messages: [],
    finalContent: null,
    finalTitle: null,
    assistantDraft: null,
    readinessStatus: null,
    sessionIntent,
    documentVersion: null,
    documentTitle: null,
    contextDrift: false,
    patchProposal: null,
    starting: true,
    busy: false
  });

  void (async () => {
    try {
      const createResponse = await fetch(`${chatBaseUrl}/v1/chat/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: SESSION_TYPE,
          context: {
            ...(options.context || {}),
            sessionIntent
          }
        })
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
      void processContextQueue();
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
  onOpen: (stream: EventSource) => void,
  onDelta?: (assistantText: string) => void,
  onEvent?: (eventName: string, payload: any) => void
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
          onDelta?.(assistantText);
        }
      } catch {
        // Ignore malformed chunks and continue streaming.
      }
    });

    stream.addEventListener('message.completed', () => {
      done({ value: assistantText });
    });

    for (const eventName of ['session.started', 'context.loaded', 'readiness.updated', 'document.patch.proposed', 'stream.keepalive']) {
      stream.addEventListener(eventName, (event: Event) => {
        const msg = event as MessageEvent;
        onEvent?.(eventName, parseJSONPayload(msg.data));
      });
    }

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
  onOpen: (stream: EventSource) => void,
  onDelta?: (assistantText: string) => void,
  onEvent?: (eventName: string, payload: any) => void
): Promise<string> {
  const postResponse = await fetch(`${chatBaseUrl}/v1/chat/sessions/${sessionId}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message })
  });

  if (postResponse.ok) {
    return streamResponse(sessionId, onOpen, onDelta, onEvent);
  }

  if (postResponse.status === 409) {
    return streamResponse(sessionId, onOpen, onDelta, onEvent);
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

function parseJSONPayload(data: unknown): any {
  if (typeof data !== 'string' || data.trim() === '') {
    return null;
  }
  try {
    return JSON.parse(data);
  } catch {
    return null;
  }
}

function asString(value: unknown): string | null {
  if (typeof value !== 'string') {
    return null;
  }
  const trimmed = value.trim();
  if (trimmed === '') {
    return null;
  }
  return trimmed;
}

function isPatchProposal(value: any): value is DocumentPatchProposal {
  return value && value.type === 'document_patch_proposal' && Array.isArray(value.operations);
}

function replacementContentFromPatch(proposal: DocumentPatchProposal): string {
  for (const op of proposal.operations) {
    if (String(op.op || '').trim() === 'replace_document' && typeof op.content === 'string') {
      const trimmed = op.content.trim();
      if (trimmed !== '') {
        return op.content;
      }
    }
  }
  return '';
}
