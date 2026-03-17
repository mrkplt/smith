package model

import "time"

type TaskContractStatus string

const (
	TaskContractStatusDraft     TaskContractStatus = "draft"
	TaskContractStatusValidated TaskContractStatus = "validated"
	TaskContractStatusApproved  TaskContractStatus = "approved"
	TaskContractStatusRunning   TaskContractStatus = "running"
	TaskContractStatusCompleted TaskContractStatus = "completed"
	TaskContractStatusBlocked   TaskContractStatus = "blocked"
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
	Metadata           map[string]string  `json:"metadata,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	CorrelationID      string             `json:"correlation_id"`
	SchemaVersion      string             `json:"schema_version"`
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
