package store

import (
	"context"
	"time"

	"smith/internal/source/model"
)

func (m *MemStore) NextSequence(ctx context.Context, prefix string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(time.Now().UnixNano()), nil
}

func (m *MemStore) AppendJournal(ctx context.Context, entry model.JournalEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry.Timestamp = time.Now().UTC()
	if entry.Sequence == 0 {
		entry.Sequence = int64(len(m.journal[entry.LoopID]) + 1)
	}
	m.journal[entry.LoopID] = append(m.journal[entry.LoopID], entry)
	for _, w := range m.journalWatchers[entry.LoopID] {
		select {
		case w <- entry:
		default:
		}
	}
	return nil
}

func (m *MemStore) ListJournal(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j := m.journal[loopID]
	if limit > 0 && int64(len(j)) > limit {
		j = j[int64(len(j))-limit:]
	}
	return j, nil
}

func (m *MemStore) ListJournalSinceWithRevision(ctx context.Context, loopID string, sinceSeq int64) ([]model.JournalEntry, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j := m.journal[loopID]
	out := make([]model.JournalEntry, 0)
	for _, e := range j {
		if e.Sequence > sinceSeq {
			out = append(out, e)
		}
	}
	return out, m.revision, nil
}

func (m *MemStore) WatchJournal(ctx context.Context, loopID string) <-chan model.JournalEntry {
	return m.WatchJournalWithRev(ctx, loopID, 0)
}

func (m *MemStore) WatchJournalWithRev(ctx context.Context, loopID string, rev int64) <-chan model.JournalEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan model.JournalEntry, 100)
	m.journalWatchers[loopID] = append(m.journalWatchers[loopID], ch)
	return ch
}

func (m *MemStore) AppendHandoff(ctx context.Context, handoff model.Handoff) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	handoff.Timestamp = time.Now().UTC()
	m.handoffs[handoff.LoopID] = append(m.handoffs[handoff.LoopID], handoff)
	return nil
}

func (m *MemStore) GetLatestHandoff(ctx context.Context, loopID string) (model.Handoff, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.handoffs[loopID]
	if len(h) == 0 {
		return model.Handoff{}, false, nil
	}
	return h[len(h)-1], true, nil
}

func (m *MemStore) ListHandoffs(ctx context.Context, loopID string, limit int64) ([]model.Handoff, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.handoffs[loopID]
	if limit > 0 && int64(len(h)) > limit {
		h = h[int64(len(h))-limit:]
	}
	return h, nil
}

func (m *MemStore) AppendOverride(ctx context.Context, override model.OperatorOverride) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	override.Timestamp = time.Now().UTC()
	m.overrides[override.LoopID] = append(m.overrides[override.LoopID], override)
	return nil
}

func (m *MemStore) ListOverrides(ctx context.Context, loopID string, limit int64) ([]model.OperatorOverride, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o := m.overrides[loopID]
	if limit > 0 && int64(len(o)) > limit {
		o = o[int64(len(o))-limit:]
	}
	return o, nil
}

func (m *MemStore) RecordPhase(ctx context.Context, record model.JournalEntry) error {
	return m.AppendJournal(ctx, record)
}
