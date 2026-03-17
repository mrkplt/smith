package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSettingsSecretRequiresID(t *testing.T) {
	_, err := NormalizeSettingsSecret(SettingsSecret{Name: "missing-id"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret id")
}

func TestFileSecretStoreCRUD(t *testing.T) {
	store := NewFileSecretStore()
	ctx := context.Background()

	err := store.PutSecret(ctx, SettingsSecret{
		ID:          "openai-key",
		Name:        "OpenAI key",
		Description: "Primary OpenAI credential",
		Value:       "sk-test-123456",
	})
	require.NoError(t, err)

	item, found, err := store.GetSecret(ctx, "openai-key")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "OpenAI key", item.Name)
	assert.Equal(t, "Primary OpenAI credential", item.Description)
	assert.Equal(t, "sk-test-123456", item.Value)
	assert.NotEmpty(t, item.UpdatedAt)

	all, err := store.ListSecrets(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "openai-key", all[0].ID)

	err = store.DeleteSecret(ctx, "openai-key")
	require.NoError(t, err)

	_, found, err = store.GetSecret(ctx, "openai-key")
	require.NoError(t, err)
	assert.False(t, found)
}
