package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"smith/internal/chat"
	api "smith/pkg/api/v1"
	"strconv"
	"strings"
	"sync"
	"time"
)

var documentPatchJSONFenceRE = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

const (
	defaultFirstResponseTimeout = 45 * time.Second
	defaultHeartbeatInterval    = 10 * time.Second
)

type SessionManager interface {
	CreateSession(sType chat.SessionType, ctx map[string]string) (*chat.Session, error)
	GetSession(id string) (*chat.Session, bool)
	AddMessage(sessionID string, msg chat.Message) error
	UpdateContext(sessionID string, updates map[string]string) (*chat.Session, error)
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
	s.mux.HandleFunc("POST /v1/chat/sessions/{sessionID}/context", s.handleUpdateContext)
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

func (s *Server) handleUpdateContext(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing session ID"})
		return
	}

	if _, ok := s.sessions.GetSession(sessionID); !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}

	var req api.ChatUpdateContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	eventType := strings.TrimSpace(req.Type)
	if eventType != "" && eventType != "ui.context.updated" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported context update type"})
		return
	}

	updates := normalizeContextUpdates(req.Context)
	for key, value := range flattenFocusContext(req.FocusContext) {
		updates[key] = value
	}
	if eventType != "" {
		updates["lastContextEventType"] = eventType
	}

	session, err := s.sessions.UpdateContext(sessionID, updates)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "session not found"})
		return
	}

	writeJSON(w, http.StatusOK, api.ChatUpdateContextResponse{
		Status:  "updated",
		Context: session.Context,
	})
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

	heartbeat := time.NewTicker(heartbeatIntervalFromContext(session.Context))
	defer heartbeat.Stop()
	firstResponseTimer := time.NewTimer(firstResponseTimeoutFromContext(session.Context))
	defer stopTimer(firstResponseTimer)
	firstResponseObserved := false

	_ = writeSSE(w, "session.started", map[string]any{
		"sessionId":   session.ID,
		"sessionType": session.Type,
	})
	_ = writeSSE(w, "context.loaded", buildContextLoadedPayload(session))
	if readiness, ok := readinessPayloadFromContext(session.Context); ok {
		_ = writeSSE(w, "readiness.updated", readiness)
	}
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
		case <-heartbeat.C:
			if err := writeSSE(w, "stream.keepalive", map[string]any{
				"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			}); err != nil {
				return
			}
			flusher.Flush()
		case <-firstResponseTimer.C:
			_ = writeSSE(w, string(chat.EventError), map[string]any{
				"message": "Assistant did not produce a response in time. Verify provider profile/auth settings and retry.",
			})
			flusher.Flush()
			return
		case <-r.Context().Done():
			return
		case evt, ok := <-events:
			if !ok {
				if err := <-errCh; err != nil {
					_ = writeSSE(w, string(chat.EventError), map[string]any{"message": err.Error()})
					flusher.Flush()
				}
				if proposal, found := extractDocumentPatchProposal(assistantReply.String()); found {
					_ = writeSSE(w, "document.patch.proposed", proposal)
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
			if !firstResponseObserved && (evt.Event == chat.EventMessageDelta || evt.Event == chat.EventMessageCompleted || evt.Event == chat.EventError) {
				firstResponseObserved = true
				stopTimer(firstResponseTimer)
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

func buildContextLoadedPayload(session *chat.Session) map[string]any {
	if session == nil {
		return map[string]any{}
	}

	payload := map[string]any{
		"sessionId":   session.ID,
		"sessionType": session.Type,
	}

	if len(session.Context) == 0 {
		return payload
	}

	for _, key := range []string{"documentId", "documentVersion", "documentTitle", "documentFormat", "documentStatus", "sessionIntent"} {
		if value := strings.TrimSpace(session.Context[key]); value != "" {
			payload[key] = value
		}
	}
	if focus := buildFocusPayloadFromContext(session.Context); len(focus) > 0 {
		payload["focus"] = focus
	}

	workspace := map[string]any{}
	for _, key := range []string{"app", "surface", "route", "projectId"} {
		if value := strings.TrimSpace(session.Context[key]); value != "" {
			workspace[key] = value
		}
	}
	if len(workspace) > 0 {
		payload["workspace"] = workspace
	}

	return payload
}

func buildFocusPayloadFromContext(sCtx map[string]string) map[string]any {
	focus := map[string]any{}
	for _, pair := range []struct {
		contextKey string
		payloadKey string
	}{
		{contextKey: "focusSurface", payloadKey: "surface"},
		{contextKey: "focusEntityType", payloadKey: "entityType"},
		{contextKey: "focusEntitySubtype", payloadKey: "entitySubtype"},
		{contextKey: "focusEntityId", payloadKey: "entityId"},
		{contextKey: "focusEntityVersion", payloadKey: "entityVersion"},
		{contextKey: "focusSectionId", payloadKey: "sectionId"},
		{contextKey: "focusSelectionText", payloadKey: "selectionText"},
		{contextKey: "focusLineIndex", payloadKey: "lineIndex"},
	} {
		if value := strings.TrimSpace(sCtx[pair.contextKey]); value != "" {
			focus[pair.payloadKey] = value
		}
	}

	uiState := map[string]any{}
	if value := strings.TrimSpace(sCtx["focusActivePane"]); value != "" {
		uiState["activePane"] = value
	}
	if value := strings.TrimSpace(sCtx["focusCenterTab"]); value != "" {
		uiState["centerTab"] = value
	}
	if len(uiState) > 0 {
		focus["uiState"] = uiState
	}

	return focus
}

func normalizeContextUpdates(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		out[trimmedKey] = strings.TrimSpace(value)
	}
	return out
}

func flattenFocusContext(focus map[string]any) map[string]string {
	if len(focus) == 0 {
		return map[string]string{}
	}

	out := map[string]string{}
	for _, pair := range []struct {
		focusKey   string
		contextKey string
	}{
		{focusKey: "surface", contextKey: "focusSurface"},
		{focusKey: "entityType", contextKey: "focusEntityType"},
		{focusKey: "entitySubtype", contextKey: "focusEntitySubtype"},
		{focusKey: "entityId", contextKey: "focusEntityId"},
		{focusKey: "entityVersion", contextKey: "focusEntityVersion"},
		{focusKey: "sectionId", contextKey: "focusSectionId"},
		{focusKey: "selectionText", contextKey: "focusSelectionText"},
		{focusKey: "lineIndex", contextKey: "focusLineIndex"},
	} {
		if value, ok := normalizeFocusValue(focus[pair.focusKey]); ok {
			out[pair.contextKey] = value
		}
	}

	if uiState, ok := focus["uiState"].(map[string]any); ok {
		if value, ok := normalizeFocusValue(uiState["activePane"]); ok {
			out["focusActivePane"] = value
		}
		if value, ok := normalizeFocusValue(uiState["centerTab"]); ok {
			out["focusCenterTab"] = value
		}
	}

	if raw, err := json.Marshal(focus); err == nil {
		out["focusContextJSON"] = string(raw)
	}

	return out
}

func normalizeFocusValue(raw any) (string, bool) {
	switch typed := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case int:
		return strconv.Itoa(typed), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		if raw == nil {
			return "", false
		}
		if encoded, err := json.Marshal(raw); err == nil {
			trimmed := strings.TrimSpace(string(encoded))
			if trimmed == "" || trimmed == "null" {
				return "", false
			}
			return trimmed, true
		}
		return "", false
	}
}

func readinessPayloadFromContext(sCtx map[string]string) (map[string]any, bool) {
	status := strings.TrimSpace(sCtx["readinessStatus"])
	if status == "" {
		return nil, false
	}
	payload := map[string]any{"status": status}
	if diagnostics := parseReadinessDiagnostics(sCtx["readinessDiagnostics"]); len(diagnostics) > 0 {
		payload["diagnostics"] = diagnostics
	}
	return payload, true
}

func parseReadinessDiagnostics(raw string) []map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var diagnostics []map[string]any
	if err := json.Unmarshal([]byte(trimmed), &diagnostics); err != nil {
		return nil
	}
	return diagnostics
}

func extractDocumentPatchProposal(reply string) (map[string]any, bool) {
	for _, candidate := range jsonCandidates(reply) {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(candidate), &decoded); err != nil {
			continue
		}
		kind, _ := decoded["type"].(string)
		if strings.TrimSpace(kind) != "document_patch_proposal" {
			continue
		}
		return decoded, true
	}
	return nil, false
}

