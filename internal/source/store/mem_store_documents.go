package store

import (
	"context"
	"sort"
	"time"

	"smith/internal/source/model"
)

func (m *MemStore) PutDocument(ctx context.Context, doc model.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc.UpdatedAt = time.Now().UTC()
	m.docs[doc.ID] = doc
	for _, w := range m.docWatchers {
		select {
		case w <- doc:
		default:
		}
	}
	return nil
}

func (m *MemStore) GetDocument(ctx context.Context, docID string) (model.Document, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.docs[docID]
	return d, ok, nil
}

func (m *MemStore) ListDocuments(ctx context.Context) ([]model.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Document, 0, len(m.docs))
	for _, d := range m.docs {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (m *MemStore) DeleteDocument(ctx context.Context, docID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs, docID)
	return nil
}

func (m *MemStore) WatchDocuments(ctx context.Context) <-chan model.Document {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan model.Document, 100)
	m.docWatchers = append(m.docWatchers, ch)
	return ch
}

func (m *MemStore) AppendAudit(ctx context.Context, rec AuditRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec.Timestamp = time.Now().UTC()
	m.audit = append(m.audit, rec)
	for _, w := range m.auditWatchers {
		select {
		case w <- rec:
		default:
		}
	}
	return nil
}

func (m *MemStore) ListAudit(ctx context.Context, loopID string, limit int64) ([]AuditRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AuditRecord, 0)
	for i := len(m.audit) - 1; i >= 0; i-- {
		if loopID == "" || m.audit[i].TargetLoopID == loopID {
			out = append(out, m.audit[i])
		}
		if limit > 0 && int64(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemStore) WatchAudit(ctx context.Context) <-chan AuditRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan AuditRecord, 100)
	m.auditWatchers = append(m.auditWatchers, ch)
	return ch
}
