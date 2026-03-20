package docstore

import (
	"context"

	"smith/internal/source/model"
	"smith/internal/source/store"
)

type Store interface {
	Close() error
	PutDocument(ctx context.Context, doc model.Document) error
	GetDocument(ctx context.Context, docID string) (model.Document, bool, error)
	ListDocuments(ctx context.Context) ([]model.Document, error)
	DeleteDocument(ctx context.Context, docID string) error
	WatchDocuments(ctx context.Context) <-chan model.Document
}

type EtcdStore struct {
	store store.StateStore
}

func NewEtcdStore(delegate store.StateStore) *EtcdStore {
	return &EtcdStore{store: delegate}
}

func (e *EtcdStore) Close() error {
	return nil
}

func (e *EtcdStore) PutDocument(ctx context.Context, doc model.Document) error {
	return e.store.PutDocument(ctx, doc)
}

func (e *EtcdStore) GetDocument(ctx context.Context, docID string) (model.Document, bool, error) {
	return e.store.GetDocument(ctx, docID)
}

func (e *EtcdStore) ListDocuments(ctx context.Context) ([]model.Document, error) {
	return e.store.ListDocuments(ctx)
}

func (e *EtcdStore) DeleteDocument(ctx context.Context, docID string) error {
	return e.store.DeleteDocument(ctx, docID)
}

func (e *EtcdStore) WatchDocuments(ctx context.Context) <-chan model.Document {
	return e.store.WatchDocuments(ctx)
}
