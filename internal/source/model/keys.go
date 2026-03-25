package model

import "fmt"

const (
	PrefixAnomalies           = "/smith/v1/anomalies"
	PrefixState               = "/smith/v1/state"
	PrefixJournal             = "/smith/v1/journal"
	PrefixHandoffs            = "/smith/v1/handoffs"
	PrefixLocks               = "/smith/v1/locks"
	PrefixOverrides           = "/smith/v1/overrides"
	PrefixAudit               = "/smith/v1/audit"
	PrefixDocuments           = "/smith/v1/documents"
	PrefixTasks               = "/smith/v1/tasks"
	PrefixProviderCredentials = "/smith/v1/provider_credentials"
)

func AnomalyKey(loopID string) string {
	return fmt.Sprintf("%s/%s", PrefixAnomalies, loopID)
}

func StateKey(loopID string) string {
	return fmt.Sprintf("%s/%s", PrefixState, loopID)
}

func DocumentKey(documentID string) string {
	return fmt.Sprintf("%s/%s", PrefixDocuments, documentID)
}

func TaskContractKey(taskID string) string {
	return fmt.Sprintf("%s/%s", PrefixTasks, taskID)
}

func JournalPrefix(loopID string) string {
	return fmt.Sprintf("%s/%s", PrefixJournal, loopID)
}

func JournalKey(loopID string, sequence int64) string {
	return fmt.Sprintf("%s/%020d", JournalPrefix(loopID), sequence)
}

func HandoffPrefix(loopID string) string {
	return fmt.Sprintf("%s/%s", PrefixHandoffs, loopID)
}

func HandoffKey(loopID string, sequence int64) string {
	return fmt.Sprintf("%s/%020d", HandoffPrefix(loopID), sequence)
}

func LockKey(loopID string) string {
	return fmt.Sprintf("%s/%s", PrefixLocks, loopID)
}

func OverridePrefix(loopID string) string {
	return fmt.Sprintf("%s/%s", PrefixOverrides, loopID)
}

func OverrideKey(loopID string, sequence int64) string {
	return fmt.Sprintf("%s/%020d", OverridePrefix(loopID), sequence)
}

func ProviderCredentialKey(providerID string) string {
	return fmt.Sprintf("%s/%s", PrefixProviderCredentials, providerID)
}
