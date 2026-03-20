package docstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"smith/internal/source/model"
	"smith/internal/source/store"
)

func TestMigratingStore_ReadThroughOnMiss(t *testing.T) {
	ctx := context.Background()
	primary := store.NewMemStore()
	fallback := store.NewMemStore()

	seed := model.Document{
		ID:        "doc-1",
		ProjectID: "proj-1",
		Title:     "Doc One",
		Content:   "from fallback",
		Format:    "markdown",
		CreatedAt: time.Now().UTC().Add(-time.Minute),
		UpdatedAt: time.Now().UTC().Add(-time.Minute),
	}
	require.NoError(t, fallback.PutDocument(ctx, seed))

	migrating := NewMigratingStore(NewEtcdStore(primary), NewEtcdStore(fallback), MigrationOptions{
		ReadThroughOnMiss: true,
	})

	doc, found, err := migrating.GetDocument(ctx, seed.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, seed.Content, doc.Content)

	replayed, replayedFound, replayErr := primary.GetDocument(ctx, seed.ID)
	require.NoError(t, replayErr)
	require.True(t, replayedFound)
	require.Equal(t, seed.Title, replayed.Title)
}

func TestBackfillDocuments(t *testing.T) {
	ctx := context.Background()
	target := store.NewMemStore()
	source := store.NewMemStore()

	older := model.Document{
		ID:        "doc-older",
		ProjectID: "proj-1",
		Title:     "Older",
		Content:   "old",
		Format:    "markdown",
		CreatedAt: time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt: time.Now().UTC().Add(-10 * time.Minute),
	}
	newer := model.Document{
		ID:        "doc-newer",
		ProjectID: "proj-1",
		Title:     "Newer",
		Content:   "new",
		Format:    "markdown",
		CreatedAt: time.Now().UTC().Add(-5 * time.Minute),
		UpdatedAt: time.Now().UTC().Add(-5 * time.Minute),
	}
	require.NoError(t, source.PutDocument(ctx, older))
	require.NoError(t, source.PutDocument(ctx, newer))

	summary, err := BackfillDocuments(ctx, NewEtcdStore(target), NewEtcdStore(source))
	require.NoError(t, err)
	require.Equal(t, 2, summary.Scanned)
	require.Equal(t, 2, summary.Upserted)

	listed, listErr := target.ListDocuments(ctx)
	require.NoError(t, listErr)
	require.Len(t, listed, 2)
}
