package completion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"smith/internal/source/model"
)

var (
	ErrCodeCommitFailed       = errors.New("code commit failed")
	ErrPullRequestFailed      = errors.New("pull request creation failed")
	ErrStateFinalizeFailed    = errors.New("state finalize failed")
	ErrCompensationFailed     = errors.New("compensation failed")
	ErrAmbiguousTerminalGuard = errors.New("ambiguous terminal prevented")
)

const (
	defaultCommitPushMaxAttempts = 3
	defaultPRCreateMaxAttempts   = 3
	defaultRetryInitialBackoff   = 1 * time.Second
	defaultRetryMaxBackoff       = 8 * time.Second
)

type Phase string

const (
	PhasePrepared         Phase = "prepared"
	PhaseCodeCommitted    Phase = "code_committed"
	PhaseStateCommitted   Phase = "state_committed"
	PhaseCompensated      Phase = "compensated"
	PhaseCompensationNeed Phase = "compensation_needed"
)

type PhaseRecord struct {
	LoopID      string
	Phase       Phase
	CommitSHA   string
	Description string
}

func (r PhaseRecord) ToJournal() model.JournalEntry {
	return model.JournalEntry{
		LoopID:    r.LoopID,
		Phase:     "completion",
		Level:     "info",
		ActorType: "replica",
		ActorID:   "smith-replica",
		Message:   fmt.Sprintf("phase: %s - %s", r.Phase, r.Description),
		Metadata: map[string]string{
			"completion_phase": string(r.Phase),
			"commit_sha":       r.CommitSHA,
		},
	}
}

type CommitRequest struct {
	LoopID        string
	CorrelationID string
	FinalDiff     string
	PullRequest   bool
	PRTitle       string
	PRBody        string
}

type Outcome string

const (
	OutcomeSynced               Outcome = "synced"
	OutcomeRetryable            Outcome = "retryable"
	OutcomeCompensationRequired Outcome = "compensation_required"
)

type CommitResult struct {
	Outcome   Outcome
	CommitSHA string
}

type PhaseStore interface {
	RecordPhase(ctx context.Context, record model.JournalEntry) error
	SetStateSynced(ctx context.Context, loopID string, commitSHA string) error
	SetStateUnresolved(ctx context.Context, loopID string, reason string) error
	AppendJournal(ctx context.Context, entry model.JournalEntry) error
}
type GitWriter interface {
	CommitAndPush(ctx context.Context, loopID string, finalDiff string) (string, error)
	CreatePullRequest(ctx context.Context, loopID string, commitSHA string, title string, body string) (string, error)
	Revert(ctx context.Context, loopID string, commitSHA string) error
}

type retryPolicy struct {
	CommitPushMaxAttempts int
	PRCreateMaxAttempts   int
	InitialBackoff        time.Duration
	MaxBackoff            time.Duration
}

func defaultRetryPolicy() retryPolicy {
	return retryPolicy{
		CommitPushMaxAttempts: defaultCommitPushMaxAttempts,
		PRCreateMaxAttempts:   defaultPRCreateMaxAttempts,
		InitialBackoff:        defaultRetryInitialBackoff,
		MaxBackoff:            defaultRetryMaxBackoff,
	}
}

type Protocol struct {
	store PhaseStore
	git   GitWriter
	retry retryPolicy
}

func NewProtocol(store PhaseStore, git GitWriter) *Protocol {
	return &Protocol{
		store: store,
		git:   git,
		retry: defaultRetryPolicy(),
	}
}

