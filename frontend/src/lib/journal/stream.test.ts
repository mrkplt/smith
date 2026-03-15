import { beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';

import { appState } from '$lib/stores';
import { appendJournalEntry, openJournalStream, renderJournalText } from '$lib/journal/stream';

class MockEventSource {
  url: string;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;

  constructor(url: string) {
    this.url = url;
  }

  close() {
    this.closed = true;
  }
}

describe('journal stream helpers', () => {
  beforeEach(() => {
    appState.set({
      ...get(appState),
      journalEntries: [],
      journalLastSeq: 0
    });
    vi.stubGlobal('EventSource', MockEventSource);
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  it('appends only newer journal entries and caps the buffer', () => {
    appendJournalEntry({ sequence: 1, message: 'first' });
    appendJournalEntry({ sequence: 1, message: 'duplicate' });
    appendJournalEntry({ Sequence: 2, Message: 'second' });

    const state = get(appState);
    expect(state.journalLastSeq).toBe(2);
    expect(state.journalEntries).toHaveLength(2);
  });

  it('renders human-readable journal text', () => {
    expect(renderJournalText([])).toBe('[journal] waiting for entries...\n');
    expect(renderJournalText([{
      timestamp: '2026-03-14T20:00:00Z',
      level: 'INFO',
      phase: 'build',
      actor_id: 'worker-1',
      message: 'done'
    }])).toBe('[2026-03-14T20:00:00Z] [info] [build] [worker-1] done\n');
  });

  it('opens the journal stream and routes entries and errors', () => {
    const entries: any[] = [];
    const onError = vi.fn();

    const source = openJournalStream('alpha/loop', entry => entries.push(entry), onError) as unknown as MockEventSource;
    expect(source.url).toBe('/api/v1/loops/alpha%2Floop/journal/stream');

    source.onmessage?.({ data: JSON.stringify({ id: 1 }) } as MessageEvent<string>);
    source.onmessage?.({ data: 'bad-json' } as MessageEvent<string>);
    source.onerror?.();

    expect(entries).toEqual([{ id: 1 }]);
    expect(source.closed).toBe(true);
    expect(onError).toHaveBeenCalled();
  });
});
