import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@testing-library/svelte';

import Sidebar from './Sidebar.svelte';

vi.mock('$app/state', () => ({
  page: {
    url: new URL('http://localhost/tasks')
  }
}));

vi.mock('$lib/feature-capability/access', () => ({
  hasFeatureCapabilityAccess: () => true
}));

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn()
  }))
});

describe('Sidebar Tasks Kanban gate', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    (window as any).__SMITH_CONFIG__ = {};
  });

  afterEach(() => {
    cleanup();
  });

  it('hides Tasks Kanban links when feature flag is disabled', () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: false,
      featureCapabilityEnabled: false
    };

    const { container } = render(Sidebar);
    expect(container.querySelector('a[href="/tasks/kanban"]')).toBeNull();
  });

  it('shows Tasks Kanban links when tasks and kanban flags are enabled', () => {
    (window as any).__SMITH_CONFIG__ = {
      featureTasksEnabled: true,
      featureTasksKanbanEnabled: true,
      featureCapabilityEnabled: false
    };

    const { container } = render(Sidebar);
    expect(container.querySelector('a[href="/tasks/kanban"]')).toBeTruthy();
  });
});
