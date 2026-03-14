package store

import (
	"context"

	"smith/internal/source/locking"
	"smith/internal/source/model"
)

func (m *MemStore) ReadLock(ctx context.Context, loopID string) (locking.Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.locks[loopID]
	if !ok {
		return locking.Record{Found: false}, nil
	}
	return locking.Record{Found: true, Lock: l.lock, Revision: l.revision}, nil
}

func (m *MemStore) PutLockIfRevision(ctx context.Context, lock model.LeaseLock, expectedRevision int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.locks[lock.LoopID]
	if !ok {
		if expectedRevision != 0 {
			return false, nil
		}
		m.revision++
		m.locks[lock.LoopID] = entry{lock: lock, revision: m.revision}
		return true, nil
	}
	if current.revision != expectedRevision {
		return false, nil
	}
	m.revision++
	m.locks[lock.LoopID] = entry{lock: lock, revision: m.revision}
	return true, nil
}

func (m *MemStore) DeleteLockIfRevision(ctx context.Context, loopID string, expectedRevision int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.locks[loopID]
	if !ok {
		return false, nil
	}
	if current.revision != expectedRevision {
		return false, nil
	}
	delete(m.locks, loopID)
	return true, nil
}
