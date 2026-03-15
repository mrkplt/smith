package v1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "smith/pkg/api/v1"
)

func TestClient_CreateLoop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/loops" {
			t.Errorf("expected /v1/loops, got %s", r.URL.Path)
		}

		res := api.LoopCreateResult{
			LoopID:  "loop-123",
			Status:  "unresolved",
			Created: true,
		}
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	req := api.LoopCreateRequest{
		Title: "Test Loop",
	}

	res, err := c.CreateLoop(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateLoop failed: %v", err)
	}

	if res.LoopID != "loop-123" {
		t.Errorf("expected loop-123, got %s", res.LoopID)
	}
}

func TestClient_GetLoop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := api.LoopResponse{
			State: api.State{
				LoopID: "loop-123",
				State:  "running",
			},
		}
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	res, err := c.GetLoop(context.Background(), "loop-123")
	if err != nil {
		t.Fatalf("GetLoop failed: %v", err)
	}

	if res.State.LoopID != "loop-123" {
		t.Errorf("expected loop-123, got %s", res.State.LoopID)
	}
}

func TestClient_CreateChatSession(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/sessions" {
			t.Errorf("expected /v1/chat/sessions, got %s", r.URL.Path)
		}

		var req api.ChatCreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Type != "prd-refinement" {
			t.Fatalf("expected type prd-refinement, got %s", req.Type)
		}

		_ = json.NewEncoder(w).Encode(api.ChatSession{ID: "sess_1", Type: req.Type, Context: req.Context})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	res, err := c.CreateChatSession(context.Background(), api.ChatCreateSessionRequest{
		Type:    "prd-refinement",
		Context: map[string]string{"documentId": "doc-1"},
	})
	if err != nil {
		t.Fatalf("CreateChatSession failed: %v", err)
	}

	if res.ID != "sess_1" {
		t.Fatalf("expected sess_1, got %s", res.ID)
	}
}

func TestClient_PostChatMessageAndCommit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/sessions/sess_1/messages":
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(api.ChatPostMessageResponse{Status: "queued"})
		case "/v1/chat/actions/commit":
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(api.ChatCommitActionResponse{Status: "accepted"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "")
	postRes, err := c.PostChatMessage(context.Background(), "sess_1", api.ChatPostMessageRequest{Message: "hello"})
	if err != nil {
		t.Fatalf("PostChatMessage failed: %v", err)
	}
	if postRes.Status != "queued" {
		t.Fatalf("expected queued status, got %s", postRes.Status)
	}

	commitRes, err := c.CommitChatAction(context.Background(), api.ChatCommitActionRequest{Action: "save", Payload: map[string]any{"k": "v"}})
	if err != nil {
		t.Fatalf("CommitChatAction failed: %v", err)
	}
	if commitRes.Status != "accepted" {
		t.Fatalf("expected accepted status, got %s", commitRes.Status)
	}
}

func TestClient_OpenChatStream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/sessions/sess_1/stream" {
			t.Errorf("expected stream path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("expected bearer token header, got %q", got)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message.delta\n"))
		_, _ = w.Write([]byte("data: {\"delta\":\"hello\"}\n\n"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	resp, err := c.OpenChatStream(context.Background(), "sess_1")
	if err != nil {
		t.Fatalf("OpenChatStream failed: %v", err)
	}
	defer resp.Body.Close()

	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", resp.Header.Get("Content-Type"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream body: %v", err)
	}
	if !strings.Contains(string(body), "event: message.delta") {
		t.Fatalf("expected stream payload, got %s", string(body))
	}
}
