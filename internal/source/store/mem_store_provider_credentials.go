package store

import (
	"context"
	"time"

	"smith/internal/source/model"
)

func (m *MemStore) GetProviderCredential(_ context.Context, providerID string) (model.ProviderCredential, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cred, ok := m.providerCredentials[normalizeProviderID(providerID)]
	return cred, ok, nil
}

func (m *MemStore) PutProviderCredential(_ context.Context, providerID string, cred model.ProviderCredential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cred.UpdatedAt.IsZero() {
		cred.UpdatedAt = time.Now().UTC()
	}
	m.providerCredentials[normalizeProviderID(providerID)] = cred
	return nil
}

func (m *MemStore) DeleteProviderCredential(_ context.Context, providerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.providerCredentials, normalizeProviderID(providerID))
	return nil
}
