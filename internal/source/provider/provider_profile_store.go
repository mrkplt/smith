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

const (
	DefaultProviderProfileID = "codex-default"
)

var ErrProtectedProviderProfile = errors.New("provider profile is protected")

type ProviderProfile struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ProviderType string            `json:"provider_type"`
	Endpoint     string            `json:"endpoint,omitempty"`
	DefaultModel string            `json:"default_model,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	SecretRef    string            `json:"secret_ref,omitempty"`
	Options      map[string]string `json:"options,omitempty"`
	UpdatedAt    string            `json:"updated_at,omitempty"`
}

type ProviderProfileStore interface {
	ListProviderProfiles(ctx context.Context) ([]ProviderProfile, error)
	GetProviderProfile(ctx context.Context, id string) (ProviderProfile, bool, error)
	PutProviderProfile(ctx context.Context, profile ProviderProfile) error
	DeleteProviderProfile(ctx context.Context, id string) error
}

type fileProviderProfileStore struct {
	mu       sync.RWMutex
	profiles map[string]ProviderProfile
}

func NewFileProviderProfileStore() ProviderProfileStore {
	defaults := defaultProviderProfiles()
	profiles := make(map[string]ProviderProfile, len(defaults))
	for _, profile := range defaults {
		profiles[profile.ID] = profile
	}
	return &fileProviderProfileStore{profiles: profiles}
}

func (s *fileProviderProfileStore) ListProviderProfiles(ctx context.Context) ([]ProviderProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	profiles := make([]ProviderProfile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles, nil
}

func (s *fileProviderProfileStore) GetProviderProfile(ctx context.Context, id string) (ProviderProfile, bool, error) {
	id = strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	profile, ok := s.profiles[id]
	if !ok {
		return ProviderProfile{}, false, nil
	}
	return profile, true, nil
}

func (s *fileProviderProfileStore) PutProviderProfile(ctx context.Context, profile ProviderProfile) error {
	normalized, err := NormalizeProviderProfile(profile)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles[normalized.ID] = normalized
	return nil
}

func (s *fileProviderProfileStore) DeleteProviderProfile(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if isProtectedProviderProfileID(id) {
		return ErrProtectedProviderProfile
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.profiles, id)
	return nil
}

type ConfigMapProviderProfileStore struct {
	client    kubernetes.Interface
	namespace string
	name      string
}

func NewConfigMapProviderProfileStore(client kubernetes.Interface, namespace, name string) (*ConfigMapProviderProfileStore, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is required")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, errors.New("kubernetes namespace is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "smith-provider-profiles"
	}
	return &ConfigMapProviderProfileStore{client: client, namespace: namespace, name: name}, nil
}

func (s *ConfigMapProviderProfileStore) ListProviderProfiles(ctx context.Context) ([]ProviderProfile, error) {
	cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return defaultProviderProfiles(), nil
		}
		return nil, err
	}

	profiles := make([]ProviderProfile, 0, len(cm.Data))
	for _, payload := range cm.Data {
		var profile ProviderProfile
		if err := json.Unmarshal([]byte(payload), &profile); err != nil {
			continue
		}
		profiles = append(profiles, profile)
	}
	profiles = mergeWithDefaultProfiles(profiles)
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles, nil
}

func (s *ConfigMapProviderProfileStore) GetProviderProfile(ctx context.Context, id string) (ProviderProfile, bool, error) {
	id = strings.TrimSpace(id)
	cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if profile, ok := defaultProfilesByID()[id]; ok {
				return profile, true, nil
			}
			return ProviderProfile{}, false, nil
		}
		return ProviderProfile{}, false, err
	}
	payload, ok := cm.Data[id]
	if !ok {
		if profile, found := defaultProfilesByID()[id]; found {
			return profile, true, nil
		}
		return ProviderProfile{}, false, nil
	}
	var profile ProviderProfile
	if err := json.Unmarshal([]byte(payload), &profile); err != nil {
		return ProviderProfile{}, false, err
	}
	return profile, true, nil
}

func (s *ConfigMapProviderProfileStore) PutProviderProfile(ctx context.Context, profile ProviderProfile) error {
	normalized, err := NormalizeProviderProfile(profile)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				cm = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace},
					Data:       map[string]string{normalized.ID: string(payload)},
				}
				_, err = s.client.CoreV1().ConfigMaps(s.namespace).Create(ctx, cm, metav1.CreateOptions{})
				return err
			}
			return err
		}

		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data[normalized.ID] = string(payload)
		_, err = s.client.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})
}

func (s *ConfigMapProviderProfileStore) DeleteProviderProfile(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if isProtectedProviderProfileID(id) {
		return ErrProtectedProviderProfile
	}
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if cm.Data == nil {
			return nil
		}
		if _, ok := cm.Data[id]; !ok {
			return nil
		}
		delete(cm.Data, id)
		_, err = s.client.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})
}

func NormalizeProviderProfile(profile ProviderProfile) (ProviderProfile, error) {
	profile.ID = strings.TrimSpace(profile.ID)
	if profile.ID == "" {
		return ProviderProfile{}, errors.New("provider profile id is required")
	}
	profile.Name = strings.TrimSpace(profile.Name)
	if profile.Name == "" {
		profile.Name = profile.ID
	}
	rawProviderType := strings.ToLower(strings.TrimSpace(profile.ProviderType))
	if rawProviderType == "" {
		rawProviderType = ProviderCodex
	}
	normalizedProviderType, ok := canonicalProviderType(rawProviderType)
	if !ok {
		return ProviderProfile{}, errors.New("unsupported provider_type; supported values: codex, claude, gemini")
	}
	profile.ProviderType = normalizedProviderType
	profile.Endpoint = strings.TrimSpace(profile.Endpoint)
	profile.SecretRef = strings.TrimSpace(profile.SecretRef)
	profile.DefaultModel = strings.TrimSpace(profile.DefaultModel)
	if profile.DefaultModel == "" {
		profile.DefaultModel = defaultModelForProviderType(profile.ProviderType)
	}
	profile.Capabilities = normalizeCapabilities(profile.Capabilities)
	if len(profile.Capabilities) == 0 {
		profile.Capabilities = defaultCapabilities(profile.ProviderType)
	}
	if profile.Options == nil {
		profile.Options = map[string]string{}
	}
	if strings.TrimSpace(profile.UpdatedAt) == "" {
		profile.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return profile, nil
}

func defaultProviderProfiles() []ProviderProfile {
	codexProfile, _ := NormalizeProviderProfile(ProviderProfile{
		ID:           DefaultProviderProfileID,
		Name:         "Codex Default",
		ProviderType: ProviderCodex,
		DefaultModel: DefaultCodexModel,
		Capabilities: []string{"chat", "tools", "loops"},
	})
	claudeProfile, _ := NormalizeProviderProfile(ProviderProfile{
		ID:           "claude-default",
		Name:         "Claude Default",
		ProviderType: ProviderClaude,
		DefaultModel: DefaultClaudeModel,
		Capabilities: []string{"chat", "tools", "loops"},
	})
	geminiProfile, _ := NormalizeProviderProfile(ProviderProfile{
		ID:           "gemini-default",
		Name:         "Gemini Default",
		ProviderType: ProviderGemini,
		DefaultModel: DefaultGeminiModel,
		Capabilities: []string{"chat", "tools", "loops"},
	})
	return []ProviderProfile{codexProfile, claudeProfile, geminiProfile}
}

func defaultProfilesByID() map[string]ProviderProfile {
	profiles := defaultProviderProfiles()
	byID := make(map[string]ProviderProfile, len(profiles))
	for _, profile := range profiles {
		byID[profile.ID] = profile
	}
	return byID
}

func mergeWithDefaultProfiles(in []ProviderProfile) []ProviderProfile {
	byID := defaultProfilesByID()
	for _, profile := range in {
		if strings.TrimSpace(profile.ID) == "" {
			continue
		}
		byID[profile.ID] = profile
	}
	out := make([]ProviderProfile, 0, len(byID))
	for _, profile := range byID {
		out = append(out, profile)
	}
	return out
}

func defaultModelForProviderType(providerType string) string {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case ProviderCodex:
		return DefaultCodexModel
	case ProviderClaude:
		return DefaultClaudeModel
	case ProviderGemini:
		return DefaultGeminiModel
	default:
		return DefaultCodexModel
	}
}

func defaultCapabilities(providerType string) []string {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case ProviderCodex, ProviderClaude, ProviderGemini:
		return []string{"chat", "tools", "loops"}
	default:
		return []string{"chat"}
	}
}

func canonicalProviderType(raw string) (string, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case ProviderCodex, "openai":
		return ProviderCodex, true
	case ProviderClaude, "anthropic":
		return ProviderClaude, true
	case ProviderGemini, "google":
		return ProviderGemini, true
	default:
		return "", false
	}
}

func isProtectedProviderProfileID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	_, ok := defaultProfilesByID()[id]
	return ok
}

func normalizeCapabilities(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, capability := range in {
		trimmed := strings.ToLower(strings.TrimSpace(capability))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
