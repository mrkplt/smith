package store

import (
	"context"
	"sort"
	"time"

	"smith/internal/source/model"
)

func (m *MemStore) PutTaskContract(ctx context.Context, task model.TaskContract) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	task.SchemaVersion = model.SchemaVersion
	m.tasks[task.ID] = task
	return nil
}

func (m *MemStore) GetTaskContract(ctx context.Context, taskID string) (model.TaskContract, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[taskID]
	return task, ok, nil
}

func (m *MemStore) ListTaskContracts(ctx context.Context) ([]model.TaskContract, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.TaskContract, 0, len(m.tasks))
	for _, task := range m.tasks {
		out = append(out, task)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}
