package provider

type CatalogEntry struct {
	ID                   string   `json:"id"`
	DisplayName          string   `json:"display_name"`
	DefaultModel         string   `json:"default_model"`
	RequiredConfigFields []string `json:"required_config_fields"`
}

func SupportedProviderCatalog() []CatalogEntry {
	return []CatalogEntry{
		{
			ID:                   ProviderCodex,
			DisplayName:          "Codex",
			DefaultModel:         DefaultCodexModel,
			RequiredConfigFields: []string{"id", "provider_type", "secret_ref"},
		},
		{
			ID:                   ProviderClaude,
			DisplayName:          "Claude",
			DefaultModel:         DefaultClaudeModel,
			RequiredConfigFields: []string{"id", "provider_type", "secret_ref"},
		},
		{
			ID:                   ProviderGemini,
			DisplayName:          "Gemini",
			DefaultModel:         DefaultGeminiModel,
			RequiredConfigFields: []string{"id", "provider_type", "secret_ref"},
		},
	}
}
