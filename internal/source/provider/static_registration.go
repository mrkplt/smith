package provider

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type staticAdapter struct {
	providerID   string
	defaultModel string
	models       map[string]struct{}
	sessionCtr   uint64
}

func NewClaudeRegistration() Registration {
	return newStaticRegistration(ProviderClaude, DefaultClaudeModel, []string{DefaultClaudeModel, ClaudeHaikuModel})
}

func NewGeminiRegistration() Registration {
	return newStaticRegistration(ProviderGemini, DefaultGeminiModel, []string{DefaultGeminiModel, GeminiFlashModel})
}

func newStaticRegistration(providerID, defaultModel string, models []string) Registration {
	supported := make(map[string]struct{}, len(models))
	normalizedModels := make([]string, 0, len(models))
	for _, model := range models {
		normalized := normalize(model)
		if normalized == "" {
			continue
		}
		normalizedModels = append(normalizedModels, normalized)
		supported[normalized] = struct{}{}
	}
	return Registration{
		ProviderID:   providerID,
		DefaultModel: normalize(defaultModel),
		Models:       normalizedModels,
		Adapter: &staticAdapter{
			providerID:   normalize(providerID),
			defaultModel: normalize(defaultModel),
			models:       supported,
		},
	}
}

func (a *staticAdapter) CreateSession(_ context.Context, request SessionRequest) (Session, error) {
	model := normalize(request.Model)
	if model == "" {
		model = a.defaultModel
	}
	if err := a.ValidateConfig(Config{Model: model, Options: request.Options}); err != nil {
		return Session{}, err
	}
	id := atomic.AddUint64(&a.sessionCtr, 1)
	return Session{
		ID:         fmt.Sprintf("%s-%d", a.providerID, id),
		ProviderID: a.providerID,
		Model:      model,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func (a *staticAdapter) SendTurn(_ context.Context, session Session, input TurnInput) (TurnResult, error) {
	if err := a.ValidateConfig(Config{Model: session.Model}); err != nil {
		return TurnResult{}, err
	}
	if strings.TrimSpace(input.Content) == "" {
		return TurnResult{}, fmt.Errorf("input content is required")
	}
	return TurnResult{Output: fmt.Sprintf("%s:%s", a.providerID, strings.TrimSpace(input.Content))}, nil
}

func (a *staticAdapter) StreamEvents(ctx context.Context, session Session) (<-chan Event, error) {
	if err := a.ValidateConfig(Config{Model: session.Model}); err != nil {
		return nil, err
	}
	ch := make(chan Event, 1)
	ch <- Event{Type: "session_started", Message: session.ID, Timestamp: time.Now().UTC()}
	close(ch)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return ch, nil
	}
}

func (a *staticAdapter) CloseSession(context.Context, Session) error {
	return nil
}

func (a *staticAdapter) ValidateConfig(config Config) error {
	model := normalize(config.Model)
	if _, ok := a.models[model]; ok {
		return nil
	}
	return fmt.Errorf("%w: provider=%s model=%s", ErrUnsupportedModel, a.providerID, model)
}
