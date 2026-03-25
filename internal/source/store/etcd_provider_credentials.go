package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"smith/internal/source/model"
)

func normalizeProviderID(providerID string) string {
	return strings.ToLower(strings.TrimSpace(providerID))
}

func (s *Store) GetProviderCredential(ctx context.Context, providerID string) (model.ProviderCredential, bool, error) {
	resp, err := s.cli.Get(ctx, model.ProviderCredentialKey(normalizeProviderID(providerID)))
	if err != nil {
		return model.ProviderCredential{}, false, err
	}
	if len(resp.Kvs) == 0 {
		return model.ProviderCredential{}, false, nil
	}
	var cred model.ProviderCredential
	if err := json.Unmarshal(resp.Kvs[0].Value, &cred); err != nil {
		return model.ProviderCredential{}, false, err
	}
	return cred, true, nil
}

func (s *Store) PutProviderCredential(ctx context.Context, providerID string, cred model.ProviderCredential) error {
	if cred.UpdatedAt.IsZero() {
		cred.UpdatedAt = time.Now().UTC()
	}
	payload, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	_, err = s.cli.Put(ctx, model.ProviderCredentialKey(normalizeProviderID(providerID)), string(payload))
	return err
}

func (s *Store) DeleteProviderCredential(ctx context.Context, providerID string) error {
	_, err := s.cli.Delete(ctx, model.ProviderCredentialKey(normalizeProviderID(providerID)), clientv3.WithPrefix())
	return err
}
