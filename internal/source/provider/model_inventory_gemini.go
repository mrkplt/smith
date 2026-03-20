package provider

import (
	"context"
	"fmt"
	"net/http"
)

type geminiModelInventory struct {
	httpClient *http.Client
}

func newGeminiModelInventory(client *http.Client) providerModelInventory {
	return &geminiModelInventory{httpClient: client}
}

func (m *geminiModelInventory) ListModels(ctx context.Context, req ModelInventoryRequest) ([]ModelDescriptor, error) {
	_ = ctx
	_ = req
	_ = m.httpClient
	return nil, fmt.Errorf("%w: provider_type %q model inventory is not implemented", ErrModelInventoryUnsupported, ProviderGemini)
}