func (p *Protocol) Execute(ctx context.Context, req CommitRequest) (CommitResult, error) {
	if req.LoopID == "" {
		return CommitResult{}, errors.New("loop id is required")
	}

	if err := p.store.RecordPhase(ctx, PhaseRecord{
		LoopID:      req.LoopID,
		Phase:       PhasePrepared,
		Description: "completion protocol prepared",
	}.ToJournal()); err != nil {
		return CommitResult{}, err
	}

	commitSHA, err := p.commitAndPushWithRetry(ctx, req)
	if err != nil {
		_ = p.store.SetStateUnresolved(ctx, req.LoopID, "commit-push-failed")
		return CommitResult{
			Outcome: OutcomeRetryable,
		}, fmt.Errorf("%w: %v", ErrCodeCommitFailed, err)
	}

	if err := p.store.RecordPhase(ctx, PhaseRecord{
		LoopID:      req.LoopID,
		Phase:       PhaseCodeCommitted,
		CommitSHA:   commitSHA,
		Description: "code commit pushed",
	}.ToJournal()); err != nil {
		return CommitResult{}, err
	}

	if req.PullRequest {
		prURL, err := p.createPullRequestWithRetry(ctx, req, commitSHA)
		if err != nil {
			if isNoCommitsBetweenPR(err) {
				_ = p.store.AppendJournal(ctx, model.JournalEntry{
					LoopID:        req.LoopID,
					Phase:         "completion",
					Level:         "info",
					ActorType:     "replica",
					ActorID:       "smith-replica",
					Message:       "pull request skipped: no branch delta to merge",
					CorrelationID: req.CorrelationID,
					Metadata: map[string]string{
						"reason": "no_commits_between_head_and_base",
					},
				})
			} else {
				_ = p.store.SetStateUnresolved(ctx, req.LoopID, "pr-create-failed")
				_ = p.store.AppendJournal(ctx, model.JournalEntry{
					LoopID:        req.LoopID,
					Phase:         "completion",
					Level:         "warn",
					ActorType:     "replica",
					ActorID:       "smith-replica",
					Message:       "pull request creation failed after retries",
					CorrelationID: req.CorrelationID,
					Metadata: map[string]string{
						"error": err.Error(),
					},
				})
				return CommitResult{
					Outcome:   OutcomeRetryable,
					CommitSHA: commitSHA,
				}, fmt.Errorf("%w: %v", ErrPullRequestFailed, err)
			}
		} else {
			_ = p.store.AppendJournal(ctx, model.JournalEntry{
				LoopID:        req.LoopID,
				Phase:         "completion",
				Level:         "info",
				ActorType:     "replica",
				ActorID:       "smith-replica",
				Message:       "pull request created: " + prURL,
				CorrelationID: req.CorrelationID,
				Metadata: map[string]string{
					"pr_url": prURL,
				},
			})
		}
	}

	if err := p.store.SetStateSynced(ctx, req.LoopID, commitSHA); err != nil {

		_ = p.store.RecordPhase(ctx, PhaseRecord{
			LoopID:      req.LoopID,
			Phase:       PhaseCompensationNeed,
			CommitSHA:   commitSHA,
			Description: "state sync failed after code commit",
		}.ToJournal())

		if revertErr := p.git.Revert(ctx, req.LoopID, commitSHA); revertErr != nil {
			return CommitResult{
					Outcome:   OutcomeCompensationRequired,
					CommitSHA: commitSHA,
				}, errors.Join(
					fmt.Errorf("%w: %v", ErrStateFinalizeFailed, err),
					fmt.Errorf("%w: %v", ErrCompensationFailed, revertErr),
					ErrAmbiguousTerminalGuard,
				)
		}

		_ = p.store.RecordPhase(ctx, PhaseRecord{
			LoopID:      req.LoopID,
			Phase:       PhaseCompensated,
			CommitSHA:   commitSHA,
			Description: "commit reverted after state sync failure",
		}.ToJournal())
		_ = p.store.SetStateUnresolved(ctx, req.LoopID, "compensated-after-sync-failure")

		return CommitResult{
			Outcome:   OutcomeRetryable,
			CommitSHA: commitSHA,
		}, fmt.Errorf("%w: %v", ErrStateFinalizeFailed, err)
	}

	if err := p.store.RecordPhase(ctx, PhaseRecord{
		LoopID:      req.LoopID,
		Phase:       PhaseStateCommitted,
		CommitSHA:   commitSHA,
		Description: "state transitioned to synced",
	}.ToJournal()); err != nil {
		return CommitResult{}, err
	}

	return CommitResult{
		Outcome:   OutcomeSynced,
		CommitSHA: commitSHA,
	}, nil
}

