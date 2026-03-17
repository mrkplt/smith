package provider

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type SettingsSecret struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type SecretStore interface {
	ListSecrets(ctx context.Context) ([]SettingsSecret, error)
	GetSecret(ctx context.Context, id string) (SettingsSecret, bool, error)
	PutSecret(ctx context.Context, secret SettingsSecret) error
	DeleteSecret(ctx context.Context, id string) error
}

type fileSecretStore struct {
	mu      sync.RWMutex
	secrets map[string]SettingsSecret
}

func NewFileSecretStore() SecretStore {
	return &fileSecretStore{secrets: map[string]SettingsSecret{}}
}

func (s *fileSecretStore) ListSecrets(ctx context.Context) ([]SettingsSecret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SettingsSecret, 0, len(s.secrets))
	for _, secret := range s.secrets {
		out = append(out, secret)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *fileSecretStore) GetSecret(ctx context.Context, id string) (SettingsSecret, bool, error) {
	id = strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	secret, ok := s.secrets[id]
	if !ok {
		return SettingsSecret{}, false, nil
	}
	return secret, true, nil
}

func (s *fileSecretStore) PutSecret(ctx context.Context, secret SettingsSecret) error {
	normalized, err := NormalizeSettingsSecret(secret)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[normalized.ID] = normalized
	return nil
}

func (s *fileSecretStore) DeleteSecret(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, id)
	return nil
}

type SecretSecretStore struct {
	client    kubernetes.Interface
	namespace string
	name      string
}

func NewSecretSecretStore(client kubernetes.Interface, namespace, name string) (*SecretSecretStore, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is required")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, errors.New("kubernetes namespace is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "smith-settings-secrets"
	}
	return &SecretSecretStore{client: client, namespace: namespace, name: name}, nil
}

func (s *SecretSecretStore) ListSecrets(ctx context.Context) ([]SettingsSecret, error) {
	secret, err := s.client.CoreV1().Secrets(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []SettingsSecret{}, nil
		}
		return nil, err
	}
	out := make([]SettingsSecret, 0, len(secret.Data))
	for _, payload := range secret.Data {
		var item SettingsSecret
		if err := json.Unmarshal(payload, &item); err != nil {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *SecretSecretStore) GetSecret(ctx context.Context, id string) (SettingsSecret, bool, error) {
	id = strings.TrimSpace(id)
	secret, err := s.client.CoreV1().Secrets(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return SettingsSecret{}, false, nil
		}
		return SettingsSecret{}, false, err
	}
	payload, ok := secret.Data[id]
	if !ok {
		return SettingsSecret{}, false, nil
	}
	var item SettingsSecret
	if err := json.Unmarshal(payload, &item); err != nil {
		return SettingsSecret{}, false, err
	}
	return item, true, nil
}

func (s *SecretSecretStore) PutSecret(ctx context.Context, secret SettingsSecret) error {
	normalized, err := NormalizeSettingsSecret(secret)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		secrets := s.client.CoreV1().Secrets(s.namespace)
		storeSecret, err := secrets.Get(ctx, s.name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = secrets.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace},
					Type:       corev1.SecretTypeOpaque,
					Data:       map[string][]byte{normalized.ID: payload},
				}, metav1.CreateOptions{})
				return err
			}
			return err
		}
		next := storeSecret.DeepCopy()
		if next.Data == nil {
			next.Data = map[string][]byte{}
		}
		next.Data[normalized.ID] = payload
		_, err = secrets.Update(ctx, next, metav1.UpdateOptions{})
		return err
	})
}

func (s *SecretSecretStore) DeleteSecret(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		secrets := s.client.CoreV1().Secrets(s.namespace)
		storeSecret, err := secrets.Get(ctx, s.name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if storeSecret.Data == nil {
			return nil
		}
		if _, ok := storeSecret.Data[id]; !ok {
			return nil
		}
		next := storeSecret.DeepCopy()
		delete(next.Data, id)
		_, err = secrets.Update(ctx, next, metav1.UpdateOptions{})
		return err
	})
}

func NormalizeSettingsSecret(secret SettingsSecret) (SettingsSecret, error) {
	secret.ID = strings.TrimSpace(secret.ID)
	if secret.ID == "" {
		return SettingsSecret{}, errors.New("secret id is required")
	}
	secret.Name = strings.TrimSpace(secret.Name)
	if secret.Name == "" {
		secret.Name = secret.ID
	}
	secret.Description = strings.TrimSpace(secret.Description)
	secret.Value = strings.TrimSpace(secret.Value)
	if strings.TrimSpace(secret.UpdatedAt) == "" {
		secret.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return secret, nil
}
