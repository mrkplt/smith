package model

import "time"

// ProviderCredential holds OAuth credential files for a provider scoped by provider ID.
// Currently used to supply Claude Max OAuth credentials to replica pods at runtime.
type ProviderCredential struct {
	CredentialsJSON string    `json:"credentials_json"`
	ClaudeJSON      string    `json:"claude_json"`
	SettingsJSON    string    `json:"settings_json"`
	UpdatedAt       time.Time `json:"updated_at"`
}
