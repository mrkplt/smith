package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type openAIModelInventory struct {
	httpClient *http.Client
}

func newOpenAIModelInventory(client *http.Client) providerModelInventory {
	return &openAIModelInventory{httpClient: client}
}

func (m *openAIModelInventory) ListModels(ctx context.Context, req ModelInventoryRequest) ([]ModelDescriptor, error) {
	endpoint := strings.TrimSpace(req.Profile.Endpoint)
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}

	modelsURL := strings.TrimRight(endpoint, "/") + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("provider model inventory request setup failed")
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(req.Credential))
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "smith-api")

	httpRes, err := m.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("provider model inventory request failed: %v", err)
	}
	defer httpRes.Body.Close()

	if httpRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(httpRes.Body, 4*1024))
		message := strings.TrimSpace(string(body))
		if message == "" {
			return nil, fmt.Errorf("provider model inventory request failed with status %d", httpRes.StatusCode)
		}
		return nil, fmt.Errorf("provider model inventory request failed with status %d: %s", httpRes.StatusCode, message)
	}

	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	if err := json.NewDecoder(httpRes.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("provider model inventory decode failed")
	}

	models := make([]ModelDescriptor, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		models = append(models, ModelDescriptor{
			ID:      id,
			OwnedBy: strings.TrimSpace(item.OwnedBy),
			Created: item.Created,
		})
	}

	return models, nil
}
