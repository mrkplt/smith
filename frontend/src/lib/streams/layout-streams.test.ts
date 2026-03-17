import { beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';

const layoutMocks = vi.hoisted(() => ({
  fetchJSON: vi.fn(),
  pushToast: vi.fn()
}));

vi.mock('$lib/api', () => ({
  apiBaseUrl: '/api',
  fetchJSON: layoutMocks.fetchJSON
}));

vi.mock('$lib/stores', async () => {
  const actual = await vi.importActual<typeof import('$lib/stores')>('$lib/stores');
  return {
    ...actual,
    pushToast: layoutMocks.pushToast
  };
});

import { appState } from '$lib/stores';
import { connectLayoutStreams, initLayoutState, normalizeLoop } from '$lib/streams/layout-streams';

class MockEventSource {
  static instances: MockEventSource[] = [];

  url: string;
  listeners = new Map<string, (event: MessageEvent<string>) => void>();
  closed = false;

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  addEventListener(type: string, cb: (event: MessageEvent<string>) => void) {
    this.listeners.set(type, cb);
  }

  emit(type: string, data: unknown) {
    this.listeners.get(type)?.({ data: JSON.stringify(data) } as MessageEvent<string>);
  }

  close() {
    this.closed = true;
  }
}

describe('layout stream helpers', () => {
  beforeEach(() => {
    MockEventSource.instances = [];
    layoutMocks.fetchJSON.mockReset();
    layoutMocks.pushToast.mockReset();
    appState.set({
      ...get(appState),
      loops: [],
      documents: [],
      projects: []
    });
    vi.stubGlobal('EventSource', MockEventSource);
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  it('normalizes loop payload variants', () => {
    expect(normalizeLoop({
      Record: {
        LoopID: 'loop-1',
        State: 'RUNNING',
        Attempt: 2,
        Reason: 'busy',
        project_name: 'alpha'
      },
      Revision: 9
    })).toEqual({
      loopID: 'loop-1',
      displayTitle: 'loop-1',
      currentCount: 2,
      targetCount: 2,
      project: 'alpha',
      status: 'running',
      attempt: 2,
      reason: 'busy',
      revision: 9
    });
  });

  it('loads initial projects into app state', async () => {
    layoutMocks.fetchJSON.mockResolvedValue([{ id: 'project-1' }]);

    await initLayoutState();

    expect(get(appState).projects).toEqual([{ id: 'project-1' }]);
  });

  it('connects streams, updates store state, and disposes sources', () => {
    const dispose = connectLayoutStreams();

    const [loopsSource, docsSource, auditSource] = MockEventSource.instances;
    expect(loopsSource.url).toBe('/api/v1/loops/stream');
    expect(docsSource.url).toBe('/api/v1/documents/stream');
    expect(auditSource.url).toBe('/api/v1/audit/stream');
    loopsSource.emit('update', {
      record: {
        loop_id: 'loop-1',
        state: 'RUNNING',
        attempt: 3,
        reason: 'syncing',
        project_id: 'alpha'
      },
      revision: 5
    });
    docsSource.emit('update', { id: 'doc-1', title: 'Draft' });
    auditSource.emit('update', { action: 'resume', target_loop_id: 'loop-1' });

    expect(get(appState).loops).toEqual([{
      loopID: 'loop-1',
      displayTitle: 'loop-1',
      currentCount: 3,
      targetCount: 3,
      project: 'alpha',
      status: 'running',
      attempt: 3,
      reason: 'syncing',
      revision: 5
    }]);
    expect(get(appState).documents).toEqual([{ id: 'doc-1', title: 'Draft' }]);
    expect(layoutMocks.pushToast).toHaveBeenCalledWith('[resume] loop-1', 'muted');

    dispose();

    expect(loopsSource.closed).toBe(true);
    expect(docsSource.closed).toBe(true);
    expect(auditSource.closed).toBe(true);
  });
});
