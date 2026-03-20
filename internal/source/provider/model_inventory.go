package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const defaultModelInventoryTimeout = 10 * time.Second

var (
	ErrModelInventoryCredentialRequired = errors.New("provider credential is required for model inventory")
	ErrModelInventoryUnsupported        = errors.New("provider model inventory unsupported")
)

type ModelDescriptor struct {
	ID      string
	OwnedBy string
	Created int64
}

type ModelInventoryRequest struct {
	Profile    ProviderProfile
	Credential string
	IncludeAll bool
}

type ModelInventoryService interface {
	ListModels(ctx context.Context, req ModelInventoryRequest) ([]ModelDescriptor, error)
}

type providerModelInventory interface {
	ListModels(ctx context.Context, req ModelInventoryRequest) ([]ModelDescriptor, error)
}

type accountModelInventoryService struct {
	fetchers map[string]providerModelInventory
}

func NewAccountModelInventoryService() ModelInventoryService {
	client := &http.Client{Timeout: defaultModelInventoryTimeout}
	return NewAccountModelInventoryServiceWithClient(client)
}

func NewAccountModelInventoryServiceWithClient(client *http.Client) ModelInventoryService {
	if client == nil {
		client = &http.Client{Timeout: defaultModelInventoryTimeout}
	}
	return &accountModelInventoryService{
		fetchers: map[string]providerModelInventory{
			ProviderCodex:  newOpenAIModelInventory(client),
			ProviderClaude: newAnthropicModelInventory(client),
			ProviderGemini: newGeminiModelInventory(client),
		},
	}
}

func (s *accountModelInventoryService) ListModels(ctx context.Context, req ModelInventoryRequest) ([]ModelDescriptor, error) {
	if strings.TrimSpace(req.Credential) == "" {
		return nil, ErrModelInventoryCredentialRequired
	}

	providerType := strings.ToLower(strings.TrimSpace(req.Profile.ProviderType))
	fetcher, ok := s.fetchers[providerType]
	if !ok {
		return nil, fmt.Errorf("%w: provider_type %q", ErrModelInventoryUnsupported, req.Profile.ProviderType)
	}

	models, err := fetcher.ListModels(ctx, req)
	if err != nil {
		return nil, err
	}

	if !req.IncludeAll {
		models = filterLikelyChatModels(models)
	}

	seen := map[string]struct{}{}
	unique := make([]ModelDescriptor, 0, len(models))
	for _, item := range models {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		item.ID = id
		unique = append(unique, item)
	}

	sort.Slice(unique, func(i, j int) bool {
		return unique[i].ID < unique[j].ID
	})

	return unique, nil
}

func filterLikelyChatModels(in []ModelDescriptor) []ModelDescriptor {
	if len(in) == 0 {
		return nil
	}
	out := make([]ModelDescriptor, 0, len(in))
	for _, item := range in {
		if !isLikelyChatModelID(item.ID) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func isLikelyChatModelID(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return false
	}
	if strings.Contains(id, ":") {
		return false
	}
	return strings.HasPrefix(id, "gpt-") ||
		strings.HasPrefix(id, "chatgpt-") ||
		strings.HasPrefix(id, "o1") ||
		strings.HasPrefix(id, "o3") ||
		strings.HasPrefix(id, "o4")
}
