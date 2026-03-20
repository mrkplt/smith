package prompts

import (
	"context"
	"encoding/json"
	"fmt"
	"smith/internal/chat"
	"smith/internal/chat/smithbridge"
	"smith/internal/source/model"
	"strings"
	"time"
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

	intent := resolveSessionIntent(chat.SessionTypePRDRefinement, sCtx)
	envelope, warnings := m.buildPRDContextEnvelope(ctx, sCtx, intent)
	serialized, err := json.MarshalIndent(envelope, "", "  ")
	if err == nil {
		sb.WriteString("Context Envelope:\n")
		sb.WriteString(string(serialized) + "\n")
	}
	for _, warning := range warnings {
		sb.WriteString(fmt.Sprintf("Context Warning: %s\n", warning))
	}

	if docID := strings.TrimSpace(sCtx["documentId"]); docID != "" && m.bridge != nil {
		doc, getErr := m.bridge.GetDocument(ctx, docID)
		if getErr == nil {
			sb.WriteString(fmt.Sprintf("Document Title: %s\n", doc.Title))
			sb.WriteString(fmt.Sprintf("Document Version: %s\n", documentVersion(doc)))
			sb.WriteString(fmt.Sprintf("Current Content:\n%s\n", truncateForPrompt(doc.Content, 16000)))
			report, ok := readinessReportForDocument(doc)
			if ok {
				sb.WriteString(fmt.Sprintf("Readiness Status: %s\n", report.Readiness))
				sb.WriteString(fmt.Sprintf("Readiness Diagnostics: errors=%d warnings=%d\n", len(report.Errors), len(report.Warnings)))
			}
		}
	}

	sb.WriteString("Session Intent: " + intent + "\n")
	sb.WriteString("Behavior Requirements:\n")
	sb.WriteString("- Ground every response in the bound document and context envelope.\n")
	sb.WriteString("- Identify missing requirements, constraints, acceptance criteria, and edge cases.\n")
	sb.WriteString("- Ask targeted clarifying questions before making assumptions.\n")
	sb.WriteString("- Do not claim document changes are applied until the user approves a patch.\n")
	sb.WriteString("- When proposing edits, include a JSON patch proposal with this shape:\n")
	sb.WriteString("  {\"type\":\"document_patch_proposal\",\"operations\":[{\"op\":\"replace_document\",\"content\":\"<full revised markdown>\"}]}\n")
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

func (m *Manager) buildPRDContextEnvelope(ctx context.Context, sCtx map[string]string, intent string) (map[string]any, []string) {
	workspace := map[string]any{}
	related := map[string]any{}
	document := map[string]any{}
	focus := map[string]any{}
	warnings := make([]string, 0, 1)

	if app := strings.TrimSpace(sCtx["app"]); app != "" {
		workspace["app"] = app
	}
	if surface := strings.TrimSpace(sCtx["surface"]); surface != "" {
		workspace["surface"] = surface
	}
	if route := strings.TrimSpace(sCtx["route"]); route != "" {
		workspace["route"] = route
	}
	if projectID := strings.TrimSpace(sCtx["projectId"]); projectID != "" {
		workspace["projectId"] = projectID
	}
	if workingDirectory := strings.TrimSpace(sCtx["workingDirectory"]); workingDirectory != "" {
		workspace["workingDirectory"] = workingDirectory
	}

	if projectsCount := strings.TrimSpace(sCtx["projectsCount"]); projectsCount != "" {
		related["projectsCount"] = projectsCount
	}
	if loopsCount := strings.TrimSpace(sCtx["loopsCount"]); loopsCount != "" {
		related["loopsCount"] = loopsCount
	}
	if activeLoopsCount := strings.TrimSpace(sCtx["activeLoopsCount"]); activeLoopsCount != "" {
		related["activeLoopsCount"] = activeLoopsCount
	}
	if documentsCount := strings.TrimSpace(sCtx["documentsCount"]); documentsCount != "" {
		related["documentsCount"] = documentsCount
	}

	for _, pair := range []struct {
		contextKey string
		focusKey   string
	}{
		{contextKey: "focusSurface", focusKey: "surface"},
		{contextKey: "focusEntityType", focusKey: "entityType"},
		{contextKey: "focusEntitySubtype", focusKey: "entitySubtype"},
		{contextKey: "focusEntityId", focusKey: "entityId"},
		{contextKey: "focusEntityVersion", focusKey: "entityVersion"},
		{contextKey: "focusSectionId", focusKey: "sectionId"},
		{contextKey: "focusSelectionText", focusKey: "selectionText"},
		{contextKey: "focusLineIndex", focusKey: "lineIndex"},
	} {
		if value := strings.TrimSpace(sCtx[pair.contextKey]); value != "" {
			focus[pair.focusKey] = value
		}
	}
	uiState := map[string]any{}
	if activePane := strings.TrimSpace(sCtx["focusActivePane"]); activePane != "" {
		uiState["activePane"] = activePane
	}
	if centerTab := strings.TrimSpace(sCtx["focusCenterTab"]); centerTab != "" {
		uiState["centerTab"] = centerTab
	}
	if len(uiState) > 0 {
		focus["uiState"] = uiState
	}
	if raw := strings.TrimSpace(sCtx["focusContextJSON"]); raw != "" {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil && len(decoded) > 0 {
			focus = decoded
		}
	}

	if docID := strings.TrimSpace(sCtx["documentId"]); docID != "" {
		document["id"] = docID
	}
	if title := strings.TrimSpace(sCtx["documentTitle"]); title != "" {
		document["title"] = title
	}
	if format := strings.TrimSpace(sCtx["documentFormat"]); format != "" {
		document["format"] = format
	}
	if status := strings.TrimSpace(sCtx["documentStatus"]); status != "" {
		document["status"] = status
	}

	boundVersion := strings.TrimSpace(sCtx["documentVersion"])
	if boundVersion != "" {
		document["boundVersion"] = boundVersion
	}

	if readinessStatus := strings.TrimSpace(sCtx["readinessStatus"]); readinessStatus != "" {
		readiness := map[string]any{"status": readinessStatus}
		if diagnostics := parseDiagnosticsJSON(strings.TrimSpace(sCtx["readinessDiagnostics"])); len(diagnostics) > 0 {
			readiness["diagnostics"] = diagnostics
		}
		document["readiness"] = readiness
	}

	if docID, ok := document["id"].(string); ok && docID != "" && m.bridge != nil {
		doc, err := m.bridge.GetDocument(ctx, docID)
		if err == nil {
			document["title"] = doc.Title
			document["format"] = doc.Format
			document["status"] = doc.Status
			document["version"] = documentVersion(doc)
			document["content"] = truncateForPrompt(doc.Content, 16000)
			document["metadata"] = doc.Metadata

			if boundVersion != "" && boundVersion != documentVersion(doc) {
				document["versionDrift"] = true
				warnings = append(warnings, "document changed since this session was bound; user should review drift before applying patches")
			}

			if report, ok := readinessReportForDocument(doc); ok {
				document["readiness"] = map[string]any{
					"status":      report.Readiness,
					"errors":      diagnosticsToMaps(report.Errors, 8),
					"warnings":    diagnosticsToMaps(report.Warnings, 8),
					"valid":       report.Valid,
					"evaluatedAt": time.Now().UTC().Format(time.RFC3339),
				}
			}
		}
	}

	return map[string]any{
		"workspace":      workspace,
		"document":       document,
		"focus":          focus,
		"relatedContext": related,
		"sessionIntent":  intent,
	}, warnings
}

func resolveSessionIntent(sessionType chat.SessionType, sCtx map[string]string) string {
	if intent := strings.TrimSpace(sCtx["sessionIntent"]); intent != "" {
		return intent
	}

	switch sessionType {
	case chat.SessionTypePRDRefinement, chat.SessionTypeDocumentAssist:
		return "document_refinement"
	case chat.SessionTypeLoopAssist:
		return "implementation_planning"
	case chat.SessionTypeRuntimeAssist:
		return "build_execution"
	default:
		return "document_refinement"
	}
}

func parseDiagnosticsJSON(raw string) []map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil
	}
	return decoded
}