func (p *Protocol) commitAndPushWithRetry(ctx context.Context, req CommitRequest) (string, error) {
	attempts := p.retry.CommitPushMaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		commitSHA, err := p.git.CommitAndPush(ctx, req.LoopID, req.FinalDiff)
		if err == nil {
			return commitSHA, nil
		}
		transient := isTransientTransportError(err)
		_ = p.store.AppendJournal(ctx, model.JournalEntry{
			LoopID:        req.LoopID,
			Phase:         "completion",
			Level:         "warn",
			ActorType:     "replica",
			ActorID:       "smith-replica",
			Message:       "commit/push attempt failed",
			CorrelationID: req.CorrelationID,
			Metadata: map[string]string{
				"operation": "commit_push",
				"attempt":   fmt.Sprintf("%d", attempt),
				"max":       fmt.Sprintf("%d", attempts),
				"transient": fmt.Sprintf("%t", transient),
				"error":     err.Error(),
			},
		})
		if !transient || attempt == attempts {
			return "", err
		}
		if err := waitForRetryBackoff(ctx, p.retry, attempt); err != nil {
			return "", err
		}
	}
	return "", errors.New("commit/push retries exhausted")
}

func (p *Protocol) createPullRequestWithRetry(ctx context.Context, req CommitRequest, commitSHA string) (string, error) {
	attempts := p.retry.PRCreateMaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		prURL, err := p.git.CreatePullRequest(ctx, req.LoopID, commitSHA, req.PRTitle, req.PRBody)
		if err == nil {
			return prURL, nil
		}
		transient := isTransientTransportError(err)
		_ = p.store.AppendJournal(ctx, model.JournalEntry{
			LoopID:        req.LoopID,
			Phase:         "completion",
			Level:         "warn",
			ActorType:     "replica",
			ActorID:       "smith-replica",
			Message:       "pull request attempt failed",
			CorrelationID: req.CorrelationID,
			Metadata: map[string]string{
				"operation": "create_pr",
				"attempt":   fmt.Sprintf("%d", attempt),
				"max":       fmt.Sprintf("%d", attempts),
				"transient": fmt.Sprintf("%t", transient),
				"error":     err.Error(),
			},
		})
		if !transient || attempt == attempts {
			return "", err
		}
		if err := waitForRetryBackoff(ctx, p.retry, attempt); err != nil {
			return "", err
		}
	}
	return "", errors.New("pr creation retries exhausted")
}

func waitForRetryBackoff(ctx context.Context, retry retryPolicy, attempt int) error {
	backoff := retry.InitialBackoff
	if backoff <= 0 {
		backoff = defaultRetryInitialBackoff
	}
	maxBackoff := retry.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = defaultRetryMaxBackoff
	}
	if attempt > 1 {
		for i := 1; i < attempt; i++ {
			backoff *= 2
			if backoff >= maxBackoff {
				backoff = maxBackoff
				break
			}
		}
	}
	if backoff <= 0 {
		return nil
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isTransientTransportError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	transientTokens := []string{
		"timeout",
		"temporarily unavailable",
		"temporary failure",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"i/o timeout",
		"tls handshake timeout",
		"gateway timeout",
		"bad gateway",
		"service unavailable",
		"eof",
		"rpc failed",
		"remote end hung up unexpectedly",
		"dial tcp",
	}
	for _, token := range transientTokens {
		if strings.Contains(message, token) {
			return true
		}
	}
	return false
}

func isNoCommitsBetweenPR(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no commits between")
}
