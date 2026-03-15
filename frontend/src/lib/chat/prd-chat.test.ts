import { beforeEach, describe, expect, it, vi } from 'vitest';

import { connectPRDChat, sendPRDChatMessage } from '$lib/chat/prd-chat';

class MockEventSource {
  static instances: MockEventSource[] = [];

  url: string;
  closed = false;
  private listeners = new Map<string, Array<(event: Event) => void>>();

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: (event: Event) => void) {
    const existing = this.listeners.get(type) || [];
    existing.push(listener);
    this.listeners.set(type, existing);
  }

  close() {
    this.closed = true;
  }

  emit(type: string, data: string) {
    const listeners = this.listeners.get(type) || [];
    for (const listener of listeners) {
      listener({ data } as unknown as Event);
    }
  }
}

function jsonResponse(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { 'Content-Type': 'application/json' }
  });
}

describe('PRD chat helpers', () => {
  beforeEach(() => {
    MockEventSource.instances = [];
    vi.stubGlobal('EventSource', MockEventSource);
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  it('creates a chat session and streams an initial assistant response', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/chat/v1/chat/sessions')) {
        return jsonResponse({ sessionId: 'sess_1' });
      }
      if (url.endsWith('/chat/v1/chat/sessions/sess_1/messages')) {
        return jsonResponse({ status: 'queued' });
      }
      throw new Error(`unexpected URL: ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    const states: Record<string, unknown>[] = [];
    connectPRDChat((next) => states.push(next));

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(MockEventSource.instances).toHaveLength(1);
    expect(MockEventSource.instances[0].url).toBe('/chat/v1/chat/sessions/sess_1/stream');

    const postCall = fetchMock.mock.calls[1];
    const postBody = JSON.parse(String(postCall?.[1]?.body));
    expect(postBody.message).toContain('I want to draft a new document');

    MockEventSource.instances[0].emit('message.delta', JSON.stringify({ delta: 'Hello ' }));
    MockEventSource.instances[0].emit('message.delta', JSON.stringify({ delta: 'world' }));
    MockEventSource.instances[0].emit('message.completed', JSON.stringify({}));

    await vi.waitFor(() => {
      expect(states).toContainEqual({ messages: [{ type: 'agent', text: 'Hello world' }], finalContent: 'Hello world', finalTitle: 'Drafted Document' });
    });
    expect(states).toContainEqual({ starting: false });
    expect(states).toContainEqual({ busy: true });
    expect(states).toContainEqual({ busy: false });
  });

  it('queues user messages while a stream is active', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/chat/v1/chat/sessions')) {
        return jsonResponse({ sessionId: 'sess_1' });
      }
      if (url.endsWith('/chat/v1/chat/sessions/sess_1/messages')) {
        return jsonResponse({ status: 'queued' });
      }
      throw new Error(`unexpected URL: ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    const socket = connectPRDChat(() => {});
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));

    expect(sendPRDChatMessage(socket, 'follow up')).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(2);

    MockEventSource.instances[0].emit('message.completed', JSON.stringify({}));
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    expect(MockEventSource.instances).toHaveLength(2);

    const queuedPostCall = fetchMock.mock.calls[2];
    const postBody = JSON.parse(String(queuedPostCall?.[1]?.body));
    expect(postBody).toEqual({ message: 'follow up' });
  });

  it('reports stream errors and validates explicit send helper behavior', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/chat/v1/chat/sessions')) {
        return jsonResponse({ sessionId: 'sess_1' });
      }
      if (url.endsWith('/chat/v1/chat/sessions/sess_1/messages')) {
        return jsonResponse({ status: 'queued' });
      }
      throw new Error(`unexpected URL: ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    const states: Record<string, unknown>[] = [];
    const socket = connectPRDChat((next) => states.push(next));
    await vi.waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit('error', JSON.stringify({ message: 'goose failed' }));
    await vi.waitFor(() => {
      expect(states).toContainEqual({ messages: [{ type: 'error', error: 'goose failed' }] });
    });

    expect(sendPRDChatMessage(socket, 'hello')).toBe(true);
    expect(sendPRDChatMessage(null, 'hello')).toBe(false);
    expect(sendPRDChatMessage(socket, '')).toBe(false);
  });

  it('recovers from post conflict by consuming pending stream', async () => {
    let messagePostCount = 0;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/chat/v1/chat/sessions')) {
        return jsonResponse({ sessionId: 'sess_1' });
      }
      if (url.endsWith('/chat/v1/chat/sessions/sess_1/messages')) {
        messagePostCount += 1;
        if (messagePostCount === 1) {
          return jsonResponse({ status: 'queued' });
        }
        return jsonResponse({ error: 'previous message is still pending stream consumption' }, 409);
      }
      throw new Error(`unexpected URL: ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    const states: Record<string, unknown>[] = [];
    const socket = connectPRDChat((next) => states.push(next));
    await vi.waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit('message.completed', JSON.stringify({}));

    expect(sendPRDChatMessage(socket, 'follow up')).toBe(true);
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    await vi.waitFor(() => expect(MockEventSource.instances).toHaveLength(2));

    MockEventSource.instances[1].emit('message.delta', JSON.stringify({ delta: 'Recovered answer' }));
    MockEventSource.instances[1].emit('message.completed', JSON.stringify({}));

    await vi.waitFor(() => {
      expect(states).toContainEqual({ messages: [{ type: 'agent', text: 'Recovered answer' }], finalContent: 'Recovered answer', finalTitle: 'Drafted Document' });
    });
  });
});
