import { beforeEach, describe, expect, it, vi } from 'vitest';

import { deleteJSON, fetchJSON, fetchWithTimeout, getJSON, postJSON, requestJSON } from '$lib/api';

describe('api helpers', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

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

  it('throws a timeout error when fetch aborts', async () => {
    const error = new Error('aborted');
    error.name = 'AbortError';
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(error);

    await expect(fetchWithTimeout('/slow', {}, 10, 'slow call')).rejects.toThrow(
      'Request timed out after 1s for slow call'
    );
  });

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
});
