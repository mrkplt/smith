import { beforeEach, describe, expect, it, vi } from 'vitest';

import { connectPRDChat, sendPRDChatMessage } from '$lib/chat/prd-chat';

class MockWebSocket {
  static instances: MockWebSocket[] = [];

  url: string;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onclose: (() => void) | null = null;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  send(payload: string) {
    this.sent.push(payload);
  }

  close() {
    this.onclose?.();
  }
}

describe('PRD chat helpers', () => {
  beforeEach(() => {
    MockWebSocket.instances = [];
    vi.stubGlobal('WebSocket', MockWebSocket);
    vi.stubGlobal('window', {
      location: {
        protocol: 'https:',
        host: 'smith.local'
      }
    });
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  it('connects, emits initial state, and sends the initial prompt', () => {
    const states: Record<string, unknown>[] = [];

    const socket = connectPRDChat(next => states.push(next));
    const mock = socket as unknown as MockWebSocket;

    expect(mock.url).toBe('wss://smith.local/api/v1/chat/prd');
    expect(states[0]).toEqual({
      messages: [],
      finalContent: null,
      finalTitle: null,
      starting: true,
      busy: false
    });

    mock.onopen?.();

    expect(states).toContainEqual({ starting: false });
    expect(mock.sent[0]).toBe(JSON.stringify({
      type: 'user',
      text: 'I want to draft a new document. Help me refine the requirements.'
    }));
  });

  it('updates busy and final state from websocket messages', () => {
    const states: Record<string, unknown>[] = [];
    const socket = connectPRDChat(next => states.push(next)) as unknown as MockWebSocket;

    socket.onmessage?.({
      data: JSON.stringify({ type: 'agent', text: 'thinking' })
    } as MessageEvent<string>);

    socket.onmessage?.({
      data: JSON.stringify({
        type: 'system',
        text: 'Final content',
        final_title: 'Doc title',
        final_prd_path: '/tmp/doc.md'
      })
    } as MessageEvent<string>);

    expect(states).toContainEqual({ busy: true });
    expect(states).toContainEqual({ messages: [{ type: 'agent', text: 'thinking' }] });
    expect(states).toContainEqual({
      finalContent: 'Final content',
      finalTitle: 'Doc title',
      busy: false
    });
  });

  it('ignores invalid websocket payloads and handles explicit messages', () => {
    const socket = connectPRDChat(() => {}) as unknown as MockWebSocket;

    socket.onmessage?.({ data: 'not-json' } as MessageEvent<string>);

    expect(sendPRDChatMessage(socket as unknown as WebSocket, 'hello')).toBe(true);
    expect(sendPRDChatMessage(null, 'hello')).toBe(false);
    expect(sendPRDChatMessage(socket as unknown as WebSocket, '')).toBe(false);
    expect(socket.sent.at(-1)).toBe(JSON.stringify({ type: 'user', text: 'hello' }));
  });
});
