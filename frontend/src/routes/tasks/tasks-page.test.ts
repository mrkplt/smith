import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/svelte';

import { appState } from '$lib/stores';
import * as api from '$lib/api';
import TasksPage from './+page.svelte';

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

function laneSection(title: string): HTMLElement {
  const heading = screen.getByRole('heading', { level: 3, name: title });
  const section = heading.closest('section');
  if (!section) {
    throw new Error(`Missing lane section for ${title}`);
  }
  return section as HTMLElement;
}

describe('Tasks page lanes', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: false
    };
    appState.update((state) => ({
      ...state,
      projects: [{ id: 'smith' }]
    }));
  });

  it('renders the default lane set and buckets seeded tasks', async () => {
    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/tasks') {
        return [
          { id: 'task-draft-1', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'Draft', status: 'draft' },
          { id: 'task-validated-1', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'Validated', status: 'validated' },
          { id: 'task-approved-1', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'Approved', status: 'approved' },
          { id: 'task-running-1', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'Running', status: 'running' },
          { id: 'task-completed-1', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'Completed', status: 'completed' },
          { id: 'task-blocked-1', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'Blocked', status: 'blocked' }
        ];
      }
      return [];
    });

    render(TasksPage);

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 3, name: 'Draft' })).toBeTruthy();
    });

    expect(screen.getByRole('heading', { level: 3, name: 'Validated' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 3, name: 'Approved' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 3, name: 'Running' })).toBeTruthy();
    expect(screen.queryByRole('heading', { level: 3, name: 'Finished' })).toBeNull();

    expect(within(laneSection('Draft')).getByText('task-draft-1')).toBeTruthy();
    expect(within(laneSection('Validated')).getByText('task-validated-1')).toBeTruthy();
    expect(within(laneSection('Approved')).getByText('task-approved-1')).toBeTruthy();
    expect(within(laneSection('Running')).getByText('task-running-1')).toBeTruthy();
    expect(within(laneSection('Running')).getByText('task-completed-1')).toBeTruthy();
    expect(within(laneSection('Running')).getByText('task-blocked-1')).toBeTruthy();
  });

  it('shows an empty-state message when there are no task contracts', async () => {
    vi.mocked(api.fetchJSON).mockResolvedValue([]);

    render(TasksPage);

    await waitFor(() => {
      expect(screen.getByText('No task contracts yet.')).toBeTruthy();
    });
  });

  it('renders each task id once across all lanes', async () => {
    const seededTasks: api.TaskContract[] = [
      { kind: 'task', id: 'task-a', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'A', status: 'draft' },
      { kind: 'task', id: 'task-b', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'B', status: 'validated' },
      { kind: 'task', id: 'task-c', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'C', status: 'approved' },
      { kind: 'task', id: 'task-d', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'D', status: 'running' },
      { kind: 'task', id: 'task-e', project_id: 'smith', provider_profile_id: 'codex-default', objective: 'E', status: 'completed' }
    ];

    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/tasks') {
        return seededTasks;
      }
      return [];
    });

    render(TasksPage);

    await waitFor(() => {
      expect(screen.getAllByText('task-a')).toHaveLength(1);
    });

    for (const task of seededTasks) {
      expect(screen.getAllByText(task.id)).toHaveLength(1);
    }
  });
});
