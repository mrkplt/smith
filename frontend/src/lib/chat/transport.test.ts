import { beforeEach, describe, expect, it, vi } from 'vitest';

import { commitChatAction, createChatSession, openChatStream, postChatMessage } from '$lib/chat/transport';

class MockEventSource {
  url: string;

  constructor(url: string) {
    this.url = url;
  }
}

describe('chat transport', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.stubGlobal('EventSource', MockEventSource);
  });

  it('creates chat sessions with JSON payloads', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true } as Response);

    await createChatSession({ type: 'prd', context: { project: 'alpha' } });

    expect(fetch).toHaveBeenCalledWith('/chat/v1/chat/sessions', expect.objectContaining({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: 'prd', context: { project: 'alpha' } })
    }));
  });

  it('posts messages and commit actions to the expected endpoints', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true } as Response);

    await postChatMessage('session-1', 'hello');
    await commitChatAction({ action: 'commit', payload: { loopID: 'loop-1' } });

    expect(fetch).toHaveBeenNthCalledWith(1, '/chat/v1/chat/sessions/session-1/messages', expect.objectContaining({
      body: JSON.stringify({ message: 'hello' })
    }));
    expect(fetch).toHaveBeenNthCalledWith(2, '/chat/v1/chat/actions/commit', expect.objectContaining({
      body: JSON.stringify({ action: 'commit', payload: { loopID: 'loop-1' } })
    }));
  });

  it('opens an event stream for chat updates', () => {
    const source = openChatStream('session-2') as unknown as MockEventSource;
    expect(source.url).toBe('/chat/v1/chat/sessions/session-2/stream');
  });
});
