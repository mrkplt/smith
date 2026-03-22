package model

import "testing"

func TestNormalizeProviderModelDefaultsToCodex(t *testing.T) {
	providerID, model, err := NormalizeProviderModel("", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if providerID != DefaultProviderID {
		t.Fatalf("expected provider %q, got %q", DefaultProviderID, providerID)
	}
	if model != DefaultModel {
		t.Fatalf("expected model %q, got %q", DefaultModel, model)
	}
}

func TestNormalizeProviderModelRejectsUnknownProvider(t *testing.T) {
	_, _, err := NormalizeProviderModel("anthropic", "claude-sonnet")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNormalizeProviderModelAcceptsCustomModel(t *testing.T) {
	providerID, model, err := NormalizeProviderModel("codex", "gpt-5.3-codex")
	if err != nil {
		t.Fatalf("expected custom model to normalize, got %v", err)
	}
	if providerID != DefaultProviderID {
		t.Fatalf("expected provider %q, got %q", DefaultProviderID, providerID)
	}
	if model != "gpt-5.3-codex" {
		t.Fatalf("expected model %q, got %q", "gpt-5.3-codex", model)
	}
}

func TestAnomalyNormalizeProviderSelection(t *testing.T) {
	a := Anomaly{}
	if err := a.NormalizeProviderSelection(); err != nil {
		t.Fatalf("expected normalization to succeed: %v", err)
	}
	if a.ProviderID != DefaultProviderID {
		t.Fatalf("expected provider %q, got %q", DefaultProviderID, a.ProviderID)
	}
	if a.Model != DefaultModel {
		t.Fatalf("expected model %q, got %q", DefaultModel, a.Model)
	}
}
