package provider

import (
	"context"
	"fmt"
	"net/http"
)

type anthropicModelInventory struct {
	httpClient *http.Client
}

func newAnthropicModelInventory(client *http.Client) providerModelInventory {
	return &anthropicModelInventory{httpClient: client}
}

func (m *anthropicModelInventory) ListModels(ctx context.Context, req ModelInventoryRequest) ([]ModelDescriptor, error) {
	_ = ctx
	_ = req
	_ = m.httpClient
	return nil, fmt.Errorf("%w: provider_type %q model inventory is not implemented", ErrModelInventoryUnsupported, ProviderClaude)
}
