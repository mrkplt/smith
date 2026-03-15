package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"smith/internal/chat"
	"smith/internal/chat/sessions"
	api "smith/pkg/api/v1"
	"strings"
	"sync"
	"testing"
)

type stubEngine struct {
	mu          sync.Mutex
	lastMessage string
	streamFn    func(ctx context.Context, session *chat.Session, message string, events chan<- chat.ChatEvent) error
}

func (s *stubEngine) Stream(ctx context.Context, session *chat.Session, message string, events chan<- chat.ChatEvent) error {
	s.mu.Lock()
	s.lastMessage = message
	s.mu.Unlock()

	if s.streamFn != nil {
		return s.streamFn(ctx, session, message, events)
	}

	events <- chat.ChatEvent{Event: chat.EventMessageDelta, Data: chat.MessageDelta{Delta: "hello"}}
	events <- chat.ChatEvent{Event: chat.EventMessageCompleted, Data: map[string]any{}}
	return nil
}

func (s *stubEngine) LastMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastMessage
}

type stubPromptBuilder struct {
	prompt string
	err    error
}

func (s stubPromptBuilder) BuildSystemPrompt(_ context.Context, _ *chat.Session) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.prompt, nil
}

func TestCreateSession(t *testing.T) {
	engine := &stubEngine{}
	sessionManager := sessions.NewManager()
	server := NewServer(engine, sessionManager, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/sessions", strings.NewReader(`{"type":"prd-refinement","context":{"documentId":"doc-1"}}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var session chat.Session
	if err := json.NewDecoder(rec.Body).Decode(&session); err != nil {
		t.Fatalf("failed to decode create session response: %v", err)
	}

	if session.ID == "" {
		t.Fatalf("expected non-empty session ID")
	}
	if session.Type != chat.SessionTypePRDRefinement {
		t.Fatalf("expected session type %q, got %q", chat.SessionTypePRDRefinement, session.Type)
	}
}

func TestPostAndStreamMessage(t *testing.T) {
	engine := &stubEngine{}
	sessionManager := sessions.NewManager()
	server := NewServer(engine, sessionManager, stubPromptBuilder{prompt: "System prompt"})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/chat/sessions", strings.NewReader(`{"type":"loop-assist","context":{}}`))
	createRec := httptest.NewRecorder()
	server.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create session failed: status %d body %s", createRec.Code, createRec.Body.String())
	}

	var session chat.Session
	if err := json.NewDecoder(createRec.Body).Decode(&session); err != nil {
		t.Fatalf("decode create session response: %v", err)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/v1/chat/sessions/"+session.ID+"/messages", strings.NewReader(`{"message":"Need a status update"}`))
	postRec := httptest.NewRecorder()
	server.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusAccepted {
		t.Fatalf("post message failed: status %d body %s", postRec.Code, postRec.Body.String())
	}

	streamReq := httptest.NewRequest(http.MethodGet, "/v1/chat/sessions/"+session.ID+"/stream", nil)
	streamRec := httptest.NewRecorder()
	server.ServeHTTP(streamRec, streamReq)

	if streamRec.Code != http.StatusOK {
		t.Fatalf("stream failed: status %d body %s", streamRec.Code, streamRec.Body.String())
	}
	if got := streamRec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", got)
	}

	body := streamRec.Body.String()
	if !strings.Contains(body, "event: message.delta") {
		t.Fatalf("expected message.delta event in stream body: %s", body)
	}
	if !strings.Contains(body, "event: message.completed") {
		t.Fatalf("expected message.completed event in stream body: %s", body)
	}

	lastPrompt := engine.LastMessage()
	if !strings.Contains(lastPrompt, "System prompt") || !strings.Contains(lastPrompt, "Need a status update") {
		t.Fatalf("engine prompt did not include expected context and message: %q", lastPrompt)
	}

	updated, ok := sessionManager.GetSession(session.ID)
	if !ok {
		t.Fatalf("session not found after stream")
	}
	if len(updated.Messages) != 2 {
		t.Fatalf("expected 2 messages in session history, got %d", len(updated.Messages))
	}
	if updated.Messages[0].Role != chat.RoleUser {
		t.Fatalf("expected first message role user, got %s", updated.Messages[0].Role)
	}
	if updated.Messages[1].Role != chat.RoleAssistant {
		t.Fatalf("expected second message role assistant, got %s", updated.Messages[1].Role)
	}
}

func TestPostMessageConflictsWhenPreviousPending(t *testing.T) {
	engine := &stubEngine{}
	sessionManager := sessions.NewManager()
	server := NewServer(engine, sessionManager, nil)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/chat/sessions", strings.NewReader(`{"type":"runtime-assist","context":{}}`))
	createRec := httptest.NewRecorder()
	server.ServeHTTP(createRec, createReq)

	var session chat.Session
	if err := json.NewDecoder(createRec.Body).Decode(&session); err != nil {
		t.Fatalf("decode create session response: %v", err)
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/chat/sessions/"+session.ID+"/messages", strings.NewReader(`{"message":"first"}`))
	firstRec := httptest.NewRecorder()
	server.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusAccepted {
		t.Fatalf("first post failed: status %d body %s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/chat/sessions/"+session.ID+"/messages", strings.NewReader(`{"message":"second"}`))
	secondRec := httptest.NewRecorder()
	server.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("expected conflict status, got %d body %s", secondRec.Code, secondRec.Body.String())
	}
}

func TestCommitActionReturnsStatusFromCommitError(t *testing.T) {
	engine := &stubEngine{}
	sessionManager := sessions.NewManager()
	server := NewServerWithCommit(engine, sessionManager, nil, func(_ *http.Request, _ api.ChatCommitActionRequest) (api.ChatCommitActionResponse, error) {
		return api.ChatCommitActionResponse{}, &HTTPError{Code: http.StatusBadGateway, Message: "upstream unavailable"}
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/actions/commit", strings.NewReader(`{"action":"create-loop","payload":{}}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d body %s", rec.Code, rec.Body.String())
	}
}
