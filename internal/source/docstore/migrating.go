package docstore

import (
	"context"
	"fmt"
	"sort"

	"smith/internal/source/model"
)

type MigrationOptions struct {
	DualWriteFallback bool
	ReadThroughOnMiss bool
	MergeListFallback bool
}

type MigratingStore struct {
	primary  Store
	fallback Store
	options  MigrationOptions
}

func NewMigratingStore(primary Store, fallback Store, options MigrationOptions) *MigratingStore {
	return &MigratingStore{
		primary:  primary,
		fallback: fallback,
		options:  options,
	}
}

func (m *MigratingStore) Close() error {
	var closeErr error
	if m.primary != nil {
		if err := m.primary.Close(); err != nil {
			closeErr = err
		}
	}
	if m.fallback != nil {
		if err := m.fallback.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (m *MigratingStore) PutDocument(ctx context.Context, doc model.Document) error {
	if m.primary == nil {
		return fmt.Errorf("primary document store unavailable")
	}
	if err := m.primary.PutDocument(ctx, doc); err != nil {
		return err
	}
	if m.options.DualWriteFallback && m.fallback != nil {
		if err := m.fallback.PutDocument(ctx, doc); err != nil {
			return fmt.Errorf("fallback dual-write failed: %w", err)
		}
	}
	return nil
}

func (m *MigratingStore) GetDocument(ctx context.Context, docID string) (model.Document, bool, error) {
	if m.primary != nil {
		doc, found, err := m.primary.GetDocument(ctx, docID)
		if err != nil {
			return model.Document{}, false, err
		}
		if found {
			return doc, true, nil
		}
	}
	if m.fallback == nil {
		return model.Document{}, false, nil
	}
	doc, found, err := m.fallback.GetDocument(ctx, docID)
	if err != nil {
		return model.Document{}, false, err
	}
	if !found {
		return model.Document{}, false, nil
	}
	if m.options.ReadThroughOnMiss && m.primary != nil {
		_ = m.primary.PutDocument(ctx, doc)
	}
	return doc, true, nil
}

func (m *MigratingStore) ListDocuments(ctx context.Context) ([]model.Document, error) {
	primaryDocs := make([]model.Document, 0)
	if m.primary != nil {
		docs, err := m.primary.ListDocuments(ctx)
		if err != nil {
			return nil, err
		}
		primaryDocs = docs
	}
	if !m.options.MergeListFallback || m.fallback == nil {
		return primaryDocs, nil
	}
	fallbackDocs, err := m.fallback.ListDocuments(ctx)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]model.Document, len(primaryDocs)+len(fallbackDocs))
	for _, doc := range primaryDocs {
		merged[doc.ID] = doc
	}
	for _, doc := range fallbackDocs {
		existing, found := merged[doc.ID]
		if !found || doc.UpdatedAt.After(existing.UpdatedAt) {
			merged[doc.ID] = doc
		}
		if m.options.ReadThroughOnMiss && m.primary != nil && (!found || doc.UpdatedAt.After(existing.UpdatedAt)) {
			_ = m.primary.PutDocument(ctx, doc)
		}
	}

	out := make([]model.Document, 0, len(merged))
	for _, doc := range merged {
		out = append(out, doc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (m *MigratingStore) DeleteDocument(ctx context.Context, docID string) error {
	if m.primary == nil {
		return fmt.Errorf("primary document store unavailable")
	}
	if err := m.primary.DeleteDocument(ctx, docID); err != nil {
		return err
	}
	if m.options.DualWriteFallback && m.fallback != nil {
		if err := m.fallback.DeleteDocument(ctx, docID); err != nil {
			return fmt.Errorf("fallback dual-delete failed: %w", err)
		}
	}
	return nil
}

func (m *MigratingStore) WatchDocuments(ctx context.Context) <-chan model.Document {
	if m.primary == nil {
		closed := make(chan model.Document)
		close(closed)
		return closed
	}
	return m.primary.WatchDocuments(ctx)
}

type BackfillSummary struct {
	Scanned  int
	Upserted int
}

func BackfillDocuments(ctx context.Context, target Store, source Store) (BackfillSummary, error) {
	if target == nil || source == nil {
		return BackfillSummary{}, nil
	}
	sourceDocs, err := source.ListDocuments(ctx)
	if err != nil {
		return BackfillSummary{}, err
	}
	targetDocs, err := target.ListDocuments(ctx)
	if err != nil {
		return BackfillSummary{}, err
	}
	targetByID := make(map[string]model.Document, len(targetDocs))
	for _, doc := range targetDocs {
		targetByID[doc.ID] = doc
	}

	summary := BackfillSummary{Scanned: len(sourceDocs)}
	for _, doc := range sourceDocs {
		existing, found := targetByID[doc.ID]
		if found && (existing.UpdatedAt.After(doc.UpdatedAt) || existing.UpdatedAt.Equal(doc.UpdatedAt)) {
			continue
		}
		if err := target.PutDocument(ctx, doc); err != nil {
			return summary, fmt.Errorf("backfill document %s: %w", doc.ID, err)
		}
		summary.Upserted++
	}
	return summary, nil
}
