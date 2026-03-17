package completion

import (
	"context"
	"errors"
	"strings"
	"testing"

	"smith/internal/source/model"
)

func TestExecuteSuccess(t *testing.T) {
	store := &fakeStore{}
	git := &fakeGit{commitSHA: "abc123"}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-1",
		CorrelationID: "corr-1",
		FinalDiff:     "diff",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != OutcomeSynced {
		t.Fatalf("expected synced outcome, got %q", result.Outcome)
	}
	if result.CommitSHA != "abc123" {
		t.Fatalf("unexpected commit sha %q", result.CommitSHA)
	}
	assertHasPhase(t, store.phases, PhasePrepared)
	assertHasPhase(t, store.phases, PhaseCodeCommitted)
	assertHasPhase(t, store.phases, PhaseStateCommitted)
}

func TestExecuteSuccessWithPullRequest(t *testing.T) {
	store := &fakeStore{}
	git := &fakeGit{commitSHA: "abc123", prURL: "https://github.com/pr/1"}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-pr",
		CorrelationID: "corr-pr",
		FinalDiff:     "diff",
		PullRequest:   true,
		PRTitle:       "Title",
		PRBody:        "Body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if git.prCalls != 1 {
		t.Fatalf("expected one PR call, got %d", git.prCalls)
	}
	if result.Outcome != OutcomeSynced {
		t.Fatalf("expected synced outcome, got %q", result.Outcome)
	}
	assertHasPhase(t, store.phases, PhasePrepared)
	assertHasPhase(t, store.phases, PhaseCodeCommitted)
	assertHasPhase(t, store.phases, PhaseStateCommitted)

	// Verify PR URL journaled
	foundPR := false
	for _, journal := range store.phases {
		if journal.Metadata != nil && journal.Metadata["pr_url"] == "https://github.com/pr/1" {
			foundPR = true
			break
		}
	}
	if !foundPR {
		t.Fatal("PR URL not found in journal")
	}
}

func TestExecuteRetriesTransientCommitPushFailure(t *testing.T) {
	store := &fakeStore{}
	git := &fakeGit{commitSeq: []gitCommitResult{
		{err: errors.New("i/o timeout")},
		{sha: "abc123"},
	}}
	p := NewProtocol(store, git)
	p.retry.InitialBackoff = 0
	p.retry.MaxBackoff = 0

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-retry-commit",
		CorrelationID: "corr-retry-commit",
		FinalDiff:     "diff",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != OutcomeSynced {
		t.Fatalf("expected synced outcome, got %q", result.Outcome)
	}
	if git.commitCalls != 2 {
		t.Fatalf("expected two commit attempts, got %d", git.commitCalls)
	}
}

func TestExecutePullRequestFailureReturnsError(t *testing.T) {
	store := &fakeStore{}
	git := &fakeGit{commitSHA: "abc123", prErr: errors.New("permission denied")}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-pr-fail",
		CorrelationID: "corr-pr-fail",
		FinalDiff:     "diff",
		PullRequest:   true,
	})
	if err == nil {
		t.Fatal("expected pull request failure")
	}
	if !errors.Is(err, ErrPullRequestFailed) {
		t.Fatalf("expected ErrPullRequestFailed, got %v", err)
	}
	if result.Outcome != OutcomeRetryable {
		t.Fatalf("expected retryable outcome, got %q", result.Outcome)
	}
	if store.unresolvedReason != "pr-create-failed" {
		t.Fatalf("expected unresolved reason pr-create-failed, got %q", store.unresolvedReason)
	}
}

func TestExecutePullRequestNoCommitsBetweenBranchesSkipsFailure(t *testing.T) {
	store := &fakeStore{}
	git := &fakeGit{commitSHA: "abc123", prErr: errors.New("gh pr create failed: GraphQL: No commits between main and main")}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-pr-no-delta",
		CorrelationID: "corr-pr-no-delta",
		FinalDiff:     "diff",
		PullRequest:   true,
	})
	if err != nil {
		t.Fatalf("expected no error for no-commit delta PR, got %v", err)
	}
	if result.Outcome != OutcomeSynced {
		t.Fatalf("expected synced outcome, got %q", result.Outcome)
	}
	if store.unresolvedReason != "" {
		t.Fatalf("expected no unresolved reason, got %q", store.unresolvedReason)
	}
	foundSkipMessage := false
	for _, journal := range store.phases {
		if strings.Contains(journal.Message, "pull request skipped") {
			foundSkipMessage = true
			break
		}
	}
	if !foundSkipMessage {
		t.Fatal("expected pull request skipped journal entry")
	}
}

func TestExecuteRetriesTransientPullRequestFailure(t *testing.T) {
	store := &fakeStore{}
	git := &fakeGit{commitSHA: "abc123", prSeq: []gitPRResult{
		{err: errors.New("service unavailable")},
		{url: "https://github.com/pr/2"},
	}}
	p := NewProtocol(store, git)
	p.retry.InitialBackoff = 0
	p.retry.MaxBackoff = 0

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-pr-retry",
		CorrelationID: "corr-pr-retry",
		FinalDiff:     "diff",
		PullRequest:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != OutcomeSynced {
		t.Fatalf("expected synced outcome, got %q", result.Outcome)
	}
	if git.prCalls != 2 {
		t.Fatalf("expected two PR attempts, got %d", git.prCalls)
	}
}

