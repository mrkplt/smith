package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"smith/internal/chat"
	api "smith/pkg/api/v1"
	"strings"
	"sync"
	"time"
)

type SessionManager interface {
	CreateSession(sType chat.SessionType, ctx map[string]string) (*chat.Session, error)
	GetSession(id string) (*chat.Session, bool)
	AddMessage(sessionID string, msg chat.Message) error
}

type PromptBuilder interface {
	BuildSystemPrompt(ctx context.Context, session *chat.Session) (string, error)
}

type StatusError interface {
	error
	StatusCode() int
}

type HTTPError struct {
	Code    int
	Message string
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) == "" {
		return http.StatusText(e.Code)
	}
	return e.Message
}

func (e *HTTPError) StatusCode() int {
	if e == nil || e.Code < 100 {
		return http.StatusInternalServerError
	}
	return e.Code
}

type Server struct {
	engine   chat.Engine
	sessions SessionManager
	prompts  PromptBuilder
	commit   func(*http.Request, api.ChatCommitActionRequest) (api.ChatCommitActionResponse, error)

	pendingMu sync.Mutex
	pending   map[string]string

	mux *http.ServeMux
}

func NewServer(engine chat.Engine, sessions SessionManager, prompts PromptBuilder) *Server {
	return NewServerWithCommit(engine, sessions, prompts, nil)
}

func NewServerWithCommit(
	engine chat.Engine,
	sessions SessionManager,
	prompts PromptBuilder,
	commit func(*http.Request, api.ChatCommitActionRequest) (api.ChatCommitActionResponse, error),
) *Server {
	s := &Server{
		engine:   engine,
		sessions: sessions,
		prompts:  prompts,
		commit:   commit,
		pending:  make(map[string]string),
		mux:      http.NewServeMux(),
	}

	s.mux.HandleFunc("POST /v1/chat/sessions", s.handleCreateSession)
	s.mux.HandleFunc("POST /v1/chat/sessions/{sessionID}/messages", s.handlePostMessage)
	s.mux.HandleFunc("GET /v1/chat/sessions/{sessionID}/stream", s.handleStream)
	s.mux.HandleFunc("POST /v1/chat/actions/commit", s.handleCommitAction)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req api.ChatCreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	sType, ok := parseSessionType(req.Type)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid session type"})
		return
	}

	if req.Context == nil {
		req.Context = map[string]string{}
	}

	session, err := s.sessions.CreateSession(sType, req.Context)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing session ID"})
		return
	}

	if _, ok := s.sessions.GetSession(sessionID); !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}

	var req api.ChatPostMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	trimmed := strings.TrimSpace(req.Message)
	if trimmed == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "message is required"})
		return
	}

	s.pendingMu.Lock()
	if _, exists := s.pending[sessionID]; exists {
		s.pendingMu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "previous message is still pending stream consumption"})
		return
	}
	s.pending[sessionID] = trimmed
	s.pendingMu.Unlock()

	if err := s.sessions.AddMessage(sessionID, chat.Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Role:      chat.RoleUser,
		Content:   trimmed,
		Timestamp: time.Now(),
	}); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, sessionID)
		s.pendingMu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}

	writeJSON(w, http.StatusAccepted, api.ChatPostMessageResponse{Status: "queued"})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing session ID"})
		return
	}

	session, ok := s.sessions.GetSession(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}

	message, ok := s.popPendingMessage(sessionID)
	if !ok {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "no queued message for this session"})
		return
	}

	promptMessage, err := s.buildPrompt(r.Context(), session, message)
	if err != nil {
		s.restorePendingMessage(sessionID, message)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.restorePendingMessage(sessionID, message)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "streaming is not supported by this response writer"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	events := make(chan chat.ChatEvent, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.engine.Stream(r.Context(), session, promptMessage, events)
		close(events)
	}()

	var assistantReply strings.Builder
	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-events:
			if !ok {
				if err := <-errCh; err != nil {
					_ = writeSSE(w, string(chat.EventError), map[string]any{"message": err.Error()})
					flusher.Flush()
				}
				if strings.TrimSpace(assistantReply.String()) != "" {
					_ = s.sessions.AddMessage(sessionID, chat.Message{
						ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
						Role:      chat.RoleAssistant,
						Content:   assistantReply.String(),
						Timestamp: time.Now(),
					})
				}
				return
			}

			if evt.Event == chat.EventMessageDelta {
				if delta, ok := evt.Data.(chat.MessageDelta); ok {
					assistantReply.WriteString(delta.Delta)
				}
			}

			if err := writeSSE(w, string(evt.Event), evt.Data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleCommitAction(w http.ResponseWriter, r *http.Request) {
	var req api.ChatCommitActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	if s.commit != nil {
		res, err := s.commit(r, req)
		if err != nil {
			status := http.StatusBadRequest
			var statusErr StatusError
			if errors.As(err, &statusErr) {
				status = statusErr.StatusCode()
			}
			writeJSON(w, status, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, res)
		return
	}

	writeJSON(w, http.StatusOK, api.ChatCommitActionResponse{Status: "accepted"})
}

func (s *Server) popPendingMessage(sessionID string) (string, bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	msg, ok := s.pending[sessionID]
	if !ok {
		return "", false
	}
	delete(s.pending, sessionID)
	return msg, true
}

func (s *Server) restorePendingMessage(sessionID, message string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if _, exists := s.pending[sessionID]; !exists {
		s.pending[sessionID] = message
	}
}

func (s *Server) buildPrompt(ctx context.Context, session *chat.Session, message string) (string, error) {
	if s.prompts == nil {
		return message, nil
	}

	systemPrompt, err := s.prompts.BuildSystemPrompt(ctx, session)
	if err != nil {
		return "", fmt.Errorf("build system prompt: %w", err)
	}

	if strings.TrimSpace(systemPrompt) == "" {
		return message, nil
	}

	return systemPrompt + "\n\nUser message:\n" + message, nil
}

func parseSessionType(raw string) (chat.SessionType, bool) {
	switch chat.SessionType(raw) {
	case chat.SessionTypePRDRefinement, chat.SessionTypeLoopAssist, chat.SessionTypeDocumentAssist, chat.SessionTypeRuntimeAssist:
		return chat.SessionType(raw), true
	default:
		return "", false
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeSSE(w http.ResponseWriter, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
		return err
	}
	return nil
}
