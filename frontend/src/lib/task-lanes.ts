import type { TaskContract } from '$lib/api';

export type LaneID = 'scheduled' | 'draft' | 'validated' | 'approved' | 'running' | 'completed' | 'blocked';

export type LaneDefinition = {
  id: LaneID;
  label: string;
};

export const laneDefinitions: LaneDefinition[] = [
  { id: 'scheduled', label: 'Scheduled' },
  { id: 'draft', label: 'Draft' },
  { id: 'validated', label: 'Validated' },
  { id: 'approved', label: 'Approved' },
  { id: 'running', label: 'Running' },
  { id: 'completed', label: 'Completed' },
  { id: 'blocked', label: 'Blocked' }
];

function normalizeRuntimeState(task: TaskContract): string {
  const topLevel = (task as TaskContract & { runtime_state?: unknown }).runtime_state;
  if (typeof topLevel === 'string') {
    return topLevel.trim().toLowerCase();
  }
  const metadataState = task.metadata?.runtime_state;
  if (typeof metadataState === 'string') {
    return metadataState.trim().toLowerCase();
  }
  return '';
}

/** Returns true when a queued task is not actively running. */
export function isQueuedNonRunning(task: TaskContract): boolean {
  return task.status === 'queued' && normalizeRuntimeState(task) !== 'running';
}

/** Resolves the kanban lane for a task using deterministic queued-state rules. */
export function getTaskLaneID(task: TaskContract): LaneID {
  if (task.status === 'running') {
    return 'running';
  }
  if (task.status === 'completed') {
    return 'completed';
  }
  if (task.status === 'queued') {
    return isQueuedNonRunning(task) ? 'scheduled' : 'running';
  }
  return laneDefinitions.some((lane) => lane.id === task.status) ? (task.status as LaneID) : 'scheduled';
}

/** Groups task contracts into kanban lanes keyed by lane id. */
export function groupTasksByLane(items: TaskContract[]): Record<LaneID, TaskContract[]> {
  const grouped = laneDefinitions.reduce(
    (acc, lane) => {
      acc[lane.id] = [];
      return acc;
    },
    {} as Record<LaneID, TaskContract[]>
  );

  for (const task of items) {
    grouped[getTaskLaneID(task)].push(task);
  }
  return grouped;
}