func diagnosticsToMaps(in []model.PRDValidationDiagnostic, limit int) []map[string]any {
	if len(in) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(in) {
		limit = len(in)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		item := in[i]
		entry := map[string]any{
			"code":    item.Code,
			"path":    item.Path,
			"message": item.Message,
		}
		if strings.TrimSpace(item.StoryID) != "" {
			entry["storyId"] = item.StoryID
		}
		if strings.TrimSpace(item.Suggestion) != "" {
			entry["suggestion"] = item.Suggestion
		}
		out = append(out, entry)
	}
	return out
}

func readinessReportForDocument(doc *model.Document) (*model.PRDValidationReport, bool) {
	if doc == nil {
		return nil, false
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		return nil, false
	}

	format := strings.ToLower(strings.TrimSpace(doc.Format))
	if format == "" {
		if strings.HasPrefix(content, "{") {
			format = "json"
		} else {
			format = "markdown"
		}
	}

	var report model.PRDValidationReport
	switch format {
	case "json", "structured":
		_, report = model.ValidatePRDJSON([]byte(content))
	case "markdown", "md":
		_, report = model.ValidatePRDMarkdown([]byte(content))
	default:
		return nil, false
	}

	return &report, true
}

func documentVersion(doc *model.Document) string {
	if doc == nil {
		return ""
	}
	if !doc.UpdatedAt.IsZero() {
		return doc.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	if strings.TrimSpace(doc.SchemaVersion) != "" {
		return strings.TrimSpace(doc.SchemaVersion)
	}
	return ""
}

func truncateForPrompt(value string, limit int) string {
	if limit <= 0 {
		limit = 16000
	}
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n\n[truncated for context budget]"
}