func jsonCandidates(reply string) []string {
	trimmed := strings.TrimSpace(reply)
	if trimmed == "" {
		return nil
	}

	candidates := make([]string, 0, 3)
	matches := documentPatchJSONFenceRE.FindAllStringSubmatch(trimmed, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		candidate := strings.TrimSpace(match[1])
		if candidate != "" {
			candidates = append(candidates, candidate)
		}
	}

	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		candidates = append(candidates, trimmed)
	}

	return candidates
}

func firstResponseTimeoutFromContext(sCtx map[string]string) time.Duration {
	return durationFromContextSeconds(sCtx, "firstResponseTimeoutSec", defaultFirstResponseTimeout, 1, 300)
}

func heartbeatIntervalFromContext(sCtx map[string]string) time.Duration {
	return durationFromContextSeconds(sCtx, "streamHeartbeatSec", defaultHeartbeatInterval, 1, 120)
}

func durationFromContextSeconds(sCtx map[string]string, key string, fallback time.Duration, minSeconds int, maxSeconds int) time.Duration {
	if sCtx == nil {
		return fallback
	}
	raw := strings.TrimSpace(sCtx[key])
	if raw == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if seconds < minSeconds {
		seconds = minSeconds
	}
	if seconds > maxSeconds {
		seconds = maxSeconds
	}
	return time.Duration(seconds) * time.Second
}

func stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
