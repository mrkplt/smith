package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"smith/internal/source/model"
)

func (s *Store) NextSequence(ctx context.Context, prefix string) (int64, error) {
	resp, err := s.cli.Get(ctx, prefix+"/", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend), clientv3.WithLimit(1))
	if err != nil {
		return 0, err
	}
	if len(resp.Kvs) == 0 {
		return 1, nil
	}
	base := path.Base(string(resp.Kvs[0].Key))
	var seq int64
	_, scanErr := fmt.Sscanf(base, "%d", &seq)
	if scanErr != nil {
		return 1, nil
	}
	return seq + 1, nil
}

func (s *Store) AppendJournal(ctx context.Context, entry model.JournalEntry) error {
	entry.SchemaVersion = model.SchemaVersion
	entry.Timestamp = time.Now().UTC()
	if entry.Sequence == 0 {
		seq, err := s.NextSequence(ctx, model.JournalPrefix(entry.LoopID))
		if err != nil {
			return err
		}
		entry.Sequence = seq
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.JournalKey(entry.LoopID, entry.Sequence), string(payload))
	return err
}

func (s *Store) ListJournal(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error) {
	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)}
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(limit))
	}
	resp, err := s.cli.Get(ctx, model.JournalPrefix(loopID)+"/", opts...)
	if err != nil {
		return nil, err
	}
	entries := make([]model.JournalEntry, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var entry model.JournalEntry
		if unmarshalErr := json.Unmarshal(kv.Value, &entry); unmarshalErr != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *Store) ListJournalSinceWithRevision(ctx context.Context, loopID string, sinceSeq int64) ([]model.JournalEntry, int64, error) {
	resp, err := s.cli.Get(
		ctx,
		model.JournalPrefix(loopID)+"/",
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
	)
	if err != nil {
		return nil, 0, err
	}
	entries := make([]model.JournalEntry, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var entry model.JournalEntry
		if err := json.Unmarshal(kv.Value, &entry); err != nil {
			continue
		}
		if entry.Sequence <= sinceSeq {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, resp.Header.Revision, nil
}

func (s *Store) WatchJournal(ctx context.Context, loopID string) <-chan model.JournalEntry {
	return s.WatchJournalWithRev(ctx, loopID, 0)
}

func (s *Store) WatchJournalWithRev(ctx context.Context, loopID string, rev int64) <-chan model.JournalEntry {
	out := make(chan model.JournalEntry)
	prefix := model.JournalPrefix(loopID) + "/"
	opts := []clientv3.OpOption{clientv3.WithPrefix()}
	if rev > 0 {
		opts = append(opts, clientv3.WithRev(rev))
	}
	watchCh := s.cli.Watch(ctx, prefix, opts...)
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
				var entry model.JournalEntry
				if err := json.Unmarshal(event.Kv.Value, &entry); err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- entry:
				}
			}
		}
	}()
	return out
}

func (s *Store) AppendHandoff(ctx context.Context, handoff model.Handoff) error {
	handoff.SchemaVersion = model.SchemaVersion
	handoff.Timestamp = time.Now().UTC()
	if handoff.Sequence == 0 {
		seq, err := s.NextSequence(ctx, model.HandoffPrefix(handoff.LoopID))
		if err != nil {
			return err
		}
		handoff.Sequence = seq
	}
	payload, err := json.Marshal(handoff)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.HandoffKey(handoff.LoopID, handoff.Sequence), string(payload))
	return err
}

func (s *Store) GetLatestHandoff(ctx context.Context, loopID string) (model.Handoff, bool, error) {
	loopID = strings.TrimSpace(loopID)
	if loopID == "" {
		return model.Handoff{}, false, errors.New("loop id is required")
	}
	resp, err := s.cli.Get(
		ctx,
		model.HandoffPrefix(loopID)+"/",
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend),
		clientv3.WithLimit(1),
	)
	if err != nil {
		return model.Handoff{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return model.Handoff{}, false, nil
	}
	var handoff model.Handoff
	if err := json.Unmarshal(resp.Kvs[0].Value, &handoff); err != nil {
		return model.Handoff{}, false, err
	}
	return handoff, true, nil
}

func (s *Store) ListHandoffs(ctx context.Context, loopID string, limit int64) ([]model.Handoff, error) {
	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)}
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(limit))
	}
	resp, err := s.cli.Get(ctx, model.HandoffPrefix(loopID)+"/", opts...)
	if err != nil {
		return nil, err
	}
	out := make([]model.Handoff, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var handoff model.Handoff
		if err := json.Unmarshal(kv.Value, &handoff); err != nil {
			continue
		}
		out = append(out, handoff)
	}
	return out, nil
}

func (s *Store) AppendOverride(ctx context.Context, override model.OperatorOverride) error {
	override.SchemaVersion = model.SchemaVersion
	override.Timestamp = time.Now().UTC()
	if override.Sequence == 0 {
		seq, err := s.NextSequence(ctx, model.OverridePrefix(override.LoopID))
		if err != nil {
			return err
		}
		override.Sequence = seq
	}
	payload, err := json.Marshal(override)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.OverrideKey(override.LoopID, override.Sequence), string(payload))
	return err
}

func (s *Store) ListOverrides(ctx context.Context, loopID string, limit int64) ([]model.OperatorOverride, error) {
	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)}
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(limit))
	}
	resp, err := s.cli.Get(ctx, model.OverridePrefix(loopID)+"/", opts...)
	if err != nil {
		return nil, err
	}
	out := make([]model.OperatorOverride, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var override model.OperatorOverride
		if err := json.Unmarshal(kv.Value, &override); err != nil {
			continue
		}
		out = append(out, override)
	}
	return out, nil
}

func (s *Store) RecordPhase(ctx context.Context, record model.JournalEntry) error {
	return s.AppendJournal(ctx, record)
}
