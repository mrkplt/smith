package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccountModelInventoryServiceFiltersChatModelsByDefault(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("expected /v1/models path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test-key" {
			t.Fatalf("expected bearer auth header, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "ada:ft-org-2025-01-01", "owned_by": "org", "created": 1700001000},
				{"id": "gpt-5-codex", "owned_by": "openai", "created": 1700002000},
				{"id": "gpt-4.1", "owned_by": "openai", "created": 1700000000},
			},
		})
	}))
	defer upstream.Close()

	svc := NewAccountModelInventoryServiceWithClient(upstream.Client())
	models, err := svc.ListModels(context.Background(), ModelInventoryRequest{
		Profile: ProviderProfile{
			ProviderType: ProviderCodex,
			Endpoint:     upstream.URL + "/v1",
		},
		Credential: "sk-test-key",
	})
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 chat models, got %d", len(models))
	}
	if models[0].ID != "gpt-4.1" || models[1].ID != "gpt-5-codex" {
		t.Fatalf("unexpected filtered models: %+v", models)
	}
}

func TestAccountModelInventoryServiceSupportsIncludeAll(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "ada:ft-org-2025-01-01", "owned_by": "org", "created": 1700001000},
				{"id": "gpt-5-codex", "owned_by": "openai", "created": 1700002000},
			},
		})
	}))
	defer upstream.Close()

	svc := NewAccountModelInventoryServiceWithClient(upstream.Client())
	models, err := svc.ListModels(context.Background(), ModelInventoryRequest{
		Profile: ProviderProfile{
			ProviderType: ProviderCodex,
			Endpoint:     upstream.URL + "/v1",
		},
		Credential: "sk-test-key",
		IncludeAll: true,
	})
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models for include-all, got %d", len(models))
	}
	if models[0].ID != "ada:ft-org-2025-01-01" || models[1].ID != "gpt-5-codex" {
		t.Fatalf("unexpected include-all models: %+v", models)
	}
}

func TestAccountModelInventoryServiceRequiresCredential(t *testing.T) {
	svc := NewAccountModelInventoryService()
	_, err := svc.ListModels(context.Background(), ModelInventoryRequest{
		Profile: ProviderProfile{ProviderType: ProviderCodex},
	})
	if err == nil {
		t.Fatal("expected credential-required error")
	}
	if err != ErrModelInventoryCredentialRequired {
		t.Fatalf("expected ErrModelInventoryCredentialRequired, got %v", err)
	}
}

func TestAccountModelInventoryServiceRejectsUnsupportedProvider(t *testing.T) {
	svc := NewAccountModelInventoryService()
	_, err := svc.ListModels(context.Background(), ModelInventoryRequest{
		Profile:    ProviderProfile{ProviderType: "custom"},
		Credential: "abc",
	})
	if err == nil {
		t.Fatal("expected unsupported-provider error")
	}
}
