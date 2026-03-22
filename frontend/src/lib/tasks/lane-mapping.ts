import type { TaskContract } from '$lib/api';

export type TaskStatus = TaskContract['status'];

export type TaskLane = 'in_focus' | 'backlog' | 'blocked' | 'done';

/** Active execution statuses that must appear in the In Focus lane. */
export const ACTIVE_EXECUTION_STATUSES: readonly TaskStatus[] = ['running'] as const;

const STATUS_TO_LANE: Record<TaskStatus, TaskLane> = {
  draft: 'backlog',
  validated: 'backlog',
  approved: 'backlog',
  running: 'in_focus',
  completed: 'done',
  blocked: 'blocked'
};

/**
 * Resolves a task status to a deterministic lane for board grouping.
 * Unknown statuses are treated as backlog and intentionally excluded from In Focus.
 */
export function resolveTaskLane(status: string): TaskLane {
  const lane = STATUS_TO_LANE[status as TaskStatus];
  if (lane) {
    return lane;
  }

  console.warn(`[tasks] unknown task status \"${status}\" mapped to backlog lane`);
  return 'backlog';
}
