import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';

import { appState } from '$lib/stores';
import * as api from '$lib/api';
import TasksPage from './+page.svelte';

const POLL_INTERVAL_MS = 10_000;

vi.mock('$app/navigation', () => ({
  goto: vi.fn()
}));

vi.mock('$lib/api', () => ({
  fetchJSON: vi.fn(),
  createTaskContract: vi.fn(),
  patchTaskContract: vi.fn(),
  approveTaskContract: vi.fn(),
  createLoopFromTask: vi.fn()
}));

describe('Tasks Kanban lanes', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (window as any).__SMITH_CONFIG__ = { featureTasksEnabled: true };
    appState.update((state) => ({
      ...state,
      projects: [{ id: 'smith' }]
    }));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders In Focus as part of the lane set and buckets seeded tasks by lane', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-22T12:00:00.000Z'));

    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/tasks') {
        return [
          {
            kind: 'task',
            id: 'task-running-1',
            project_id: 'smith',
            provider_profile_id: 'codex-default',
            objective: 'Run migrations',
            status: 'running',
            metadata: { current_step: 'Applying migration 3/8' },
            updated_at: '2026-03-22T11:55:00.000Z'
          },
          {
            kind: 'task',
            id: 'task-running-2',
            project_id: 'smith',
            provider_profile_id: 'codex-default',
            objective: 'Backfill cache',
            status: 'running',
            metadata: { current_step: 'Replaying 120 events' },
            updated_at: '2026-03-22T11:59:40.000Z'
          },
          {
            kind: 'task',
            id: 'task-draft-1',
            project_id: 'smith',
            provider_profile_id: 'codex-default',
            objective: 'Draft follow-up',
            status: 'draft'
          },
          {
            kind: 'task',
            id: 'task-blocked-1',
            project_id: 'smith',
            provider_profile_id: 'codex-default',
            objective: 'Waiting on dependency',
            status: 'blocked'
          },
          {
            kind: 'task',
            id: 'task-completed-1',
            project_id: 'smith',
            provider_profile_id: 'codex-default',
            objective: 'Release docs',
            status: 'completed'
          }
        ];
      }
      return [];
    });

    render(TasksPage);

    await waitFor(() => {
      expect(screen.getByTestId('lane-in_focus')).toBeTruthy();
    });

    expect(screen.getByTestId('lane-backlog')).toBeTruthy();
    expect(screen.getByTestId('lane-blocked')).toBeTruthy();
    expect(screen.getByTestId('lane-done')).toBeTruthy();

    expect(screen.getAllByTestId('task-card-in_focus')).toHaveLength(2);
    expect(screen.getAllByTestId('task-card-backlog')).toHaveLength(1);
    expect(screen.getAllByTestId('task-card-blocked')).toHaveLength(1);
    expect(screen.getAllByTestId('task-card-done')).toHaveLength(1);
    expect(screen.getByText('Step: Applying migration 3/8')).toBeTruthy();
    expect(screen.getByText('Step: Replaying 120 events')).toBeTruthy();
    expect(screen.getByText('Updated 5m ago')).toBeTruthy();
    expect(screen.getByText('Updated just now')).toBeTruthy();
  });

  it('renders safe metadata fallbacks when in-focus card metadata is missing', async () => {
    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/tasks') {
        return [
          {
            kind: 'task',
            id: 'task-running-fallbacks',
            project_id: 'smith',
            provider_profile_id: 'codex-default',
            objective: 'Observe active fallback behavior',
            status: 'running'
          }
        ];
      }
      return [];
    });

    render(TasksPage);

    await waitFor(() => {
      expect(screen.getAllByTestId('task-card-in_focus')).toHaveLength(1);
    });

    expect(screen.getByText('Step unavailable')).toBeTruthy();
    expect(screen.getByText('Update time unavailable')).toBeTruthy();
  });

  it('shows In Focus empty state and zero cards when no active tasks are present', async () => {
    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/tasks') {
        return [
          {
            kind: 'task',
            id: 'task-draft-1',
            project_id: 'smith',
            provider_profile_id: 'codex-default',
            objective: 'Draft follow-up',
            status: 'draft'
          }
        ];
      }
      return [];
    });

    render(TasksPage);

    await waitFor(() => {
      expect(screen.getByTestId('lane-empty-in_focus')).toBeTruthy();
    });

    expect(screen.getByText('No tasks currently in active execution.')).toBeTruthy();
    expect(screen.queryAllByTestId('task-card-in_focus')).toHaveLength(0);
  });

  it('moves a card into In Focus on the next refresh cycle when status becomes running', async () => {
    vi.useFakeTimers();
    let tasksResponse: api.TaskContract[] = [
      {
        kind: 'task',
        id: 'task-transition-1',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Prepare release',
        status: 'draft'
      }
    ];

    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/tasks') {
        return tasksResponse;
      }
      return [];
    });

    render(TasksPage);

    await waitFor(() => {
      expect(screen.getAllByTestId('task-card-backlog')).toHaveLength(1);
    });
    expect(screen.queryAllByTestId('task-card-in_focus')).toHaveLength(0);

    tasksResponse = [
      {
        kind: 'task',
        id: 'task-transition-1',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Prepare release',
        status: 'running'
      }
    ];

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    await waitFor(() => {
      expect(screen.getAllByTestId('task-card-in_focus')).toHaveLength(1);
    });
    expect(screen.queryAllByTestId('task-card-backlog')).toHaveLength(0);
  });

  it('moves a card out of In Focus and into Done on the next refresh cycle when execution completes', async () => {
    vi.useFakeTimers();
    let tasksResponse: api.TaskContract[] = [
      {
        kind: 'task',
        id: 'task-transition-2',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Apply migration',
        status: 'running'
      }
    ];

    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/tasks') {
        return tasksResponse;
      }
      return [];
    });

    render(TasksPage);

    await waitFor(() => {
      expect(screen.getAllByTestId('task-card-in_focus')).toHaveLength(1);
    });
    expect(screen.queryAllByTestId('task-card-done')).toHaveLength(0);

    tasksResponse = [
      {
        kind: 'task',
        id: 'task-transition-2',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Apply migration',
        status: 'completed'
      }
    ];

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    await waitFor(() => {
      expect(screen.getAllByTestId('task-card-done')).toHaveLength(1);
    });
    expect(screen.queryAllByTestId('task-card-in_focus')).toHaveLength(0);
  });

  it('preserves non-active lane counts and keeps cards exclusive to one lane', async () => {
    const seededTasks: api.TaskContract[] = [
      {
        kind: 'task',
        id: 'task-draft-only',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Draft plan',
        status: 'draft'
      },
      {
        kind: 'task',
        id: 'task-validated-only',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Validate plan',
        status: 'validated'
      },
      {
        kind: 'task',
        id: 'task-approved-only',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Approved task',
        status: 'approved'
      },
      {
        kind: 'task',
        id: 'task-blocked-only',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Blocked task',
        status: 'blocked'
      },
      {
        kind: 'task',
        id: 'task-completed-only',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Completed task',
        status: 'completed'
      },
      {
        kind: 'task',
        id: 'task-running-only',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Running task',
        status: 'running'
      }
    ];

    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/tasks') {
        return seededTasks;
      }
      return [];
    });

    render(TasksPage);

    await waitFor(() => {
      expect(screen.getAllByTestId('task-card-backlog')).toHaveLength(3);
    });

    expect(screen.getAllByTestId('task-card-in_focus')).toHaveLength(1);
    expect(screen.getAllByTestId('task-card-blocked')).toHaveLength(1);
    expect(screen.getAllByTestId('task-card-done')).toHaveLength(1);

    const totalRenderedCards =
      screen.getAllByTestId('task-card-in_focus').length +
      screen.getAllByTestId('task-card-backlog').length +
      screen.getAllByTestId('task-card-blocked').length +
      screen.getAllByTestId('task-card-done').length;
    expect(totalRenderedCards).toBe(seededTasks.length);

    for (const task of seededTasks) {
      expect(screen.getAllByText(task.id)).toHaveLength(1);
    }
  });
});
