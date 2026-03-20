package goosed

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGooseEnvOverrides(t *testing.T) {
	t.Run("openai provider uses openai key env", func(t *testing.T) {
		env := gooseEnvOverrides(map[string]string{
			"provider":       "openai",
			"model":          "gpt-5.4",
			"thinkingLevel":  "balanced",
			"providerApiKey": "sk-test",
		})

		assert.Contains(t, env, "GOOSE_PROVIDER=openai")
		assert.Contains(t, env, "GOOSE_MODEL=gpt-5.4")
		assert.Contains(t, env, "OPENAI_REASONING_EFFORT=medium")
		assert.Contains(t, env, "OPENAI_API_KEY=sk-test")
	})

	t.Run("codex provider aliases to openai for goose", func(t *testing.T) {
		env := gooseEnvOverrides(map[string]string{
			"provider":       "codex",
			"model":          "gpt-5-codex",
			"providerApiKey": "sk-codex-key",
		})

		assert.Contains(t, env, "GOOSE_PROVIDER=openai")
		assert.Contains(t, env, "GOOSE_MODEL=gpt-5-codex")
		assert.Contains(t, env, "OPENAI_API_KEY=sk-codex-key")
	})

	t.Run("provider profile fallback infers provider mapping", func(t *testing.T) {
		env := gooseEnvOverrides(map[string]string{
			"providerProfileID": "claude-default",
			"providerApiKey":    "anthropic-key",
		})

		assert.Contains(t, env, "GOOSE_PROVIDER=anthropic")
		assert.Contains(t, env, "ANTHROPIC_API_KEY=anthropic-key")
	})

	t.Run("defaults provider to openai when unset", func(t *testing.T) {
		env := gooseEnvOverrides(map[string]string{})
		assert.Contains(t, env, "GOOSE_PROVIDER=openai")
	})

	t.Run("anthropic provider maps to anthropic api key", func(t *testing.T) {
		env := gooseEnvOverrides(map[string]string{
			"provider":       "anthropic",
			"providerApiKey": "anthropic-key",
		})

		assert.Contains(t, env, "GOOSE_PROVIDER=anthropic")
		assert.Contains(t, env, "ANTHROPIC_API_KEY=anthropic-key")
		assert.NotContains(t, env, "OPENAI_API_KEY=anthropic-key")
	})

	t.Run("google provider maps to google and gemini key env vars", func(t *testing.T) {
		env := gooseEnvOverrides(map[string]string{
			"provider":       "google",
			"providerApiKey": "google-key",
		})

		assert.Contains(t, env, "GOOGLE_API_KEY=google-key")
		assert.Contains(t, env, "GEMINI_API_KEY=google-key")
	})

	t.Run("thinking level normalizes to openai reasoning effort", func(t *testing.T) {
		assert.Equal(t, "low", normalizeThinkingLevel("quick"))
		assert.Equal(t, "medium", normalizeThinkingLevel("balanced"))
		assert.Equal(t, "high", normalizeThinkingLevel("deep"))
		assert.Equal(t, "", normalizeThinkingLevel("unknown"))
	})
}
