package prompts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"smith/internal/chat"
	"smith/internal/source/model"
)

type mockBridge struct {
	getLoopFn     func(ctx context.Context, loopID string) (*model.State, error)
	getJournalFn  func(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error)
	getDocumentFn func(ctx context.Context, docID string) (*model.Document, error)
}

func (m *mockBridge) GetLoop(ctx context.Context, loopID string) (*model.State, error) {
	if m.getLoopFn != nil {
		return m.getLoopFn(ctx, loopID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBridge) GetJournal(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error) {
	if m.getJournalFn != nil {
		return m.getJournalFn(ctx, loopID, limit)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBridge) GetDocument(ctx context.Context, docID string) (*model.Document, error) {
	if m.getDocumentFn != nil {
		return m.getDocumentFn(ctx, docID)
	}
	return nil, errors.New("not implemented")
}

func TestManager_BuildSystemPrompt(t *testing.T) {
	ctx := context.Background()

	t.Run("PRD Refinement - Without Document", func(t *testing.T) {
		bridge := &mockBridge{}
		manager := NewManager(bridge)
		session := &chat.Session{
			Type:    chat.SessionTypePRDRefinement,
			Context: map[string]string{},
		}

		prompt, err := manager.BuildSystemPrompt(ctx, session)
		assert.NoError(t, err)
		assert.Contains(t, prompt, "Smith Interactive Chat Assistant")
		assert.Contains(t, prompt, "prd-refinement")
		assert.Contains(t, prompt, "CONTEXT: PRD Refinement")
		assert.NotContains(t, prompt, "Document Title:")
	})

	t.Run("PRD Refinement - With Document", func(t *testing.T) {
		bridge := &mockBridge{
			getDocumentFn: func(ctx context.Context, docID string) (*model.Document, error) {
				assert.Equal(t, "doc-123", docID)
				return &model.Document{
					Title:   "Test PRD",
					Content: "PRD Content here",
				}, nil
			},
		}
		manager := NewManager(bridge)
		session := &chat.Session{
			Type: chat.SessionTypePRDRefinement,
			Context: map[string]string{
				"documentId": "doc-123",
			},
		}

		prompt, err := manager.BuildSystemPrompt(ctx, session)
		assert.NoError(t, err)
		assert.Contains(t, prompt, "Document Title: Test PRD")
		assert.Contains(t, prompt, "Current Content:\nPRD Content here")
	})

	t.Run("PRD Refinement - Document Error Fallback", func(t *testing.T) {
		bridge := &mockBridge{
			getDocumentFn: func(ctx context.Context, docID string) (*model.Document, error) {
				return nil, errors.New("document not found")
			},
		}
		manager := NewManager(bridge)
		session := &chat.Session{
			Type: chat.SessionTypePRDRefinement,
			Context: map[string]string{
				"documentId": "doc-123",
			},
		}

		prompt, err := manager.BuildSystemPrompt(ctx, session)
		assert.NoError(t, err)
		assert.Contains(t, prompt, "CONTEXT: PRD Refinement")
		assert.NotContains(t, prompt, "Document Title:")
	})

	t.Run("Loop Assist - Without Loop", func(t *testing.T) {
		bridge := &mockBridge{}
		manager := NewManager(bridge)
		session := &chat.Session{
			Type:    chat.SessionTypeLoopAssist,
			Context: map[string]string{},
		}

		prompt, err := manager.BuildSystemPrompt(ctx, session)
		assert.NoError(t, err)
		assert.Contains(t, prompt, "loop-assist")
		assert.Contains(t, prompt, "CONTEXT: Loop Assist")
		assert.NotContains(t, prompt, "Loop ID:")
	})

	t.Run("Loop Assist - With Loop and Journal", func(t *testing.T) {
		fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		bridge := &mockBridge{
			getLoopFn: func(ctx context.Context, loopID string) (*model.State, error) {
				assert.Equal(t, "loop-123", loopID)
				return &model.State{
					LoopID: "loop-123",
					State:  model.LoopStateRunning,
					Reason: "testing",
				}, nil
			},
			getJournalFn: func(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error) {
				assert.Equal(t, "loop-123", loopID)
				assert.Equal(t, int64(10), limit)
				return []model.JournalEntry{
					{
						Timestamp: fixedTime,
						Message:   "First entry",
					},
					{
						Timestamp: fixedTime.Add(time.Second),
						Message:   "Second entry",
					},
				}, nil
			},
		}
		manager := NewManager(bridge)
		session := &chat.Session{
			Type: chat.SessionTypeLoopAssist,
			Context: map[string]string{
				"loopId": "loop-123",
			},
		}

		prompt, err := manager.BuildSystemPrompt(ctx, session)
		assert.NoError(t, err)
		assert.Contains(t, prompt, "Loop ID: loop-123")
		assert.Contains(t, prompt, "Status: running")
		assert.Contains(t, prompt, "Reason: testing")
		assert.Contains(t, prompt, "Recent Journal Entries:")
		assert.Contains(t, prompt, "[12:00:00] First entry")
		assert.Contains(t, prompt, "[12:00:01] Second entry")
	})

	t.Run("Loop Assist - Loop Error Fallback", func(t *testing.T) {
		bridge := &mockBridge{
			getLoopFn: func(ctx context.Context, loopID string) (*model.State, error) {
				return nil, errors.New("loop not found")
			},
			getJournalFn: func(ctx context.Context, loopID string, limit int64) ([]model.JournalEntry, error) {
				return nil, errors.New("journal not found")
			},
		}
		manager := NewManager(bridge)
		session := &chat.Session{
			Type: chat.SessionTypeLoopAssist,
			Context: map[string]string{
				"loopId": "loop-123",
			},
		}

		prompt, err := manager.BuildSystemPrompt(ctx, session)
		assert.NoError(t, err)
		assert.Contains(t, prompt, "CONTEXT: Loop Assist")
		assert.NotContains(t, prompt, "Loop ID:")
	})

	t.Run("Document Assist - With Document", func(t *testing.T) {
		bridge := &mockBridge{
			getDocumentFn: func(ctx context.Context, docID string) (*model.Document, error) {
				return &model.Document{
					Title:   "Assist Doc",
					Content: "Assist Content",
				}, nil
			},
		}
		manager := NewManager(bridge)
		session := &chat.Session{
			Type: chat.SessionTypeDocumentAssist,
			Context: map[string]string{
				"documentId": "doc-assist",
			},
		}

		prompt, err := manager.BuildSystemPrompt(ctx, session)
		assert.NoError(t, err)
		assert.Contains(t, prompt, "CONTEXT: Document Assist")
		assert.Contains(t, prompt, "Document: Assist Doc")
		assert.Contains(t, prompt, "Content:\nAssist Content")
	})

	t.Run("Unknown Session Type", func(t *testing.T) {
		bridge := &mockBridge{}
		manager := NewManager(bridge)
		session := &chat.Session{
			Type:    "unknown-type",
			Context: map[string]string{},
		}

		prompt, err := manager.BuildSystemPrompt(ctx, session)
		assert.NoError(t, err)
		assert.Contains(t, prompt, "Smith Interactive Chat Assistant")
		assert.Contains(t, prompt, "unknown-type")
		assert.Contains(t, prompt, "Follow the operator's instructions")
		assert.NotContains(t, prompt, "CONTEXT:")
	})
}
