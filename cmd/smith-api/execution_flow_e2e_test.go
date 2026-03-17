package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smith/internal/source/model"
	"smith/internal/source/store"
	api "smith/pkg/api/v1"
)

func TestExecutionFlowHappyPathRecordsInterventionReplayability(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)
	seedApprovedTask(t, ms, "task-happy")

	loopID := createLoopFromTask(t, s, "task-happy")
	resumeLoop(t, s, loopID, "resume-for-execution")

	firstIntervention := postIntervention(t, s, loopID, "evt-happy-1", "do not modify auth routes")
	assert.False(t, firstIntervention.Idempotent)
	secondIntervention := postIntervention(t, s, loopID, "evt-happy-1", "do not modify auth routes")
	assert.True(t, secondIntervention.Idempotent)
	assert.Equal(t, firstIntervention.Sequence, secondIntervention.Sequence)

	overrideLoopState(t, s, loopID, model.LoopStateSynced, "completion-saga-succeeded")

	state, found, err := ms.GetState(context.Background(), loopID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.LoopStateSynced, state.Record.State)

	task, found, err := ms.GetTaskContract(context.Background(), "task-happy")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskContractStatusCompleted, task.Status)

	entries, err := ms.ListJournal(context.Background(), loopID, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, countInterventionEntries(entries, "evt-happy-1"))
	assert.True(t, hasJournalReason(entries, "completion-saga-succeeded"))
}

func TestExecutionFlowFailurePathCapturesReasonAndBlocksTask(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)
	seedApprovedTask(t, ms, "task-fail")

	loopID := createLoopFromTask(t, s, "task-fail")
	resumeLoop(t, s, loopID, "resume-before-validation")
	overrideLoopState(t, s, loopID, model.LoopStateFlatline, "task-validation-failed")

	state, found, err := ms.GetState(context.Background(), loopID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.LoopStateFlatline, state.Record.State)
	assert.Equal(t, "task-validation-failed", state.Record.Reason)

	task, found, err := ms.GetTaskContract(context.Background(), "task-fail")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskContractStatusBlocked, task.Status)

	entries, err := ms.ListJournal(context.Background(), loopID, 0)
	require.NoError(t, err)
	assert.True(t, hasJournalReason(entries, "task-validation-failed"))
}

func TestExecutionFlowCancelledPathCapturesReasonAndBlocksTask(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)
	seedApprovedTask(t, ms, "task-cancel")

	loopID := createLoopFromTask(t, s, "task-cancel")
	resumeLoop(t, s, loopID, "resume-before-cancel")

	cancelRec := httptest.NewRecorder()
	cancelReq := httptest.NewRequest(http.MethodPost, "/api/loops/"+loopID+"/cancel", strings.NewReader(`{"actor":"operator","reason":"operator-requested-stop"}`))
	s.handleLoopByID(cancelRec, cancelReq)
	require.Equal(t, http.StatusOK, cancelRec.Code)

	state, found, err := ms.GetState(context.Background(), loopID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.LoopStateCancelled, state.Record.State)
	assert.Equal(t, "operator-requested-stop", state.Record.Reason)

	task, found, err := ms.GetTaskContract(context.Background(), "task-cancel")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskContractStatusBlocked, task.Status)

	entries, err := ms.ListJournal(context.Background(), loopID, 0)
	require.NoError(t, err)
	assert.True(t, hasJournalReason(entries, "operator-requested-stop"))
}

func seedApprovedTask(t *testing.T, ms *store.MemStore, id string) {
	t.Helper()
	require.NoError(t, ms.PutTaskContract(context.Background(), model.TaskContract{
		Kind:              "smith.task",
		ID:                id,
		ProjectID:         "smith",
		ProviderProfileID: "openai-work",
		Objective:         "Execution flow task",
		Validation:        []string{"go test ./..."},
		Status:            model.TaskContractStatusApproved,
		CorrelationID:     "corr-" + id,
	}))
}

func createLoopFromTask(t *testing.T, s *server, taskID string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops", strings.NewReader(`{"task_contract_id":"`+taskID+`"}`))
	s.handleLoopCreate(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created api.LoopCreateResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
	require.True(t, created.Created)
	require.NotEmpty(t, created.LoopID)
	return created.LoopID
}

func resumeLoop(t *testing.T, s *server, loopID, reason string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/loops/"+loopID+"/resume", strings.NewReader(`{"actor":"operator","reason":"`+reason+`"}`))
	s.handleLoopByID(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func overrideLoopState(t *testing.T, s *server, loopID string, target model.LoopState, reason string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/control/override", strings.NewReader(`{"loop_id":"`+loopID+`","target_state":"`+string(target)+`","reason":"`+reason+`","actor":"operator"}`))
	s.handleOverride(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func postIntervention(t *testing.T, s *server, loopID, eventID, instruction string) api.LoopInterventionResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/loops/"+loopID+"/interventions", strings.NewReader(`{"actor":"operator","instruction":"`+instruction+`","event_id":"`+eventID+`"}`))
	s.handleLoopByID(rec, req)
	require.True(t, rec.Code == http.StatusCreated || rec.Code == http.StatusOK)

	var resp api.LoopInterventionResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	return resp
}

func hasJournalReason(entries []model.JournalEntry, reason string) bool {
	for _, entry := range entries {
		if entry.Metadata["reason"] == reason {
			return true
		}
	}
	return false
}

func countInterventionEntries(entries []model.JournalEntry, eventID string) int {
	count := 0
	for _, entry := range entries {
		if entry.Metadata["intervention_event_id"] == eventID {
			count++
		}
	}
	return count
}
