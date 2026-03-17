import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  executeFeatureCapabilityTask,
  FeatureCapabilityExecutionError,
  getFeatureCapabilityRunStatus,
  isTerminalRunStatus,
  retryFeatureCapabilityRun
} from '$lib/feature-capability/execution';

describe('feature capability execution', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('submits execution payload with idempotency and returns a confirmation indicator', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({
        status: 'EXECUTING',
        confirmation_id: 'confirm-1',
        message: 'Execution accepted.'
      })
    } as Response);

    const result = await executeFeatureCapabilityTask({
      taskName: 'Prepare deployment report',
      targetEnvironment: 'staging'
    });

    expect(result).toEqual({
      status: 'EXECUTING',
      confirmationId: 'confirm-1',
      message: 'Execution accepted.',
      idempotencyKey: expect.any(String),
      runId: null
    });

    expect(fetchSpy).toHaveBeenCalledWith('/api/v1/feature-capability/execute', expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({
        'Idempotency-Key': expect.any(String)
      })
    }));
  });

  it('returns actionable messaging for validation failures', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: false,
      status: 422,
      json: async () => ({ error: 'invalid payload' })
    } as Response);

    await expect(executeFeatureCapabilityTask({
      taskName: 'Run',
      targetEnvironment: 'prod'
    })).rejects.toMatchObject({
      status: 422,
      message: 'Execution failed validation. Review the required fields and correct the highlighted values.'
    } satisfies Partial<FeatureCapabilityExecutionError>);
  });

  it('uses fallback confirmation details when API response omits optional fields', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({})
    } as Response);

    const result = await executeFeatureCapabilityTask({
      taskName: 'Run',
      targetEnvironment: 'prod'
    });

    expect(result.status).toBe('EXECUTING');
    expect(result.message).toBe('Execution started successfully.');
    expect(result.confirmationId).toBe(result.idempotencyKey);
    expect(result.runId).toBe(null);
  });

  it('fetches backend run status for status updates', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({ status: 'SUCCEEDED', attempt_summary: { retry_available: false } })
    } as Response);

    await expect(getFeatureCapabilityRunStatus('run-1')).resolves.toEqual({
      status: 'SUCCEEDED',
      retryAvailable: false
    });
  });

  it('submits retry requests with idempotency and returns retry metadata', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({ status: 'EXECUTING', attempt_id: 'attempt-2' })
    } as Response);

    const result = await retryFeatureCapabilityRun('run-1');

    expect(result).toEqual({
      status: 'EXECUTING',
      attemptId: 'attempt-2',
      message: 'Retry started successfully.',
      idempotencyKey: expect.any(String)
    });

    expect(fetchSpy).toHaveBeenCalledWith('/api/v1/feature-capability/runs/run-1/retry', expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({
        'Idempotency-Key': expect.any(String)
      })
    }));
  });

  it('flags terminal statuses', () => {
    expect(isTerminalRunStatus('SUCCEEDED')).toBe(true);
    expect(isTerminalRunStatus('FAILED_RECOVERABLE')).toBe(true);
    expect(isTerminalRunStatus('FAILED_TERMINAL')).toBe(true);
    expect(isTerminalRunStatus('EXECUTING')).toBe(false);
  });
});
