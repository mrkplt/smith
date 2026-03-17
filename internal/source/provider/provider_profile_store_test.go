package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileProviderProfileStoreDefaultsAndCRUD(t *testing.T) {
	store := NewFileProviderProfileStore()

	profiles, err := store.ListProviderProfiles(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, profiles)
	ids := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
	}
	assert.Contains(t, ids, DefaultProviderProfileID)
	assert.Contains(t, ids, "claude-default")
	assert.Contains(t, ids, "gemini-default")

	profile := ProviderProfile{
		ID:           "openai-work",
		Name:         "OpenAI Work",
		ProviderType: "openai",
		DefaultModel: "gpt-5.4",
	}
	require.NoError(t, store.PutProviderProfile(context.Background(), profile))

	fetched, found, err := store.GetProviderProfile(context.Background(), "openai-work")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ProviderCodex, fetched.ProviderType)
	assert.Equal(t, "gpt-5.4", fetched.DefaultModel)

	err = store.DeleteProviderProfile(context.Background(), "openai-work")
	require.NoError(t, err)
	_, found, err = store.GetProviderProfile(context.Background(), "openai-work")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestFileProviderProfileStoreProtectsDefaultProfile(t *testing.T) {
	store := NewFileProviderProfileStore()
	err := store.DeleteProviderProfile(context.Background(), DefaultProviderProfileID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProtectedProviderProfile)
}

func TestNormalizeProviderProfileAppliesDefaults(t *testing.T) {
	normalized, err := NormalizeProviderProfile(ProviderProfile{
		ID:           " openai-default ",
		ProviderType: "openai",
		Capabilities: []string{"Chat", "tools", "chat"},
	})
	require.NoError(t, err)
	assert.Equal(t, "openai-default", normalized.ID)
	assert.Equal(t, "openai-default", normalized.Name)
	assert.Equal(t, ProviderCodex, normalized.ProviderType)
	assert.Equal(t, DefaultCodexModel, normalized.DefaultModel)
	assert.Equal(t, []string{"chat", "tools"}, normalized.Capabilities)
	assert.NotEmpty(t, normalized.UpdatedAt)
}

func TestNormalizeProviderProfileRejectsUnsupportedProviderType(t *testing.T) {
	_, err := NormalizeProviderProfile(ProviderProfile{
		ID:           "custom-ai",
		ProviderType: "custom",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider_type")
}
