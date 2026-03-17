import { apiBaseUrl, fetchWithTimeout } from '$lib/api';

import type { FeatureCapabilityInputs } from './validation';

export type FeatureCapabilityExecutionResult = {
  confirmationId: string;
  message: string;
  status: string;
  idempotencyKey: string;
  runId: string | null;
};

export type FeatureCapabilityRunStatusResult = {
  status: string;
  retryAvailable: boolean;
};

export type FeatureCapabilityRetryResult = {
  attemptId: string | null;
  message: string;
  status: string;
  idempotencyKey: string;
};

/** Represents an API-level failure while executing or polling feature capability runs. */
export class FeatureCapabilityExecutionError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'FeatureCapabilityExecutionError';
    this.status = status;
  }
}

type ExecuteDeps = {
  fetcher?: typeof fetch;
};

export const TERMINAL_RUN_STATUSES = ['SUCCEEDED', 'FAILED_RECOVERABLE', 'FAILED_TERMINAL'] as const;

function getActionableErrorMessage(status: number, fallback: string): string {
  if (status === 403) {
    return 'You do not have permission to execute this feature flow. Contact an administrator.';
  }
  if (status === 409) {
    return 'Execution is already in progress for this flow. Refresh and verify current status before retrying.';
  }
  if (status === 422) {
    return 'Execution failed validation. Review the required fields and correct the highlighted values.';
  }
  return fallback;
}

/** Generates an idempotency key used by execute/retry API requests. */
export function generateIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `fc-${Date.now()}-${Math.random().toString(36).slice(2, 12)}`;
}

/** Returns true when a run status is considered terminal by UI polling logic. */
export function isTerminalRunStatus(status: string): boolean {
  return TERMINAL_RUN_STATUSES.includes(status as (typeof TERMINAL_RUN_STATUSES)[number]);
}

/** Triggers the feature capability task execution with an idempotency key. */
export async function executeFeatureCapabilityTask(
  inputs: FeatureCapabilityInputs,
  deps: ExecuteDeps = {}
): Promise<FeatureCapabilityExecutionResult> {
  const idempotencyKey = generateIdempotencyKey();
  const fetcher = deps.fetcher || fetch;

  const response = await fetchWithTimeout(
    `${apiBaseUrl}/v1/feature-capability/execute`,
    {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        'Idempotency-Key': idempotencyKey
      },
      body: JSON.stringify({
        input_payload: {
          task_name: inputs.taskName,
          target_environment: inputs.targetEnvironment
        }
      })
    },
    20000,
    'feature capability execute'
  );

  const body = await response.json().catch(() => ({} as Record<string, any>));

  if (!response.ok) {
    const rawError = typeof body.error === 'string' ? body.error : `HTTP ${response.status}`;
    throw new FeatureCapabilityExecutionError(
      response.status,
      getActionableErrorMessage(response.status, rawError)
    );
  }

  const confirmationId = typeof body.confirmation_id === 'string' && body.confirmation_id.length > 0
    ? body.confirmation_id
    : idempotencyKey;
  const message = typeof body.message === 'string' && body.message.length > 0
    ? body.message
    : 'Execution started successfully.';
  const status = typeof body.status === 'string' && body.status.length > 0
    ? body.status
    : 'EXECUTING';

  return {
    confirmationId,
    message,
    status,
    idempotencyKey,
    runId: typeof body.run_id === 'string' && body.run_id.length > 0 ? body.run_id : null
  };
}

/** Fetches the latest backend status for an existing feature capability run. */
export async function getFeatureCapabilityRunStatus(runId: string): Promise<FeatureCapabilityRunStatusResult> {
  const response = await fetchWithTimeout(
    `${apiBaseUrl}/v1/feature-capability/runs/${encodeURIComponent(runId)}`,
    {
      method: 'GET',
      headers: {
        Accept: 'application/json'
      }
    },
    20000,
    'feature capability run status'
  );

  const body = await response.json().catch(() => ({} as Record<string, any>));
  if (!response.ok) {
    const rawError = typeof body.error === 'string' ? body.error : `HTTP ${response.status}`;
    throw new FeatureCapabilityExecutionError(response.status, rawError);
  }

  if (typeof body.status === 'string' && body.status.length > 0) {
    return {
      status: body.status,
      retryAvailable: body?.attempt_summary?.retry_available === true
    };
  }
  throw new FeatureCapabilityExecutionError(500, 'Run status response did not include status.');
}

/** Requests a retry for a recoverable feature capability run failure. */
export async function retryFeatureCapabilityRun(runId: string): Promise<FeatureCapabilityRetryResult> {
  const idempotencyKey = generateIdempotencyKey();
  const response = await fetchWithTimeout(
    `${apiBaseUrl}/v1/feature-capability/runs/${encodeURIComponent(runId)}/retry`,
    {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        'Idempotency-Key': idempotencyKey
      },
      body: JSON.stringify({
        reason: 'operator_retry'
      })
    },
    20000,
    'feature capability retry'
  );

  const body = await response.json().catch(() => ({} as Record<string, any>));
  if (!response.ok) {
    const rawError = typeof body.error === 'string' ? body.error : `HTTP ${response.status}`;
    throw new FeatureCapabilityExecutionError(
      response.status,
      getActionableErrorMessage(response.status, rawError)
    );
  }

  return {
    attemptId: typeof body.attempt_id === 'string' && body.attempt_id.length > 0 ? body.attempt_id : null,
    message: typeof body.message === 'string' && body.message.length > 0 ? body.message : 'Retry started successfully.',
    status: typeof body.status === 'string' && body.status.length > 0 ? body.status : 'EXECUTING',
    idempotencyKey
  };
}
