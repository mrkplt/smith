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

const INITIAL_PROMPT = 'I want to draft a new document. Help me refine the requirements.';

/** Opens the PRD chat websocket and emits parsed state changes to the caller. */
export function connectPRDChat(
  onState: (next: Partial<PRDChatState>) => void
) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  const url = `${protocol}//${host}/api/v1/chat/prd`;

  const socket = new WebSocket(url);
  onState({
    messages: [],
    finalContent: null,
    finalTitle: null,
    starting: true,
    busy: false
  });

  socket.onopen = () => {
    onState({ starting: false });
    socket.send(JSON.stringify({ type: 'user', text: INITIAL_PROMPT }));
  };

  socket.onmessage = event => {
    try {
      const message = JSON.parse(event.data) as PRDChatMessage;
      if (message.type === 'system' && message.final_prd_path) {
        onState({
          finalContent: message.text ?? null,
          finalTitle: message.final_title ?? 'Drafted Document',
          busy: false
        });
      } else if (message.type === 'agent') {
        onState({ busy: true });
      }
      onState({ messages: [message] });
    } catch (err) {
      console.error('Failed to parse chat message', err);
    }
  };

  socket.onclose = () => {
    onState({ starting: false, busy: false });
  };

  return socket;
}

/** Sends a user-authored message through the active PRD chat socket. */
export function sendPRDChatMessage(socket: WebSocket | null, text: string) {
  if (!socket || !text) {
    return false;
  }
  socket.send(JSON.stringify({ type: 'user', text }));
  return true;
}
