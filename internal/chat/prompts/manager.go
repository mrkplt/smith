package prompts

import (
	"context"
	"fmt"
	"smith/internal/chat"
	"smith/internal/chat/smithbridge"
	"strings"
)

type Manager struct {
	bridge smithbridge.Bridge
}

func NewManager(b smithbridge.Bridge) *Manager {
	return &Manager{bridge: b}
}

func (m *Manager) BuildSystemPrompt(ctx context.Context, session *chat.Session) (string, error) {
	var sb strings.Builder
	sb.WriteString("You are the Smith Interactive Chat Assistant. ")
	sb.WriteString(fmt.Sprintf("You are currently in a %s session.\n\n", session.Type))
	m.injectApplicationContext(&sb, session.Context)

	switch session.Type {
	case chat.SessionTypePRDRefinement:
		m.injectPRDContext(ctx, &sb, session.Context)
	case chat.SessionTypeLoopAssist:
		m.injectLoopContext(ctx, &sb, session.Context)
	case chat.SessionTypeDocumentAssist:
		m.injectDocumentContext(ctx, &sb, session.Context)
	}

	sb.WriteString("\nFollow the operator's instructions and provide helpful, concise responses.")
	return sb.String(), nil
}

func (m *Manager) injectApplicationContext(sb *strings.Builder, sCtx map[string]string) {
	if len(sCtx) == 0 {
		return
	}

	var lines []string
	if app := strings.TrimSpace(sCtx["app"]); app != "" {
		lines = append(lines, fmt.Sprintf("Application: %s", app))
	}
	if surface := strings.TrimSpace(sCtx["surface"]); surface != "" {
		lines = append(lines, fmt.Sprintf("Surface: %s", surface))
	}
	if route := strings.TrimSpace(sCtx["route"]); route != "" {
		lines = append(lines, fmt.Sprintf("Route: %s", route))
	}
	if projectID := strings.TrimSpace(sCtx["projectId"]); projectID != "" {
		lines = append(lines, fmt.Sprintf("Project ID: %s", projectID))
	}
	if loopID := strings.TrimSpace(sCtx["loopId"]); loopID != "" {
		lines = append(lines, fmt.Sprintf("Loop ID Hint: %s", loopID))
	}
	if documentID := strings.TrimSpace(sCtx["documentId"]); documentID != "" {
		lines = append(lines, fmt.Sprintf("Document ID Hint: %s", documentID))
	}
	if provider := strings.TrimSpace(sCtx["provider"]); provider != "" {
		lines = append(lines, fmt.Sprintf("Preferred Provider: %s", provider))
	}
	if model := strings.TrimSpace(sCtx["model"]); model != "" {
		lines = append(lines, fmt.Sprintf("Preferred Model: %s", model))
	}
	if thinking := strings.TrimSpace(sCtx["thinkingLevel"]); thinking != "" {
		lines = append(lines, fmt.Sprintf("Thinking Level: %s", thinking))
	}
	if projectCount := strings.TrimSpace(sCtx["projectsCount"]); projectCount != "" {
		lines = append(lines, fmt.Sprintf("Projects Loaded: %s", projectCount))
	}
	if loopCount := strings.TrimSpace(sCtx["loopsCount"]); loopCount != "" {
		lines = append(lines, fmt.Sprintf("Pods Loaded: %s", loopCount))
	}
	if activeLoopCount := strings.TrimSpace(sCtx["activeLoopsCount"]); activeLoopCount != "" {
		lines = append(lines, fmt.Sprintf("Active Pods: %s", activeLoopCount))
	}
	if documentCount := strings.TrimSpace(sCtx["documentsCount"]); documentCount != "" {
		lines = append(lines, fmt.Sprintf("Documents Loaded: %s", documentCount))
	}

	if len(lines) == 0 {
		return
	}

	sb.WriteString("CONTEXT: Application\n")
	for _, line := range lines {
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n")
}

func (m *Manager) injectPRDContext(ctx context.Context, sb *strings.Builder, sCtx map[string]string) {
	sb.WriteString("CONTEXT: PRD Refinement\n")
	if docID, ok := sCtx["documentId"]; ok {
		doc, err := m.bridge.GetDocument(ctx, docID)
		if err == nil {
			sb.WriteString(fmt.Sprintf("Document Title: %s\n", doc.Title))
			sb.WriteString(fmt.Sprintf("Current Content:\n%s\n", doc.Content))
		}
	}
}

func (m *Manager) injectLoopContext(ctx context.Context, sb *strings.Builder, sCtx map[string]string) {
	sb.WriteString("CONTEXT: Loop Assist\n")
	if loopID, ok := sCtx["loopId"]; ok {
		loop, err := m.bridge.GetLoop(ctx, loopID)
		if err == nil {
			sb.WriteString(fmt.Sprintf("Loop ID: %s\n", loop.LoopID))
			sb.WriteString(fmt.Sprintf("Status: %s\n", loop.State))
			sb.WriteString(fmt.Sprintf("Reason: %s\n", loop.Reason))
		}

		journal, err := m.bridge.GetJournal(ctx, loopID, 10)
		if err == nil && len(journal) > 0 {
			sb.WriteString("Recent Journal Entries:\n")
			for _, entry := range journal {
				sb.WriteString(fmt.Sprintf("- [%s] %s\n", entry.Timestamp.Format("15:04:05"), entry.Message))
			}
		}
	}
}

func (m *Manager) injectDocumentContext(ctx context.Context, sb *strings.Builder, sCtx map[string]string) {
	sb.WriteString("CONTEXT: Document Assist\n")
	if docID, ok := sCtx["documentId"]; ok {
		doc, err := m.bridge.GetDocument(ctx, docID)
		if err == nil {
			sb.WriteString(fmt.Sprintf("Document: %s\n", doc.Title))
			sb.WriteString(fmt.Sprintf("Content:\n%s\n", doc.Content))
		}
	}
}