func TestExecuteCommitFailureIsRetryable(t *testing.T) {

	store := &fakeStore{}
	git := &fakeGit{commitErr: errors.New("push failed")}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-2",
		CorrelationID: "corr-2",
		FinalDiff:     "diff",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrCodeCommitFailed) {
		t.Fatalf("expected ErrCodeCommitFailed, got %v", err)
	}
	if result.Outcome != OutcomeRetryable {
		t.Fatalf("expected retryable, got %q", result.Outcome)
	}
	if store.unresolvedReason != "commit-push-failed" {
		t.Fatalf("expected unresolved reason set, got %q", store.unresolvedReason)
	}
}

func TestExecuteSyncFailureCompensates(t *testing.T) {
	store := &fakeStore{syncErr: errors.New("etcd unavailable")}
	git := &fakeGit{commitSHA: "def456"}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-3",
		CorrelationID: "corr-3",
		FinalDiff:     "diff",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrStateFinalizeFailed) {
		t.Fatalf("expected ErrStateFinalizeFailed, got %v", err)
	}
	if result.Outcome != OutcomeRetryable {
		t.Fatalf("expected retryable outcome, got %q", result.Outcome)
	}
	if git.revertCalls != 1 {
		t.Fatalf("expected one revert call, got %d", git.revertCalls)
	}
	assertHasPhase(t, store.phases, PhaseCompensationNeed)
	assertHasPhase(t, store.phases, PhaseCompensated)
	if store.unresolvedReason != "compensated-after-sync-failure" {
		t.Fatalf("unexpected unresolved reason %q", store.unresolvedReason)
	}
}

func TestExecuteSyncFailureAndRevertFailureSignalsCompensationRequired(t *testing.T) {
	store := &fakeStore{syncErr: errors.New("cas mismatch")}
	git := &fakeGit{
		commitSHA: "xyz789",
		revertErr: errors.New("revert failed"),
	}
	p := NewProtocol(store, git)

	result, err := p.Execute(context.Background(), CommitRequest{
		LoopID:        "loop-4",
		CorrelationID: "corr-4",
		FinalDiff:     "diff",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrAmbiguousTerminalGuard) {
		t.Fatalf("expected ErrAmbiguousTerminalGuard, got %v", err)
	}
	if result.Outcome != OutcomeCompensationRequired {
		t.Fatalf("expected compensation required outcome, got %q", result.Outcome)
	}
	assertHasPhase(t, store.phases, PhaseCompensationNeed)
	if hasPhase(store.phases, PhaseStateCommitted) {
		t.Fatal("state committed phase should not be set on sync failure")
	}
}

func TestExecuteRequiresLoopID(t *testing.T) {
	store := &fakeStore{}
	git := &fakeGit{commitSHA: "abc123"}
	p := NewProtocol(store, git)

	_, err := p.Execute(context.Background(), CommitRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func assertHasPhase(t *testing.T, phases []model.JournalEntry, phase Phase) {
	t.Helper()
	if !hasPhase(phases, phase) {
		t.Fatalf("expected phase %q in %+v", phase, phases)
	}
}

func hasPhase(phases []model.JournalEntry, phase Phase) bool {
	for _, record := range phases {
		if record.Metadata != nil && record.Metadata["completion_phase"] == string(phase) {
			return true
		}
	}
	return false
}

type fakeStore struct {
	phases           []model.JournalEntry
	syncErr          error
	unresolvedReason string
}

func (f *fakeStore) RecordPhase(_ context.Context, record model.JournalEntry) error {
	f.phases = append(f.phases, record)
	return nil
}

func (f *fakeStore) SetStateSynced(_ context.Context, _ string, _ string) error {
	return f.syncErr
}

func (f *fakeStore) SetStateUnresolved(_ context.Context, _ string, reason string) error {
	f.unresolvedReason = reason
	return nil
}

func (f *fakeStore) AppendJournal(_ context.Context, entry model.JournalEntry) error {
	f.phases = append(f.phases, entry)
	return nil
}

type fakeGit struct {
	commitSHA   string
	commitErr   error
	commitSeq   []gitCommitResult
	prURL       string
	prErr       error
	prSeq       []gitPRResult
	prCalls     int
	commitCalls int
	revertErr   error
	revertCalls int
}

type gitCommitResult struct {
	sha string
	err error
}

type gitPRResult struct {
	url string
	err error
}

func (f *fakeGit) CommitAndPush(_ context.Context, _ string, _ string) (string, error) {
	f.commitCalls++
	if len(f.commitSeq) > 0 {
		current := f.commitSeq[0]
		f.commitSeq = f.commitSeq[1:]
		if current.err != nil {
			return "", current.err
		}
		return current.sha, nil
	}
	if f.commitErr != nil {
		return "", f.commitErr
	}
	return f.commitSHA, nil
}

func (f *fakeGit) CreatePullRequest(_ context.Context, _ string, _ string, _ string, _ string) (string, error) {
	f.prCalls++
	if len(f.prSeq) > 0 {
		current := f.prSeq[0]
		f.prSeq = f.prSeq[1:]
		if current.err != nil {
			return "", current.err
		}
		return current.url, nil
	}
	if f.prErr != nil {
		return "", f.prErr
	}
	return f.prURL, nil
}

func (f *fakeGit) Revert(_ context.Context, _ string, _ string) error {

	f.revertCalls++
	return f.revertErr
}
