package store

import (
	"sync"

	"smith/internal/source/model"
)

type MemStore struct {
	mu        sync.RWMutex
	states    map[string]LoopWithRevision
	anomalies map[string]model.Anomaly
	docs      map[string]model.Document
	tasks     map[string]model.TaskContract
	journal   map[string][]model.JournalEntry
	handoffs  map[string][]model.Handoff
	overrides map[string][]model.OperatorOverride
	audit     []AuditRecord
	locks     map[string]entry
	revision  int64

	stateWatchers   []chan Event
	docWatchers     []chan model.Document
	journalWatchers map[string][]chan model.JournalEntry
	auditWatchers   []chan AuditRecord
}

func NewMemStore() *MemStore {
	return &MemStore{
		states:          make(map[string]LoopWithRevision),
		anomalies:       make(map[string]model.Anomaly),
		docs:            make(map[string]model.Document),
		tasks:           make(map[string]model.TaskContract),
		journal:         make(map[string][]model.JournalEntry),
		handoffs:        make(map[string][]model.Handoff),
		overrides:       make(map[string][]model.OperatorOverride),
		locks:           make(map[string]entry),
		journalWatchers: make(map[string][]chan model.JournalEntry),
	}
}

func (m *MemStore) Close() error {
	return nil
}

type entry struct {
	lock     model.LeaseLock
	revision int64
}
