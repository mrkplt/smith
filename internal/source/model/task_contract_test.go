package model

import (
	"testing"
	"time"
)

func TestIsTaskContractPatchTransitionAllowed(t *testing.T) {
	tests := []struct {
		name string
		from TaskContractStatus
		to   TaskContractStatus
		want bool
	}{
		{name: "draft to validated", from: TaskContractStatusDraft, to: TaskContractStatusValidated, want: true},
		{name: "validated to draft", from: TaskContractStatusValidated, to: TaskContractStatusDraft, want: true},
		{name: "same state", from: TaskContractStatusValidated, to: TaskContractStatusValidated, want: true},
		{name: "validated to approved disallowed", from: TaskContractStatusValidated, to: TaskContractStatusApproved, want: false},
		{name: "draft to completed disallowed", from: TaskContractStatusDraft, to: TaskContractStatusCompleted, want: false},
		{name: "approved to draft disallowed", from: TaskContractStatusApproved, to: TaskContractStatusDraft, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTaskContractPatchTransitionAllowed(tc.from, tc.to); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestApplyTaskStatusTransition(t *testing.T) {
	now := time.Date(2026, 3, 22, 3, 0, 0, 0, time.UTC)
	task := &TaskContract{
		Status:          TaskContractStatusRunning,
		TerminalOutcome: TaskTerminalOutcomeBlocked,
		TerminalReason:  "old",
		TerminalAt:      &now,
	}

	ApplyTaskStatusTransition(task, TaskContractStatusCompleted, "ignored", now.Add(time.Minute))
	if task.Status != TaskContractStatusCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}
	if task.TerminalOutcome != TaskTerminalOutcomeCompleted {
		t.Fatalf("expected completed outcome, got %s", task.TerminalOutcome)
	}
	if task.TerminalReason != "" {
		t.Fatalf("expected empty terminal reason, got %q", task.TerminalReason)
	}
	if task.TerminalAt == nil {
		t.Fatal("expected terminal_at to be set for completed status")
	}

	ApplyTaskStatusTransition(task, TaskContractStatusBlocked, "  runtime failed  ", now.Add(2*time.Minute))
	if task.TerminalOutcome != TaskTerminalOutcomeBlocked {
		t.Fatalf("expected blocked outcome, got %s", task.TerminalOutcome)
	}
	if task.TerminalReason != "runtime failed" {
		t.Fatalf("expected trimmed reason, got %q", task.TerminalReason)
	}
	if task.TerminalAt == nil {
		t.Fatal("expected terminal_at to be set for blocked status")
	}

	ApplyTaskStatusTransition(task, TaskContractStatusRunning, "ignored", now.Add(3*time.Minute))
	if task.TerminalOutcome != "" {
		t.Fatalf("expected cleared terminal outcome, got %s", task.TerminalOutcome)
	}
	if task.TerminalReason != "" {
		t.Fatalf("expected cleared terminal reason, got %q", task.TerminalReason)
	}
	if task.TerminalAt != nil {
		t.Fatal("expected terminal_at to clear on active status")
	}
}
