import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';

import { appState } from '$lib/stores';
import * as api from '$lib/api';
import TasksPage from './+page.svelte';

vi.mock('$app/navigation', () => ({
  goto: vi.fn()
}));

vi.mock('$lib/api', () => ({
  approveTaskContract: vi.fn(),
  createLoopFromTask: vi.fn(),
  createTaskContract: vi.fn(),
  fetchJSON: vi.fn(),
  patchTaskContract: vi.fn()
}));

describe('Tasks page Kanban lane visibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    appState.update((state) => ({
      ...state,
      projects: [{ id: 'smith' }]
    }));
    vi.mocked(api.fetchJSON).mockResolvedValue([]);
  });

  it('shows Awaiting Approval lane when tasks Kanban feature flag is enabled', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true
    };

    render(TasksPage);

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledWith('/tasks');
    });
    expect(screen.getByRole('heading', { name: 'Awaiting Approval' })).toBeTruthy();
  });

  it('hides Awaiting Approval lane when tasks Kanban feature flag is disabled', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: false
    };

    render(TasksPage);

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledWith('/tasks');
    });
    expect(screen.queryByRole('heading', { name: 'Awaiting Approval' })).toBeNull();
  });

  it('shows only validated tasks in Awaiting Approval lane with no duplicates', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON).mockResolvedValue([
      {
        id: 'task-1',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Validate auth flow',
        status: 'validated'
      },
      {
        id: 'task-2',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Draft runtime config',
        status: 'draft'
      },
      {
        id: 'task-3',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Review queue telemetry',
        status: 'validated'
      },
      {
        id: 'task-4',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Ship approval endpoint',
        status: 'approved'
      }
    ]);

    render(TasksPage);

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledWith('/tasks');
    });

    const lane = screen.getByTestId('awaiting-approval-lane');
    const laneScope = within(lane);
    const cards = laneScope.getAllByTestId('awaiting-approval-card');

    expect(cards).toHaveLength(2);
    expect(laneScope.getByText('Validate auth flow')).toBeTruthy();
    expect(laneScope.getByText('Review queue telemetry')).toBeTruthy();
    expect(laneScope.queryByText('Draft runtime config')).toBeNull();
    expect(laneScope.queryByText('Ship approval endpoint')).toBeNull();
  });

  it('orders Awaiting Approval by priority desc, validated timestamp asc, then id asc', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON).mockResolvedValue([
      {
        id: 'task-c',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Low priority',
        status: 'validated',
        review_priority: 1,
        validated_at: '2026-02-01T12:00:00Z'
      },
      {
        id: 'task-b',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'High priority older',
        status: 'validated',
        review_priority: 9,
        validated_at: '2026-02-01T11:00:00Z'
      },
      {
        id: 'task-a',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'High priority newer',
        status: 'validated',
        review_priority: 9,
        validated_at: '2026-02-01T12:00:00Z'
      },
      {
        id: 'task-d',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'High priority newer same timestamp',
        status: 'validated',
        review_priority: 9,
        validated_at: '2026-02-01T12:00:00Z'
      }
    ]);

    render(TasksPage);

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledWith('/tasks');
    });

    const laneCards = within(screen.getByTestId('awaiting-approval-lane')).getAllByTestId('awaiting-approval-card');
    const orderedIDs = laneCards.map((card) => within(card).getByText(/^task-/).textContent);

    expect(orderedIDs).toEqual(['task-b', 'task-a', 'task-d', 'task-c']);
  });

  it('keeps stable order on refresh and inserts newly validated tasks by the same rules', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON)
      .mockResolvedValueOnce([
        {
          id: 'task-10',
          kind: 'TaskContract',
          project_id: 'smith',
          provider_profile_id: 'codex-default',
          objective: 'First',
          status: 'validated',
          review_priority: 5,
          validated_at: '2026-02-01T10:00:00Z'
        },
        {
          id: 'task-11',
          kind: 'TaskContract',
          project_id: 'smith',
          provider_profile_id: 'codex-default',
          objective: 'Second',
          status: 'validated',
          review_priority: 5,
          validated_at: '2026-02-01T11:00:00Z'
        }
      ])
      .mockResolvedValueOnce([
        {
          id: 'task-10',
          kind: 'TaskContract',
          project_id: 'smith',
          provider_profile_id: 'codex-default',
          objective: 'First',
          status: 'validated',
          review_priority: 5,
          validated_at: '2026-02-01T10:00:00Z'
        },
        {
          id: 'task-11',
          kind: 'TaskContract',
          project_id: 'smith',
          provider_profile_id: 'codex-default',
          objective: 'Second',
          status: 'validated',
          review_priority: 5,
          validated_at: '2026-02-01T11:00:00Z'
        },
        {
          id: 'task-09',
          kind: 'TaskContract',
          project_id: 'smith',
          provider_profile_id: 'codex-default',
          objective: 'Inserted by ordering',
          status: 'validated',
          review_priority: 5,
          validated_at: '2026-02-01T10:30:00Z'
        }
      ]);

    render(TasksPage);

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledTimes(1);
    });

    let orderedIDs = within(screen.getByTestId('awaiting-approval-lane'))
      .getAllByTestId('awaiting-approval-card')
      .map((card) => within(card).getByText(/^task-/).textContent);
    expect(orderedIDs).toEqual(['task-10', 'task-11']);

    await fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledTimes(2);
    });

    orderedIDs = within(screen.getByTestId('awaiting-approval-lane'))
      .getAllByTestId('awaiting-approval-card')
      .map((card) => within(card).getByText(/^task-/).textContent);
    expect(orderedIDs).toEqual(['task-10', 'task-09', 'task-11']);
  });

  it('removes approved tasks from Awaiting Approval and shows them in Approved lane after refresh', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON)
      .mockResolvedValueOnce([
        {
          id: 'task-approve',
          kind: 'TaskContract',
          project_id: 'smith',
          provider_profile_id: 'codex-default',
          objective: 'Approve me',
          status: 'validated'
        }
      ])
      .mockResolvedValueOnce([
        {
          id: 'task-approve',
          kind: 'TaskContract',
          project_id: 'smith',
          provider_profile_id: 'codex-default',
          objective: 'Approve me',
          status: 'approved'
        }
      ]);
    vi.mocked(api.approveTaskContract).mockResolvedValue({ id: 'task-approve' } as any);

    render(TasksPage);

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledTimes(1);
    });
    expect(within(screen.getByTestId('awaiting-approval-lane')).getAllByTestId('awaiting-approval-card')).toHaveLength(1);

    await fireEvent.click(screen.getByRole('button', { name: 'Approve' }));

    await waitFor(() => {
      expect(api.approveTaskContract).toHaveBeenCalledWith('task-approve', 'operator');
      expect(api.fetchJSON).toHaveBeenCalledTimes(2);
    });

    expect(within(screen.getByTestId('awaiting-approval-lane')).queryByTestId('awaiting-approval-card')).toBeNull();
    expect(within(screen.getByTestId('approved-lane')).getByText('Approve me')).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Awaiting Approval' }).textContent).toContain('(0)');
    expect(screen.getByRole('heading', { name: 'Approved' }).textContent).toContain('(1)');
  });

  it('removes rejected tasks from Awaiting Approval and shows them in Draft lane after refresh', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON)
      .mockResolvedValueOnce([
        {
          id: 'task-reject',
          kind: 'TaskContract',
          project_id: 'smith',
          provider_profile_id: 'codex-default',
          objective: 'Reject me',
          status: 'validated'
        }
      ])
      .mockResolvedValueOnce([
        {
          id: 'task-reject',
          kind: 'TaskContract',
          project_id: 'smith',
          provider_profile_id: 'codex-default',
          objective: 'Reject me',
          status: 'draft'
        }
      ]);
    vi.mocked(api.patchTaskContract).mockResolvedValue({ id: 'task-reject' } as any);

    render(TasksPage);

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledTimes(1);
    });
    expect(within(screen.getByTestId('awaiting-approval-lane')).getAllByTestId('awaiting-approval-card')).toHaveLength(1);

    await fireEvent.click(screen.getByRole('button', { name: 'Reopen Draft' }));

    await waitFor(() => {
      expect(api.patchTaskContract).toHaveBeenCalledWith('task-reject', {
        status: 'draft',
        actor: 'operator'
      });
      expect(api.fetchJSON).toHaveBeenCalledTimes(2);
    });

    expect(within(screen.getByTestId('awaiting-approval-lane')).queryByTestId('awaiting-approval-card')).toBeNull();
    expect(within(screen.getByTestId('draft-lane')).getByText('Reject me')).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Awaiting Approval' }).textContent).toContain('(0)');
    expect(screen.getByRole('heading', { name: 'Draft' }).textContent).toContain('(1)');
  });

  it('shows Awaiting Approval empty state when no validated tasks exist', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON).mockResolvedValue([
      {
        id: 'task-draft',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'No validated tasks yet',
        status: 'draft'
      }
    ]);

    render(TasksPage);

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledTimes(1);
    });
    expect(screen.getByTestId('awaiting-approval-empty')).toBeTruthy();

    await fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledTimes(2);
    });
  });

  it('hides Awaiting Approval empty state when validated tasks exist', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true
    };
    vi.mocked(api.fetchJSON).mockResolvedValue([
      {
        id: 'task-validated',
        kind: 'TaskContract',
        project_id: 'smith',
        provider_profile_id: 'codex-default',
        objective: 'Ready for review',
        status: 'validated'
      }
    ]);

    render(TasksPage);

    await waitFor(() => {
      expect(api.fetchJSON).toHaveBeenCalledTimes(1);
    });
    expect(screen.queryByTestId('awaiting-approval-empty')).toBeNull();
  });
});
