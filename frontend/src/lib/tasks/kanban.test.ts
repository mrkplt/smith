import { describe, expect, it } from 'vitest';

import type { TaskContract } from '$lib/api';
import { buildTaskLanes, getTaskLaneID, getTaskTerminalOutcome } from '$lib/tasks/kanban';

function makeTask(id: string, status: TaskContract['status']): TaskContract {
  return {
    kind: 'task_contract',
    id,
    project_id: 'smith',
    provider_profile_id: 'codex-default',
    objective: `task ${id}`,
    status
  };
}

describe('tasks kanban lane mapping', () => {
  it('maps completed tasks to finished lane', () => {
    expect(getTaskLaneID(makeTask('t-1', 'completed'), { kanbanEnabled: true })).toBe('finished');
  });

  it('maps blocked tasks to finished lane', () => {
    expect(getTaskLaneID(makeTask('t-1', 'blocked'), { kanbanEnabled: true })).toBe('finished');
  });

  it('keeps completed tasks out of active lanes', () => {
    const lanes = buildTaskLanes([
      makeTask('t-1', 'completed'),
      makeTask('t-2', 'draft'),
      makeTask('t-3', 'running')
    ], { kanbanEnabled: true });

    const finished = lanes.find((lane) => lane.id === 'finished');
    const active = lanes.filter((lane) => lane.id !== 'finished');

    expect(finished?.tasks.map((task) => task.id)).toEqual(['t-1']);
    for (const lane of active) {
      expect(lane.tasks.some((task) => task.id === 't-1')).toBe(false);
    }
  });

  it('shows all completed tasks in finished lane', () => {
    const lanes = buildTaskLanes([
      makeTask('t-1', 'completed'),
      makeTask('t-2', 'completed'),
      makeTask('t-3', 'completed')
    ], { kanbanEnabled: true });
    const finished = lanes.find((lane) => lane.id === 'finished');
    expect(finished?.tasks.map((task) => task.id)).toEqual(['t-1', 't-2', 't-3']);
  });

  it('includes blocked items when filtering to finished lane', () => {
    const lanes = buildTaskLanes([
      makeTask('t-1', 'blocked'),
      makeTask('t-2', 'completed'),
      makeTask('t-3', 'running')
    ], { kanbanEnabled: true });
    const finished = lanes.find((lane) => lane.id === 'finished');
    expect(finished?.tasks.map((task) => task.id)).toEqual(['t-1', 't-2']);
  });

  it('does not activate finished grouping when kanban flag is disabled', () => {
    const lanes = buildTaskLanes([
      makeTask('t-1', 'blocked'),
      makeTask('t-2', 'completed'),
      makeTask('t-3', 'running')
    ]);
    const finished = lanes.find((lane) => lane.id === 'finished');
    const running = lanes.find((lane) => lane.id === 'running');

    expect(finished).toBeUndefined();
    expect(running?.tasks.map((task) => task.id)).toEqual(['t-1', 't-2', 't-3']);
  });

  it('prefers explicit terminal outcome metadata when present', () => {
    const task = makeTask('t-1', 'completed');
    task.terminal_outcome = 'blocked';
    expect(getTaskTerminalOutcome(task)).toBe('blocked');
  });

  it('falls back to task status when terminal outcome metadata is absent', () => {
    expect(getTaskTerminalOutcome(makeTask('t-1', 'blocked'))).toBe('blocked');
    expect(getTaskTerminalOutcome(makeTask('t-2', 'completed'))).toBe('completed');
  });

  it('finished lane count equals completed plus blocked tasks', () => {
    const lanes = buildTaskLanes([
      makeTask('t-1', 'completed'),
      makeTask('t-2', 'blocked'),
      makeTask('t-3', 'running'),
      makeTask('t-4', 'draft')
    ], { kanbanEnabled: true });
    const finished = lanes.find((lane) => lane.id === 'finished');
    expect(finished?.tasks.length).toBe(2);
  });

  it('finished lane count decreases when a terminal task returns to active', () => {
    const tasks = [
      makeTask('t-1', 'completed'),
      makeTask('t-2', 'blocked'),
      makeTask('t-3', 'running')
    ];
    const before = buildTaskLanes(tasks, { kanbanEnabled: true });
    tasks[1].status = 'running';
    const after = buildTaskLanes(tasks, { kanbanEnabled: true });

    const beforeFinished = before.find((lane) => lane.id === 'finished');
    const afterFinished = after.find((lane) => lane.id === 'finished');
    expect(beforeFinished?.tasks.length).toBe(2);
    expect(afterFinished?.tasks.length).toBe(1);
  });

  it('assigns every task to exactly one lane with no omissions or duplication', () => {
    const tasks = [
      makeTask('t-1', 'draft'),
      makeTask('t-2', 'validated'),
      makeTask('t-3', 'approved'),
      makeTask('t-4', 'running'),
      makeTask('t-5', 'completed'),
      makeTask('t-6', 'blocked')
    ];
    const lanes = buildTaskLanes(tasks, { kanbanEnabled: true });
    const assigned = lanes.flatMap((lane) => lane.tasks.map((task) => task.id));
    const uniqueAssigned = new Set(assigned);

    expect(assigned.length).toBe(tasks.length);
    expect(uniqueAssigned.size).toBe(tasks.length);
  });
});
