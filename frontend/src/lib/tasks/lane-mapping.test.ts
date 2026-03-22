import { describe, expect, it, vi } from 'vitest';

import { ACTIVE_EXECUTION_STATUSES, resolveTaskLane, type TaskLane, type TaskStatus } from '$lib/tasks/lane-mapping';

describe('task lane mapping', () => {
  it('documents running as the active execution status set', () => {
    expect(ACTIVE_EXECUTION_STATUSES).toEqual(['running']);
  });

  it('maps each known task status to the expected lane', () => {
    const expected: Record<TaskStatus, TaskLane> = {
      draft: 'backlog',
      validated: 'backlog',
      approved: 'backlog',
      queued: 'backlog',
      running: 'in_focus',
      completed: 'done',
      blocked: 'blocked'
    };

    for (const [status, lane] of Object.entries(expected) as [TaskStatus, TaskLane][]) {
      expect(resolveTaskLane(status)).toBe(lane);
    }
  });

  it('never maps non-active statuses into in_focus', () => {
    const nonActiveStatuses: TaskStatus[] = ['draft', 'validated', 'approved', 'queued', 'completed', 'blocked'];
    for (const status of nonActiveStatuses) {
      expect(resolveTaskLane(status)).not.toBe('in_focus');
    }
  });

  it('falls back unknown statuses to backlog and logs a warning', () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => undefined);

    expect(resolveTaskLane('paused')).toBe('backlog');
    expect(warnSpy).toHaveBeenCalledWith('[tasks] unknown task status "paused" mapped to backlog lane');

    warnSpy.mockRestore();
  });
});
