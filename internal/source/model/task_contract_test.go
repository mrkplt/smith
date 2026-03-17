package model

import "testing"

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
