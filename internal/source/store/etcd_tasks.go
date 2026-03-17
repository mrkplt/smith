package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"smith/internal/source/model"
)

func (s *Store) PutTaskContract(ctx context.Context, task model.TaskContract) error {
	if strings.TrimSpace(task.ID) == "" {
		return errors.New("task id is required")
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	task.SchemaVersion = model.SchemaVersion
	payload, err := json.Marshal(task)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.TaskContractKey(task.ID), string(payload))
	return err
}

func (s *Store) GetTaskContract(ctx context.Context, taskID string) (model.TaskContract, bool, error) {
	resp, err := s.cli.Get(ctx, model.TaskContractKey(taskID))
	if err != nil {
		return model.TaskContract{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return model.TaskContract{}, false, nil
	}
	var task model.TaskContract
	if err := json.Unmarshal(resp.Kvs[0].Value, &task); err != nil {
		return model.TaskContract{}, false, err
	}
	return task, true, nil
}

func (s *Store) ListTaskContracts(ctx context.Context) ([]model.TaskContract, error) {
	resp, err := s.cli.Get(ctx, model.PrefixTasks+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	out := make([]model.TaskContract, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var task model.TaskContract
		if err := json.Unmarshal(kv.Value, &task); err != nil {
			continue
		}
		out = append(out, task)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}
