package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"smith/internal/source/model"
)

func (s *Store) ListStates(ctx context.Context) ([]LoopWithRevision, error) {
	resp, err := s.cli.Get(ctx, model.PrefixState+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	out := make([]LoopWithRevision, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		rec, err := model.DecodeStateRecord(kv.Value)
		if err != nil {
			continue
		}
		out = append(out, LoopWithRevision{Record: rec, Revision: kv.ModRevision})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Record.UpdatedAt.Before(out[j].Record.UpdatedAt)
	})
	return out, nil
}

func (s *Store) PutAnomaly(ctx context.Context, anomaly model.Anomaly) error {
	if strings.TrimSpace(anomaly.ID) == "" {
		return errors.New("anomaly id is required")
	}
	anomaly.SchemaVersion = model.SchemaVersion
	now := time.Now().UTC()
	if anomaly.CreatedAt.IsZero() {
		anomaly.CreatedAt = now
	}
	anomaly.UpdatedAt = now
	payload, err := json.Marshal(anomaly)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.AnomalyKey(anomaly.ID), string(payload))
	return err
}

func (s *Store) GetAnomaly(ctx context.Context, loopID string) (model.Anomaly, bool, error) {
	resp, err := s.cli.Get(ctx, model.AnomalyKey(loopID))
	if err != nil {
		return model.Anomaly{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return model.Anomaly{}, false, nil
	}
	var anomaly model.Anomaly
	if err := json.Unmarshal(resp.Kvs[0].Value, &anomaly); err != nil {
		return model.Anomaly{}, false, err
	}
	return anomaly, true, nil
}

func (s *Store) GetState(ctx context.Context, loopID string) (LoopWithRevision, bool, error) {
	key := model.StateKey(loopID)
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return LoopWithRevision{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return LoopWithRevision{}, false, nil
	}
	rec, err := model.DecodeStateRecord(resp.Kvs[0].Value)
	if err != nil {
		return LoopWithRevision{}, false, err
	}
	return LoopWithRevision{Record: rec, Revision: resp.Kvs[0].ModRevision}, true, nil
}

func (s *Store) PutState(ctx context.Context, rec model.StateRecord, expectedRevision int64) (int64, error) {
	if strings.TrimSpace(rec.LoopID) == "" {
		return 0, errors.New("loop id is required")
	}
	rec.SchemaVersion = model.SchemaVersion
	rec.UpdatedAt = time.Now().UTC()
	payload, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}

	key := model.StateKey(rec.LoopID)
	cmp := clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision)
	if expectedRevision == 0 {
		cmp = clientv3.Compare(clientv3.Version(key), "=", 0)
	}

	txnResp, err := s.cli.Txn(ctx).
		If(cmp).
		Then(clientv3.OpPut(key, string(payload))).
		Else(clientv3.OpGet(key)).
		Commit()
	if err != nil {
		return 0, err
	}
	if !txnResp.Succeeded {
		return 0, ErrRevisionMismatch
	}

	getResp, err := s.cli.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if len(getResp.Kvs) == 0 {
		return 0, errors.New("state write succeeded but value missing")
	}
	return getResp.Kvs[0].ModRevision, nil
}

func (s *Store) PutStateFromCurrent(ctx context.Context, loopID string, mutate func(current model.StateRecord) (model.StateRecord, error)) (LoopWithRevision, error) {
	current, found, err := s.GetState(ctx, loopID)
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
	rev, err := s.PutState(ctx, next, current.Revision)
	if err != nil {
		return LoopWithRevision{}, err
	}
	next.ObservedRevision = rev
	return LoopWithRevision{Record: next, Revision: rev}, nil
}

func (s *Store) DeleteLoop(ctx context.Context, loopID string) error {
	id := strings.TrimSpace(loopID)
	if id == "" {
		return errors.New("loop id is required")
	}
	ops := []clientv3.Op{
		clientv3.OpDelete(model.StateKey(id)),
		clientv3.OpDelete(model.AnomalyKey(id)),
		clientv3.OpDelete(model.LockKey(id)),
		clientv3.OpDelete(model.JournalPrefix(id)+"/", clientv3.WithPrefix()),
		clientv3.OpDelete(model.HandoffPrefix(id)+"/", clientv3.WithPrefix()),
		clientv3.OpDelete(model.OverridePrefix(id)+"/", clientv3.WithPrefix()),
	}
	_, err := s.cli.Txn(ctx).Then(ops...).Commit()
	return err
}

func (s *Store) WatchState(ctx context.Context) <-chan Event {
	out := make(chan Event)
	watchCh := s.cli.Watch(ctx, model.PrefixState+"/", clientv3.WithPrefix())
	go func() {
		defer close(out)
		for watchResp := range watchCh {
			if watchResp.Err() != nil {
				continue
			}
			for _, event := range watchResp.Events {
				if event.Type != clientv3.EventTypePut || len(event.Kv.Value) == 0 {
					continue
				}
				rec, err := model.DecodeStateRecord(event.Kv.Value)
				if err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- Event{
					LoopID:   rec.LoopID,
					State:    rec,
					Revision: event.Kv.ModRevision,
					HasState: true,
					RawKey:   string(event.Kv.Key),
					RawValue: event.Kv.Value,
				}:
				}
			}
		}
	}()
	return out
}

func (s *Store) SetStateSynced(ctx context.Context, loopID string, commitSHA string) error {
	_, err := s.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		current.State = model.LoopStateSynced
		current.Reason = fmt.Sprintf("completion-saga-succeeded: commit=%s", commitSHA)
		return current, nil
	})
	return err
}

func (s *Store) SetStateUnresolved(ctx context.Context, loopID string, reason string) error {
	_, err := s.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		current.State = model.LoopStateUnresolved
		current.Reason = reason
		return current, nil
	})
	return err
}
