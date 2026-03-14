package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"smith/internal/source/model"
)

func (m *MemStore) ListStates(ctx context.Context) ([]LoopWithRevision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]LoopWithRevision, 0, len(m.states))
	for _, s := range m.states {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Record.UpdatedAt.Before(out[j].Record.UpdatedAt)
	})
	return out, nil
}

func (m *MemStore) GetState(ctx context.Context, loopID string) (LoopWithRevision, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.states[loopID]
	return s, ok, nil
}

func (m *MemStore) PutState(ctx context.Context, rec model.StateRecord, expectedRevision int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.states[rec.LoopID]
	if (expectedRevision == 0 && ok) || (expectedRevision != 0 && current.Revision != expectedRevision) {
		return 0, ErrRevisionMismatch
	}

	m.revision++
	rec.UpdatedAt = time.Now().UTC()
	m.states[rec.LoopID] = LoopWithRevision{Record: rec, Revision: m.revision}

	event := Event{
		LoopID:   rec.LoopID,
		State:    rec,
		Revision: m.revision,
		HasState: true,
	}
	for _, w := range m.stateWatchers {
		select {
		case w <- event:
		default:
		}
	}
	return m.revision, nil
}

func (m *MemStore) PutStateFromCurrent(ctx context.Context, loopID string, mutate func(current model.StateRecord) (model.StateRecord, error)) (LoopWithRevision, error) {
	current, found, err := m.GetState(ctx, loopID)
	if err != nil {
		return LoopWithRevision{}, err
	}
	if !found {
		return LoopWithRevision{}, fmt.Errorf("state not found for loop %s", loopID)
	}
	next, err := mutate(current.Record)
	if err != nil {
		return LoopWithRevision{}, err
	}
	rev, err := m.PutState(ctx, next, current.Revision)
	if err != nil {
		return LoopWithRevision{}, err
	}
	next.ObservedRevision = rev
	return LoopWithRevision{Record: next, Revision: rev}, nil
}

func (m *MemStore) DeleteLoop(ctx context.Context, loopID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, loopID)
	delete(m.anomalies, loopID)
	delete(m.locks, loopID)
	delete(m.journal, loopID)
	delete(m.handoffs, loopID)
	delete(m.overrides, loopID)
	return nil
}

func (m *MemStore) WatchState(ctx context.Context) <-chan Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan Event, 100)
	m.stateWatchers = append(m.stateWatchers, ch)
	return ch
}

func (m *MemStore) PutAnomaly(ctx context.Context, anomaly model.Anomaly) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	anomaly.UpdatedAt = time.Now().UTC()
	m.anomalies[anomaly.ID] = anomaly
	return nil
}

func (m *MemStore) GetAnomaly(ctx context.Context, loopID string) (model.Anomaly, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.anomalies[loopID]
	return a, ok, nil
}

func (m *MemStore) SetStateSynced(ctx context.Context, loopID string, commitSHA string) error {
	_, err := m.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		current.State = model.LoopStateSynced
		current.Reason = fmt.Sprintf("completion-saga-succeeded: commit=%s", commitSHA)
		return current, nil
	})
	return err
}

func (m *MemStore) SetStateUnresolved(ctx context.Context, loopID string, reason string) error {
	_, err := m.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		current.State = model.LoopStateUnresolved
		current.Reason = reason
		return current, nil
	})
	return err
}
