package smithbridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIDocumentFetcher_GetDocument(t *testing.T) {
	t.Run("fetches document from api", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/documents/doc-1", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"doc-1","project_id":"proj-1","title":"Doc","content":"hello","format":"markdown"}`))
		}))
		defer ts.Close()

		fetcher := NewAPIDocumentFetcher(ts.URL, ts.Client())
		doc, err := fetcher.GetDocument(context.Background(), "doc-1")
		require.NoError(t, err)
		require.NotNil(t, doc)
		require.Equal(t, "doc-1", doc.ID)
		require.Equal(t, "hello", doc.Content)
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		fetcher := NewAPIDocumentFetcher(ts.URL, ts.Client())
		doc, err := fetcher.GetDocument(context.Background(), "missing")
		require.Error(t, err)
		require.Nil(t, doc)
	})
}
