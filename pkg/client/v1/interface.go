package v1

import (
	"context"
	"net/http"

	api "smith/pkg/api/v1"
)

// Interface is the Smith API client interface
type Interface interface {
	CreateLoop(ctx context.Context, req api.LoopCreateRequest) (*api.LoopCreateResult, error)
	CreateLoops(ctx context.Context, req api.LoopBatchRequest) (*api.LoopCreateResult, error)
	GetLoop(ctx context.Context, loopID string) (*api.LoopResponse, error)
	ListLoops(ctx context.Context) ([]api.LoopWithRevision, error)
	TraceLoop(ctx context.Context, loopID string) (*api.LoopTraceResponse, error)
	GetJournal(ctx context.Context, loopID string, limit int64) ([]api.JournalEntry, error)
	GetRuntime(ctx context.Context, loopID string) (*api.LoopRuntimeResponse, error)
	AttachTerminal(ctx context.Context, loopID string, req api.TerminalAttachRequest) (map[string]any, error)
	DetachTerminal(ctx context.Context, loopID string, req api.TerminalDetachRequest) (map[string]any, error)
	SendCommand(ctx context.Context, loopID string, req api.TerminalCommandRequest) (map[string]any, error)
	CreateChatSession(ctx context.Context, req api.ChatCreateSessionRequest) (*api.ChatSession, error)
	PostChatMessage(ctx context.Context, sessionID string, req api.ChatPostMessageRequest) (*api.ChatPostMessageResponse, error)
	OpenChatStream(ctx context.Context, sessionID string) (*http.Response, error)
	CommitChatAction(ctx context.Context, req api.ChatCommitActionRequest) (*api.ChatCommitActionResponse, error)
	SubmitPRD(ctx context.Context, req api.PRDIngressRequest) (*api.IngressSummary, error)
	IngestGitHubIssues(ctx context.Context, req api.GitHubIngressRequest) (*api.IngressSummary, error)
	OverrideLoop(ctx context.Context, req api.OverrideRequest) (*api.OverrideResponse, error)
	SetHTTPClient(httpClient *http.Client)
	Do(ctx context.Context, method, path string, body any, out any) error
}

var _ Interface = (*Client)(nil)
