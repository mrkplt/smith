import { describe, expect, it } from 'vitest';

import type { TaskContract } from '$lib/api';
import { getTaskLaneID, groupTasksByLane, isQueuedNonRunning } from '$lib/task-lanes';

function task(overrides: Partial<TaskContract>): TaskContract {
  return {
    kind: 'task_contract',
    id: 'task-1',
    project_id: 'smith',
    provider_profile_id: 'codex-default',
    objective: 'Objective',
    status: 'draft',
    ...overrides
  };
}

describe('task lane mapping', () => {
  it('maps queued non-running tasks to scheduled', () => {
    const queued = task({ status: 'queued' });
    expect(isQueuedNonRunning(queued)).toBe(true);
    expect(getTaskLaneID(queued)).toBe('scheduled');
  });

  it('keeps queued running tasks out of scheduled', () => {
    const queuedRunning = task({ status: 'queued', runtime_state: 'running' });
    expect(isQueuedNonRunning(queuedRunning)).toBe(false);
    expect(getTaskLaneID(queuedRunning)).toBe('running');
  });

  it('uses metadata runtime_state for queued running detection', () => {
    const queuedRunning = task({ status: 'queued', metadata: { runtime_state: 'running' } });
    expect(isQueuedNonRunning(queuedRunning)).toBe(false);
    expect(getTaskLaneID(queuedRunning)).toBe('running');
  });

  it('never maps running or completed tasks into scheduled', () => {
    expect(getTaskLaneID(task({ status: 'running' }))).toBe('running');
    expect(getTaskLaneID(task({ status: 'completed' }))).toBe('completed');
  });

  it('groups tasks into lanes using scheduled mapping rules', () => {
    const grouped = groupTasksByLane([
      task({ id: 'queued-non-running', status: 'queued' }),
      task({ id: 'queued-running', status: 'queued', runtime_state: 'running' }),
      task({ id: 'running', status: 'running' }),
      task({ id: 'completed', status: 'completed' })
    ]);

    expect(grouped.scheduled.map((item) => item.id)).toEqual(['queued-non-running']);
    expect(grouped.running.map((item) => item.id)).toEqual(['queued-running', 'running']);
    expect(grouped.completed.map((item) => item.id)).toEqual(['completed']);
  });

  it('moves queued task out of scheduled when runtime_state becomes running', () => {
    const queued = task({ id: 'transition', status: 'queued' });
    expect(getTaskLaneID(queued)).toBe('scheduled');

    const runningRuntime = { ...queued, runtime_state: 'running' };
    expect(getTaskLaneID(runningRuntime)).toBe('running');
  });

  it('moves queued task back into scheduled when runtime_state is no longer running', () => {
    const queuedRunning = task({ id: 'transition-back', status: 'queued', runtime_state: 'running' });
    expect(getTaskLaneID(queuedRunning)).toBe('running');

    const queuedIdle = { ...queuedRunning, runtime_state: 'queued' };
    expect(getTaskLaneID(queuedIdle)).toBe('scheduled');
  });
});
