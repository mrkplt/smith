import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';

import * as stores from '$lib/stores';
import TasksKanbanPage from './+page.svelte';
import { goto } from '$app/navigation';

vi.mock('$app/navigation', () => ({
  goto: vi.fn()
}));

vi.mock('$lib/stores', async (importOriginal) => {
  const original = await importOriginal<typeof import('$lib/stores')>();
  return {
    ...original,
    pushToast: vi.fn()
  };
});

describe('Tasks Kanban route gate', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (window as any).__SMITH_CONFIG__ = {};
  });

  it('redirects to /tasks when Tasks Kanban gate is disabled', async () => {
    const infoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: false
    };

    render(TasksKanbanPage);

    await waitFor(() => {
      expect(goto).toHaveBeenCalledWith('/tasks', { replaceState: true });
    });
    expect(stores.pushToast).toHaveBeenCalledWith('Tasks Kanban is not enabled in this environment', 'muted');
    expect(infoSpy).toHaveBeenCalledWith('[feature-flag] feature=tasks-kanban action=route-block redirect=/tasks reason=disabled');
    infoSpy.mockRestore();
  });

  it('renders Kanban content when Tasks and Kanban gates are enabled', async () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true
    };

    render(TasksKanbanPage);

    await waitFor(() => {
      expect(screen.getByText('Kanban workspace is enabled for this environment.')).toBeTruthy();
    });
    expect(goto).not.toHaveBeenCalled();
  });
});
