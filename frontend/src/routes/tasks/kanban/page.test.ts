import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';

import * as stores from '$lib/stores';
import * as api from '$lib/api';
import TasksKanbanPage from './+page.svelte';
import { goto } from '$app/navigation';

vi.mock('$app/navigation', () => ({
  goto: vi.fn()
}));

vi.mock('$lib/api', () => ({
  apiBaseUrl: '/api',
  fetchJSON: vi.fn()
}));

vi.mock('$lib/stores', async (importOriginal) => {
  const original = await importOriginal<typeof import('$lib/stores')>();
  return {
    ...original,
    pushToast: vi.fn()
  };
});

describe('Tasks Kanban route', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (window as any).__SMITH_CONFIG__ = {};
  });

  it('redirects to /pods when Tasks Kanban gate is disabled', async () => {
    const infoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
    (window as any).__SMITH_CONFIG__ = {
      featureTasksKanbanEnabled: false
    };

    render(TasksKanbanPage);

    await waitFor(() => {
      expect(goto).toHaveBeenCalledWith('/pods', { replaceState: true });
    });
    expect(stores.pushToast).toHaveBeenCalledWith('Tasks Kanban is not enabled in this environment', 'muted');
    expect(infoSpy).toHaveBeenCalledWith('[feature-flag] feature=tasks-kanban action=route-block redirect=/pods reason=disabled');
    infoSpy.mockRestore();
  });

  it('renders lane cards from task status data when enabled', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON).mockResolvedValue([
      {
        kind: 'task',
        id: 'task-running-1',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Run migration',
        status: 'running',
        updated_at: '2026-03-22T12:00:00.000Z',
        metadata: { current_step: 'Applying migration 2/6', runtime_state: 'running' }
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
        id: 'task-done-1',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Release docs',
        status: 'completed'
      }
    ]);

    render(TasksKanbanPage);

    await waitFor(() => {
      expect(screen.getByTestId('lane-in_focus')).toBeTruthy();
    });

    expect(screen.getByTestId('lane-backlog')).toBeTruthy();
    expect(screen.getByTestId('lane-blocked')).toBeTruthy();
    expect(screen.getByTestId('lane-done')).toBeTruthy();

    expect(screen.getAllByTestId('task-card-in_focus')).toHaveLength(1);
    expect(screen.getAllByTestId('task-card-backlog')).toHaveLength(1);
    expect(screen.getAllByTestId('task-card-blocked')).toHaveLength(1);
    expect(screen.getAllByTestId('task-card-done')).toHaveLength(1);

    expect(screen.getByText('Step: Applying migration 2/6')).toBeTruthy();
    expect(screen.getByText('State: running')).toBeTruthy();
  });

  it('opens task detail panel when a card is selected', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON).mockResolvedValue([
      {
        kind: 'task',
        id: 'task-running-2',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Ship kanban detail panel',
        status: 'running',
        created_at: '2026-03-22T10:00:00.000Z',
        updated_at: '2026-03-22T12:00:00.000Z',
        source_document: 'doc-123',
        acceptance_criteria: ['Card click opens panel', 'Panel shows dependencies'],
        validation: ['npm --prefix frontend run check'],
        metadata: {
          current_step: 'Render detail section',
          runtime_state: 'running',
          loop_id: 'loop-99',
          prd_story_dependencies: 'story-a, story-b'
        }
      }
    ]);

    const { container } = render(TasksKanbanPage);

    await waitFor(() => {
      expect(screen.getByTestId('task-card-in_focus')).toBeTruthy();
    });

    const card = container.querySelector('[data-testid="task-card-in_focus"]') as HTMLButtonElement;
    card.click();

    await waitFor(() => {
      expect(screen.getByTestId('task-detail-panel')).toBeTruthy();
    });

    const detailPanel = screen.getByTestId('task-detail-panel');
    const detail = within(detailPanel);

    expect(detail.getByText('Ship kanban detail panel')).toBeTruthy();
    expect(detail.getByText('doc-123')).toBeTruthy();
    expect(detail.getByText('loop-99')).toBeTruthy();
    expect(detail.getByText('story-a, story-b')).toBeTruthy();
    expect(detail.getByText('Card click opens panel')).toBeTruthy();
    expect(detail.getByText('npm --prefix frontend run check')).toBeTruthy();
  });

  it('filters tasks by lane and text query', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON).mockResolvedValue([
      {
        kind: 'task',
        id: 'task-running-3',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Run worker sync',
        status: 'running',
        source_document: 'doc-run'
      },
      {
        kind: 'task',
        id: 'task-blocked-3',
        project_id: 'smith',
        provider_profile_id: 'claude-default',
        objective: 'Investigate blocked migration',
        status: 'blocked',
        source_document: 'doc-blocked'
      }
    ]);

    render(TasksKanbanPage);

    await waitFor(() => {
      expect(screen.getAllByTestId('task-card-in_focus')).toHaveLength(1);
      expect(screen.getAllByTestId('task-card-blocked')).toHaveLength(1);
    });

    const laneFilter = screen.getByTestId('tasks-filter-lane') as HTMLSelectElement;
    await fireEvent.change(laneFilter, { target: { value: 'blocked' } });

    await waitFor(() => {
      expect(screen.queryAllByTestId('task-card-in_focus')).toHaveLength(0);
      expect(screen.getAllByTestId('task-card-blocked')).toHaveLength(1);
    });

    const search = screen.getByTestId('tasks-filter-search') as HTMLInputElement;
    await fireEvent.input(search, { target: { value: 'migration' } });

    await waitFor(() => {
      expect(screen.getAllByTestId('task-card-blocked')).toHaveLength(1);
    });

    await fireEvent.input(search, { target: { value: 'no-match' } });

    await waitFor(() => {
      expect(screen.queryAllByTestId('task-card-blocked')).toHaveLength(0);
      expect(screen.getByTestId('lane-empty-blocked')).toBeTruthy();
    });
  });
});
