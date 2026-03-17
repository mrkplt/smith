import { beforeEach, describe, expect, it, vi, afterEach } from 'vitest';

import {
  attachLoopTerminal,
  approveTaskContract,
  cancelLoop,
  createLoopFromTask,
  createLoopIntervention,
  createTaskContract,
  deleteJSON,
  fetchJSON,
  fetchWithTimeout,
  getJSON,
  getTaskContract,
  patchTaskContract,
  pauseLoop,
  postJSON,
  requestJSON,
  resumeLoop,
  detachLoopTerminal,
  sendLoopCommand
} from '$lib/api';

describe('api helpers', () => {
  let originalClearTimeout: any;

  beforeEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
    // Cache original globally to restore later
    originalClearTimeout = globalThis.clearTimeout;
    if (typeof globalThis.clearTimeout === 'undefined') {
      (globalThis as any).clearTimeout = vi.fn();
    } else {
      vi.spyOn(globalThis, 'clearTimeout');
    }
  });

  afterEach(() => {
    vi.clearAllTimers();
    if (originalClearTimeout === undefined) {
      delete (globalThis as any).clearTimeout;
    } else {
      (globalThis as any).clearTimeout = originalClearTimeout;
    }
  });

  describe('fetchWithTimeout', () => {
    it('returns the response when fetch succeeds before timeout', async () => {
      const mockResponse = new Response('ok', { status: 200 });
      vi.spyOn(globalThis, 'fetch').mockResolvedValue(mockResponse);

      const response = await fetchWithTimeout('/fast', {}, 1000);

      expect(response).toBe(mockResponse);
      expect(fetch).toHaveBeenCalledWith('/fast', expect.objectContaining({
        signal: expect.any(AbortSignal)
      }));
      expect(globalThis.clearTimeout).toHaveBeenCalled();
    });

    it('throws a timeout error when fetch aborts', async () => {
      const error = new Error('aborted');
      error.name = 'AbortError';
      vi.spyOn(globalThis, 'fetch').mockRejectedValue(error);

      await expect(fetchWithTimeout('/slow', {}, 10, 'slow call')).rejects.toThrow(
        'Request timed out after 1s for slow call'
      );
    });

    it('throws standard errors unmodified', async () => {
      const error = new TypeError('Network error');
      vi.spyOn(globalThis, 'fetch').mockRejectedValue(error);

      await expect(fetchWithTimeout('/error', {}, 1000)).rejects.toThrow(TypeError);
      expect(globalThis.clearTimeout).toHaveBeenCalled();
    });

    it('aborts the request when timeout is reached', async () => {
      vi.useFakeTimers();

      let abortSignal: AbortSignal;

      vi.spyOn(globalThis, 'fetch').mockImplementation((url, options: any) => {
        abortSignal = options.signal;

        return new Promise((res, rej) => {
          abortSignal.addEventListener('abort', () => {
            const error = new Error('aborted');
            error.name = 'AbortError';
            rej(error);
          });
        });
      });

      const timeoutPromise = fetchWithTimeout('/slow', {}, 10000, 'slow call');

      vi.advanceTimersByTime(10000);

      await expect(timeoutPromise).rejects.toThrow('Request timed out after 10s for slow call');
      expect(abortSignal!.aborted).toBe(true);
    });
  });

  describe('fetchJSON', () => {
    it('returns parsed JSON for successful fetchJSON calls', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValue({
        ok: true,
        json: async () => ({ ok: true })
      } as Response);

      await expect(fetchJSON('/v1/projects')).resolves.toEqual({ ok: true });
      expect(fetch).toHaveBeenCalledWith('/api/v1/projects', expect.objectContaining({
        headers: { Accept: 'application/json' },
        signal: expect.any(AbortSignal)
      }));
    });
  });

  describe('requestJSON', () => {
    it('throws requestJSON errors from API payloads', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValue({
        ok: false,
        status: 400,
        json: async () => ({ error: 'bad request' })
      } as Response);

      await expect(requestJSON('/v1/test', 'POST', { ok: false })).rejects.toThrow('bad request');
    });

    it('falls back to HTTP status text when error payload is missing', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValue({
        ok: false,
        status: 503,
        json: async () => ({})
      } as Response);

      await expect(deleteJSON('/v1/test')).rejects.toThrow('HTTP 503');
    });

    it('serializes payloads for postJSON and getJSON helpers', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValue({
        ok: true,
        json: async () => ({ ok: true })
      } as Response);

      await postJSON('/v1/loops', { id: 'loop-1' });
      await getJSON('/v1/loops');

      expect(fetch).toHaveBeenNthCalledWith(1, '/api/v1/loops', expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ id: 'loop-1' })
      }));
      expect(fetch).toHaveBeenNthCalledWith(2, '/api/v1/loops', expect.objectContaining({
        method: 'GET',
        body: undefined
      }));
    });

    it('uses task and loop helper paths under /api base', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValue({
        ok: true,
        json: async () => ({ ok: true })
      } as Response);

      await createTaskContract({ project_id: 'smith', provider_profile_id: 'codex-default', objective: 'obj' });
      await getTaskContract('task-1');
      await patchTaskContract('task-1', { objective: 'updated' });
      await approveTaskContract('task-1');
      await pauseLoop('loop-1', { actor: 'alice' });
      await resumeLoop('loop-1', { actor: 'alice' });
      await cancelLoop('loop-1', { actor: 'alice' });
      await createLoopIntervention('loop-1', { instruction: 'avoid auth changes', event_id: 'evt-1' });
      await attachLoopTerminal('loop-1', { actor: 'alice', terminal: 'console-pods' });
      await sendLoopCommand('loop-1', { actor: 'alice', command: 'ls -la' });
      await detachLoopTerminal('loop-1', { actor: 'alice' });
      await createLoopFromTask('task-1', 'idem-1');

      expect(fetch).toHaveBeenNthCalledWith(1, '/api/tasks', expect.objectContaining({ method: 'POST' }));
      expect(fetch).toHaveBeenNthCalledWith(2, '/api/tasks/task-1', expect.objectContaining({ method: 'GET' }));
      expect(fetch).toHaveBeenNthCalledWith(3, '/api/tasks/task-1', expect.objectContaining({ method: 'PATCH' }));
      expect(fetch).toHaveBeenNthCalledWith(4, '/api/tasks/task-1/approve', expect.objectContaining({ method: 'POST' }));
      expect(fetch).toHaveBeenNthCalledWith(5, '/api/loops/loop-1/pause', expect.objectContaining({ method: 'POST' }));
      expect(fetch).toHaveBeenNthCalledWith(6, '/api/loops/loop-1/resume', expect.objectContaining({ method: 'POST' }));
      expect(fetch).toHaveBeenNthCalledWith(7, '/api/loops/loop-1/cancel', expect.objectContaining({ method: 'POST' }));
      expect(fetch).toHaveBeenNthCalledWith(8, '/api/loops/loop-1/interventions', expect.objectContaining({ method: 'POST' }));
      expect(fetch).toHaveBeenNthCalledWith(9, '/api/v1/loops/loop-1/control/attach', expect.objectContaining({ method: 'POST' }));
      expect(fetch).toHaveBeenNthCalledWith(10, '/api/v1/loops/loop-1/control/command', expect.objectContaining({ method: 'POST' }));
      expect(fetch).toHaveBeenNthCalledWith(11, '/api/v1/loops/loop-1/control/detach', expect.objectContaining({ method: 'POST' }));
      expect(fetch).toHaveBeenNthCalledWith(12, '/api/v1/loops', expect.objectContaining({ method: 'POST' }));
    });
  });
});
