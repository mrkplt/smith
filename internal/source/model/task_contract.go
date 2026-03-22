package model

import (
	"strings"
	"time"
)

type TaskContractStatus string
type TaskTerminalOutcome string

const (
	TaskContractStatusDraft     TaskContractStatus = "draft"
	TaskContractStatusValidated TaskContractStatus = "validated"
	TaskContractStatusApproved  TaskContractStatus = "approved"
	TaskContractStatusRunning   TaskContractStatus = "running"
	TaskContractStatusCompleted TaskContractStatus = "completed"
	TaskContractStatusBlocked   TaskContractStatus = "blocked"

	TaskTerminalOutcomeCompleted TaskTerminalOutcome = "completed"
	TaskTerminalOutcomeBlocked   TaskTerminalOutcome = "blocked"
)

type TaskContract struct {
	Kind               string             `json:"kind"`
	ID                 string             `json:"id"`
	ProjectID          string             `json:"project_id"`
	ProviderProfileID  string             `json:"provider_profile_id"`
	SourceDocument     string             `json:"source_document,omitempty"`
	Objective          string             `json:"objective"`
	Constraints        []string           `json:"constraints,omitempty"`
	AcceptanceCriteria []string           `json:"acceptance_criteria,omitempty"`
	Validation         []string           `json:"validation,omitempty"`
	Status             TaskContractStatus `json:"status"`
	TerminalOutcome    TaskTerminalOutcome `json:"terminal_outcome,omitempty"`
	TerminalReason     string             `json:"terminal_reason,omitempty"`
	TerminalAt         *time.Time         `json:"terminal_at,omitempty"`
	Metadata           map[string]string  `json:"metadata,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	CorrelationID      string             `json:"correlation_id"`
	SchemaVersion      string             `json:"schema_version"`
}

func IsTaskTerminalOutcome(outcome TaskTerminalOutcome) bool {
	switch outcome {
	case TaskTerminalOutcomeCompleted, TaskTerminalOutcomeBlocked:
		return true
	default:
		return false
	}
}

func ApplyTaskStatusTransition(task *TaskContract, nextStatus TaskContractStatus, terminalReason string, now time.Time) {
	if task == nil {
		return
	}
	task.Status = nextStatus
	switch nextStatus {
	case TaskContractStatusCompleted:
		at := now.UTC()
		task.TerminalOutcome = TaskTerminalOutcomeCompleted
		task.TerminalReason = ""
		task.TerminalAt = &at
	case TaskContractStatusBlocked:
		at := now.UTC()
		task.TerminalOutcome = TaskTerminalOutcomeBlocked
		task.TerminalReason = strings.TrimSpace(terminalReason)
		task.TerminalAt = &at
	default:
		task.TerminalOutcome = ""
		task.TerminalReason = ""
		task.TerminalAt = nil
	}
}

func IsTaskContractStatus(status TaskContractStatus) bool {
	switch status {
	case TaskContractStatusDraft,
		TaskContractStatusValidated,
		TaskContractStatusApproved,
		TaskContractStatusRunning,
		TaskContractStatusCompleted,
		TaskContractStatusBlocked:
		return true
	default:
		return false
	}
}

func IsTaskContractPatchTransitionAllowed(from, to TaskContractStatus) bool {
	if from == to {
		return true
	}
	if !IsTaskContractStatus(from) || !IsTaskContractStatus(to) {
		return false
	}
	if to == TaskContractStatusApproved {
		return false
	}
	switch from {
	case TaskContractStatusDraft:
		return to == TaskContractStatusValidated
	case TaskContractStatusValidated:
		return to == TaskContractStatusDraft
	default:
		return false
	}
}
