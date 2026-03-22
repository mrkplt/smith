import type { TaskContract } from '$lib/api';

export type TaskLaneID = 'draft' | 'validated' | 'approved' | 'running' | 'finished';

export interface TaskLane {
  id: TaskLaneID;
  title: string;
  tasks: TaskContract[];
}

export type TerminalOutcome = 'completed' | 'blocked';
export interface TaskKanbanOptions {
  kanbanEnabled?: boolean;
}

const laneTitles: Record<TaskLaneID, string> = {
  draft: 'Draft',
  validated: 'Validated',
  approved: 'Approved',
  running: 'Running',
  finished: 'Finished'
};

/**
 * Assigns tasks to a single canonical lane.
 * Terminal outcomes live in Finished only when kanban grouping is enabled.
 */
export function getTaskLaneID(task: TaskContract, options: TaskKanbanOptions = {}): TaskLaneID {
  switch (task.status) {
    case 'draft':
      return 'draft';
    case 'validated':
      return 'validated';
    case 'approved':
      return 'approved';
    case 'completed':
    case 'blocked':
      return options.kanbanEnabled ? 'finished' : 'running';
    case 'running':
    default:
      return 'running';
  }
}

/** Resolves terminal outcome using explicit metadata first, then status fallback. */
export function getTaskTerminalOutcome(task: TaskContract): TerminalOutcome | '' {
  if (task.terminal_outcome === 'completed' || task.terminal_outcome === 'blocked') {
    return task.terminal_outcome;
  }
  if (task.status === 'completed' || task.status === 'blocked') {
    return task.status;
  }
  return '';
}

/** Builds kanban lanes from the given task list using canonical lane assignment. */
export function buildTaskLanes(tasks: TaskContract[], options: TaskKanbanOptions = {}): TaskLane[] {
  const byLane: Record<TaskLaneID, TaskContract[]> = {
    draft: [],
    validated: [],
    approved: [],
    running: [],
    finished: []
  };

  for (const task of tasks) {
    byLane[getTaskLaneID(task, options)].push(task);
  }

  const laneOrder = options.kanbanEnabled
    ? (Object.keys(laneTitles) as TaskLaneID[])
    : (['draft', 'validated', 'approved', 'running'] as TaskLaneID[]);

  return laneOrder.map((id) => ({
    id,
    title: laneTitles[id],
    tasks: byLane[id]
  }));
}
