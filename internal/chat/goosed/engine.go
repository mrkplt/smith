package goosed

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"smith/internal/chat"
	"strings"
	"sync"
)

type Engine struct {
	agentCmd string
}

func NewEngine() *Engine {
	return &Engine{
		agentCmd: "goose acp",
	}
}

func (e *Engine) Stream(ctx context.Context, session *chat.Session, message string, events chan<- chat.ChatEvent) error {
	fields := strings.Fields(e.agentCmd)
	if len(fields) == 0 {
		return fmt.Errorf("agent command is empty")
	}

	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	if wd := session.Context["workingDirectory"]; wd != "" {
		cmd.Dir = wd
	}

	overrides := gooseEnvOverrides(session.Context)
	if len(overrides) > 0 {
		cmd.Env = append(os.Environ(), overrides...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to open goose stdin: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to open goose stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to open goose stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start goose: %w", err)
	}

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			_ = stdin.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
		})
	}
	defer cleanup()

	const (
		initializeRequestID = 1
		newSessionRequestID = 2
		promptRequestID     = 3
	)

	sendRPC := func(method string, params any, id int) error {
		req := map[string]any{
			"jsonrpc": "2.0",
			"method":  method,
			"params":  params,
			"id":      id,
		}

		data, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to encode %s request: %w", method, err)
		}
		if _, err := stdin.Write(data); err != nil {
			return fmt.Errorf("failed to write %s request: %w", method, err)
		}
		if _, err := stdin.Write([]byte("\n")); err != nil {
			return fmt.Errorf("failed to terminate %s request: %w", method, err)
		}
		return nil
	}

	if err := sendRPC("initialize", map[string]any{
		"protocolVersion":    "v1",
		"clientCapabilities": map[string]any{},
		"clientInfo":         map[string]any{"name": "smith-chat", "version": "1.0.0"},
	}, initializeRequestID); err != nil {
		return err
	}

	go func() {
		s := bufio.NewScanner(stderr)
		s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for s.Scan() {
			_ = s.Text()
		}
	}()

	done := make(chan error, 1)

	go func() {
		var sessionID string
		s := bufio.NewScanner(stdout)
		s.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for s.Scan() {
			var rpc struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}

			if err := json.Unmarshal(s.Bytes(), &rpc); err != nil {
				continue
			}

			if rpc.Error != nil {
				events <- chat.ChatEvent{Event: chat.EventError, Data: rpc.Error.Message}
				done <- fmt.Errorf("goose rpc error (%d): %s", rpc.Error.Code, rpc.Error.Message)
				return
			}

			var id int
			if len(rpc.ID) > 0 {
				_ = json.Unmarshal(rpc.ID, &id)
			}

			switch {
			case id == initializeRequestID:
				cwd := "."
				if cmd.Dir != "" {
					cwd = cmd.Dir
				}
				if err := sendRPC("session/new", map[string]any{
					"mcpServers": []any{},
					"cwd":        cwd,
				}, newSessionRequestID); err != nil {
					done <- err
					return
				}

			case id == newSessionRequestID:
				var res struct {
					SessionID string `json:"sessionId"`
				}
				if err := json.Unmarshal(rpc.Result, &res); err != nil {
					done <- fmt.Errorf("failed to parse session/new result: %w", err)
					return
				}
				sessionID = res.SessionID
				if sessionID == "" {
					done <- fmt.Errorf("goose returned empty sessionId")
					return
				}

				if err := sendRPC("session/prompt", map[string]any{
					"sessionId": sessionID,
					"prompt": []map[string]any{
						{"type": "text", "text": message},
					},
				}, promptRequestID); err != nil {
					done <- err
					return
				}

			case rpc.Method == "session/update":
				var params struct {
					SessionID string `json:"sessionId"`
					Update    struct {
						SessionUpdate string          `json:"sessionUpdate"`
						Content       json.RawMessage `json:"content"`
					} `json:"update"`
				}
				if err := json.Unmarshal(rpc.Params, &params); err != nil {
					continue
				}
				if sessionID != "" && params.SessionID != "" && params.SessionID != sessionID {
					continue
				}

				switch params.Update.SessionUpdate {
				case "agent_message_chunk":
					var content struct {
						Text string `json:"text"`
					}
					if err := json.Unmarshal(params.Update.Content, &content); err == nil && content.Text != "" {
						events <- chat.ChatEvent{Event: chat.EventMessageDelta, Data: chat.MessageDelta{Delta: content.Text}}
					}
				}

			case id == promptRequestID:
				events <- chat.ChatEvent{Event: chat.EventMessageCompleted, Data: map[string]any{}}
				done <- nil
				return
			}
		}

		if err := s.Err(); err != nil && err != io.EOF {
			done <- fmt.Errorf("failed to read goose output: %w", err)
			return
		}
		done <- fmt.Errorf("goose stream ended before prompt completion")
	}()

	select {
	case <-ctx.Done():
		cleanup()
		return ctx.Err()
	case err := <-done:
		cleanup()
		return err
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func gooseEnvOverrides(sessionContext map[string]string) []string {
	provider := strings.ToLower(firstNonEmpty(sessionContext["gooseProvider"], sessionContext["provider"]))
	model := firstNonEmpty(sessionContext["gooseModel"], sessionContext["model"])
	apiKey := firstNonEmpty(sessionContext["providerApiKey"], sessionContext["apiKey"])

	var env []string
	if provider != "" {
		env = append(env, "GOOSE_PROVIDER="+provider)
	}
	if model != "" {
		env = append(env, "GOOSE_MODEL="+model)
	}
	if thinkingLevel := normalizeThinkingLevel(sessionContext["thinkingLevel"]); thinkingLevel != "" {
		env = append(env, "OPENAI_REASONING_EFFORT="+thinkingLevel)
	}
	if apiKey != "" {
		for _, keyEnv := range providerKeyEnvVars(provider) {
			env = append(env, keyEnv+"="+apiKey)
		}
	}
	return env
}

func providerKeyEnvVars(provider string) []string {
	switch provider {
	case "anthropic":
		return []string{"ANTHROPIC_API_KEY"}
	case "google":
		return []string{"GOOGLE_API_KEY", "GEMINI_API_KEY"}
	case "openai", "":
		return []string{"OPENAI_API_KEY"}
	default:
		return []string{"OPENAI_API_KEY"}
	}
}

func normalizeThinkingLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "quick":
		return "low"
	case "balanced":
		return "medium"
	case "deep":
		return "high"
	default:
		return ""
	}
}
