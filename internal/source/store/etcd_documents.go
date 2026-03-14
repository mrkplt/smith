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

func (s *Store) PutDocument(ctx context.Context, doc model.Document) error {
	if strings.TrimSpace(doc.ID) == "" {
		return errors.New("document id is required")
	}
	doc.SchemaVersion = model.SchemaVersion
	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	payload, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.DocumentKey(doc.ID), string(payload))
	return err
}

func (s *Store) GetDocument(ctx context.Context, docID string) (model.Document, bool, error) {
	resp, err := s.cli.Get(ctx, model.DocumentKey(docID))
	if err != nil {
		return model.Document{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return model.Document{}, false, nil
	}
	var doc model.Document
	if err := json.Unmarshal(resp.Kvs[0].Value, &doc); err != nil {
		return model.Document{}, false, err
	}
	return doc, true, nil
}

func (s *Store) ListDocuments(ctx context.Context) ([]model.Document, error) {
	resp, err := s.cli.Get(ctx, model.PrefixDocuments+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	out := make([]model.Document, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var doc model.Document
		if err := json.Unmarshal(kv.Value, &doc); err != nil {
			continue
		}
		out = append(out, doc)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *Store) DeleteDocument(ctx context.Context, docID string) error {
	if strings.TrimSpace(docID) == "" {
		return errors.New("document id is required")
	}
	_, err := s.cli.Delete(ctx, model.DocumentKey(docID))
	return err
}

func (s *Store) AppendAudit(ctx context.Context, rec AuditRecord) error {
	if rec.EventID == "" {
		rec.EventID = fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	rec.SchemaVersion = model.SchemaVersion
	rec.Timestamp = time.Now().UTC()
	key := fmt.Sprintf("%s/%04d/%02d/%02d/%s", model.PrefixAudit, rec.Timestamp.Year(), rec.Timestamp.Month(), rec.Timestamp.Day(), rec.EventID)
	payload, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, key, string(payload))
	return err
}

func (s *Store) ListAudit(ctx context.Context, loopID string, limit int64) ([]AuditRecord, error) {
	opts := []clientv3.OpOption{
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend),
	}
	resp, err := s.cli.Get(ctx, model.PrefixAudit+"/", opts...)
	if err != nil {
		return nil, err
	}
	records := make([]AuditRecord, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var rec AuditRecord
		if err := json.Unmarshal(kv.Value, &rec); err != nil {
			continue
		}
		if loopID != "" && rec.TargetLoopID != loopID {
			continue
		}
		records = append(records, rec)
		if limit > 0 && int64(len(records)) >= limit {
			break
		}
	}
	return records, nil
}

func (s *Store) WatchDocuments(ctx context.Context) <-chan model.Document {
	out := make(chan model.Document)
	watchCh := s.cli.Watch(ctx, model.PrefixDocuments+"/", clientv3.WithPrefix())
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
				var doc model.Document
				if err := json.Unmarshal(event.Kv.Value, &doc); err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- doc:
				}
			}
		}
	}()
	return out
}

func (s *Store) WatchAudit(ctx context.Context) <-chan AuditRecord {
	out := make(chan AuditRecord)
	watchCh := s.cli.Watch(ctx, model.PrefixAudit+"/", clientv3.WithPrefix())
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
				var rec AuditRecord
				if err := json.Unmarshal(event.Kv.Value, &rec); err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- rec:
				}
			}
		}
	}()
	return out
}
