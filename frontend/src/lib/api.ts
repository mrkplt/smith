import { appState } from './stores';
import { get } from 'svelte/store';

const config = (typeof window !== 'undefined' ? (window as any).__SMITH_CONFIG__ : {}) || {};
export const apiBaseUrl = (config.apiBaseUrl || "/api").replace(/\/+$/, "");
export const chatBaseUrl = (config.chatBaseUrl || "/chat").replace(/\/+$/, "");

/** Fetches a resource and aborts when the timeout elapses. */
export async function fetchWithTimeout(url: string, options: any = {}, timeoutMs = 20000, label?: string) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { ...options, signal: controller.signal });
  } catch (err: any) {
    if (err && err.name === "AbortError") {
      throw new Error(`Request timed out after ${Math.ceil(timeoutMs / 1000)}s for ${label || url}`);
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

/** Fetches JSON from the Smith API and throws on non-success responses. */
export async function fetchJSON(path: string) {
  const res = await fetchWithTimeout(`${apiBaseUrl}${path}`, {
    headers: { Accept: "application/json" }
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status} for ${path}`);
  }
  return res.json();
}

/** Sends a JSON POST request to the Smith API. */
export async function postJSON(path: string, payload: any) {
  return requestJSON(path, "POST", payload);
}

/** Sends a JSON GET request to the Smith API. */
export async function getJSON(path: string) {
  return requestJSON(path, "GET");
}

/** Sends a JSON DELETE request to the Smith API. */
export async function deleteJSON(path: string) {
  return requestJSON(path, "DELETE");
}

/** Sends a JSON request to the Smith API and returns the decoded payload. */
export async function requestJSON(path: string, method: string, payload?: any) {
  const res = await fetchWithTimeout(`${apiBaseUrl}${path}`, {
    method,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const msg = body.error || `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return body;
}

export interface TaskContract {
  kind: string;
  id: string;
  project_id: string;
  provider_profile_id: string;
  source_document?: string;
  objective: string;
  constraints?: string[];
  acceptance_criteria?: string[];
  validation?: string[];
  status: 'draft' | 'validated' | 'approved' | 'running' | 'completed' | 'blocked';
  metadata?: Record<string, string>;
  created_at?: string;
  updated_at?: string;
  correlation_id?: string;
  schema_version?: string;
}

export interface TaskContractCreateRequest {
  id?: string;
  project_id: string;
  provider_profile_id: string;
  source_document?: string;
  objective: string;
  constraints?: string[];
  acceptance_criteria?: string[];
  validation?: string[];
  status?: TaskContract['status'];
  metadata?: Record<string, string>;
  correlation_id?: string;
  actor?: string;
}

export interface TaskContractPatchRequest {
  project_id?: string;
  provider_profile_id?: string;
  source_document?: string;
  objective?: string;
  constraints?: string[];
  acceptance_criteria?: string[];
  validation?: string[];
  status?: TaskContract['status'];
  metadata?: Record<string, string>;
  actor?: string;
}

export interface LoopLifecycleRequest {
  actor?: string;
  reason?: string;
}

export interface LoopInterventionRequest {
  actor?: string;
  type?: string;
  instruction: string;
  event_id?: string;
}

export interface LoopInterventionResponse {
  loop_id: string;
  event_id: string;
  sequence: number;
  idempotent: boolean;
  instruction: string;
}

export interface LoopCleanupRequest {
  actor?: string;
  loop_ids?: string[];
  states?: string[];
}

export interface LoopCleanupResponse {
  actor: string;
  matched_count: number;
  deleted_count: number;
  deleted: string[];
  skipped_active?: string[];
  not_found?: string[];
}

export interface LoopTerminalAttachRequest {
  actor?: string;
  terminal?: string;
}

export interface LoopTerminalDetachRequest {
  actor?: string;
}

export interface LoopTerminalCommandRequest {
  actor?: string;
  command: string;
}

export interface LoopCreateResult {
  loop_id: string;
  status: string;
  created: boolean;
  message?: string;
}

/** Creates a task contract draft from the supplied payload. */
export async function createTaskContract(payload: TaskContractCreateRequest): Promise<TaskContract> {
  return requestJSON('/tasks', 'POST', payload);
}

/** Loads a single task contract by id. */
export async function getTaskContract(taskID: string): Promise<TaskContract> {
  return requestJSON(`/tasks/${taskID}`, 'GET');
}

/** Applies a partial update to an existing task contract. */
export async function patchTaskContract(taskID: string, payload: TaskContractPatchRequest): Promise<TaskContract> {
  return requestJSON(`/tasks/${taskID}`, 'PATCH', payload);
}

/** Approves a task contract for loop creation and execution. */
export async function approveTaskContract(taskID: string, actor = 'operator'): Promise<TaskContract> {
  return requestJSON(`/tasks/${taskID}/approve`, 'POST', { actor });
}

/** Requests the control plane to pause a running loop. */
export async function pauseLoop(loopID: string, payload: LoopLifecycleRequest = {}): Promise<any> {
  return requestJSON(`/loops/${loopID}/pause`, 'POST', payload);
}

/** Requests the control plane to resume a paused loop. */
export async function resumeLoop(loopID: string, payload: LoopLifecycleRequest = {}): Promise<any> {
  return requestJSON(`/loops/${loopID}/resume`, 'POST', payload);
}

/** Requests cancellation of an active loop. */
export async function cancelLoop(loopID: string, payload: LoopLifecycleRequest = {}): Promise<any> {
  return requestJSON(`/loops/${loopID}/cancel`, 'POST', payload);
}

/** Deletes a non-active loop. */
export async function deleteLoop(loopID: string, payload: { actor?: string } = {}): Promise<any> {
  return requestJSON(`/loops/${loopID}`, 'DELETE', payload);
}

/** Deletes non-active loops in bulk by ids or state selectors. */
export async function cleanupLoops(payload: LoopCleanupRequest): Promise<LoopCleanupResponse> {
  return requestJSON('/v1/loops/cleanup', 'POST', payload);
}

/** Sends an intervention instruction to a loop event stream. */
export async function createLoopIntervention(loopID: string, payload: LoopInterventionRequest): Promise<LoopInterventionResponse> {
  return requestJSON(`/loops/${loopID}/interventions`, 'POST', payload);
}

/** Attaches an operator terminal session to a running loop runtime target. */
export async function attachLoopTerminal(loopID: string, payload: LoopTerminalAttachRequest = {}): Promise<any> {
  return requestJSON(`/v1/loops/${loopID}/control/attach`, 'POST', payload);
}

/** Detaches an operator terminal session from a running loop runtime target. */
export async function detachLoopTerminal(loopID: string, payload: LoopTerminalDetachRequest = {}): Promise<any> {
  return requestJSON(`/v1/loops/${loopID}/control/detach`, 'POST', payload);
}

/** Sends a command to the active loop runtime terminal session. */
export async function sendLoopCommand(loopID: string, payload: LoopTerminalCommandRequest): Promise<any> {
  return requestJSON(`/v1/loops/${loopID}/control/command`, 'POST', payload);
}

/** Creates a loop from an approved task contract. */
export async function createLoopFromTask(taskContractID: string, idempotencyKey = ''): Promise<LoopCreateResult> {
  const payload: Record<string, string> = { task_contract_id: taskContractID };
  if (idempotencyKey.trim() !== '') {
    payload.idempotency_key = idempotencyKey.trim();
  }
  return requestJSON('/v1/loops', 'POST', payload);
}
