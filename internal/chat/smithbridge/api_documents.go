package smithbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"smith/internal/source/model"
)

type APIDocumentFetcher struct {
	baseURL string
	client  *http.Client
}

func NewAPIDocumentFetcher(apiURL string, client *http.Client) *APIDocumentFetcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &APIDocumentFetcher{
		baseURL: strings.TrimRight(strings.TrimSpace(apiURL), "/"),
		client:  client,
	}
}

func (f *APIDocumentFetcher) GetDocument(ctx context.Context, docID string) (*model.Document, error) {
	if strings.TrimSpace(f.baseURL) == "" {
		return nil, fmt.Errorf("api document fetcher base url is required")
	}
	if strings.TrimSpace(docID) == "" {
		return nil, fmt.Errorf("document id is required")
	}
	requestURL := fmt.Sprintf("%s/v1/documents/%s", f.baseURL, strings.TrimSpace(docID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("document lookup failed with status %d", resp.StatusCode)
	}
	var doc model.Document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode document response: %w", err)
	}
	return &doc, nil
}
