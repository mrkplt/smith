import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ChatSession } from '$lib/chat/store.svelte';

class MockEventSource {
  url: string;
  addEventListener: any;
  close: any;

  constructor(url: string) {
    this.url = url;
    this.addEventListener = vi.fn();
    this.close = vi.fn();
  }
}

describe('ChatSession error handling in store.svelte.ts', () => {
    let session: ChatSession;

    beforeEach(() => {
        vi.restoreAllMocks();
        session = new ChatSession();
        vi.stubGlobal('EventSource', MockEventSource);
    });

    it('sets error state when createSession fails', async () => {
        const errorMessage = 'Network connection failed';
        vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error(errorMessage));

        await session.createSession('prd', { project: 'alpha' });

        expect(session.error).toBe(`Failed to create session: ${errorMessage}`);
        expect(session.sessionId).toBeNull();
    });

    it('sets error state when sendMessage fails', async () => {
        session.sessionId = 'mock-session-id';
        const errorMessage = 'API rate limit exceeded';
        vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error(errorMessage));

        await session.sendMessage('Hello world');

        expect(session.error).toBe(`Failed to send message: ${errorMessage}`);
    });

    it('sets error state when commitAction fails', async () => {
        const errorMessage = 'Invalid payload';
        vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error(errorMessage));

        const success = await session.commitAction('approve', { id: '123' });

        expect(success).toBe(false);
        expect(session.error).toBe(`Failed to commit action: ${errorMessage}`);
    });
});
