package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"smith/internal/source/model"
	"smith/internal/source/provider"
	"smith/internal/source/store"
	api "smith/pkg/api/v1"
	pb "smith/proto/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIngressSummary(t *testing.T) {
	summary := newIngressSummary([]ingressResult{
		{Status: "unresolved", Created: true},
		{Status: "error", Created: false},
		{Status: "unresolved", Created: false},
	})
	if len(summary.Results) != 3 || summary.Summary.Created != 1 || summary.Summary.Existing != 1 || summary.Summary.Errors != 1 {
		t.Fatalf("unexpected summary: %#v", summary.Summary)
	}
}

func TestHandleIngressPRDAcceptsCanonicalPRD(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/ingress/prd", strings.NewReader(`{
		"format":"json",
		"source_ref":"docs/prd.json",
		"prd":{
			"version":1,
			"project":"Validation",
			"overview":"Canonical PRD validation",
			"qualityGates":["go test ./..."],
			"stories":[
				{
					"id":"US-001",
					"title":"Define validation contract",
					"status":"open",
					"description":"As a maintainer, I want shared validation.",
					"acceptanceCriteria":["Validation report is shared."]
				}
			]
		}
	}`))
	s.handleIngressPRD(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	states, err := ms.ListStates(context.Background())
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected one created loop, got %d", len(states))
	}
	anomaly, found, err := ms.GetAnomaly(context.Background(), states[0].Record.LoopID)
	if err != nil {
		t.Fatalf("get anomaly: %v", err)
	}
	if !found {
		t.Fatal("expected anomaly for created loop")
	}
	if anomaly.SourceType != "prd_story" || anomaly.SourceRef != "docs/prd.json#US-001" {
		t.Fatalf("unexpected created anomaly: %+v", anomaly)
	}
	if anomaly.Metadata["prd_story_id"] != "US-001" {
		t.Fatalf("expected prd_story_id metadata, got %#v", anomaly.Metadata)
	}
	taskID := strings.TrimSpace(anomaly.Metadata["task_contract_id"])
	if taskID == "" {
		t.Fatalf("expected task_contract_id metadata, got %#v", anomaly.Metadata)
	}
	task, found, err := ms.GetTaskContract(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !found {
		t.Fatalf("expected task contract %q", taskID)
	}
	if task.Status != model.TaskContractStatusRunning {
		t.Fatalf("expected running task contract, got %q", task.Status)
	}
}

func TestHandleIngressPRDPropagatesBindingMetadata(t *testing.T) {
	ms := store.NewMemStore()
	projectStore := &memoryProjectStore{items: map[string]provider.Project{
		"smith": {
			ID:                "smith",
			Name:              "Smith",
			RepoURL:           "https://github.com/callmeradical/smith",
			ProviderProfileID: "codex-default",
			SkillsImage:       "ghcr.io/callmeradical/smith-skills:v1",
			SkillsPullPolicy:  "IfNotPresent",
		},
	}}
	s := newPRDValidationTestServer(ms)
	s.projectStore = projectStore

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/ingress/prd", strings.NewReader(`{
		"format":"json",
		"source_ref":"docs/prd.json",
		"metadata":{
			"project_id":"smith"
		},
		"prd":{
			"version":1,
			"project":"Validation",
			"overview":"Canonical PRD validation",
			"qualityGates":["go test ./..."],
			"stories":[
				{
					"id":"US-001",
					"title":"Define validation contract",
					"status":"open",
					"description":"As a maintainer, I want shared validation.",
					"acceptanceCriteria":["Validation report is shared."]
				}
			]
		}
	}`))
	s.handleIngressPRD(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	states, err := ms.ListStates(context.Background())
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected one created loop, got %d", len(states))
	}
	anomaly, found, err := ms.GetAnomaly(context.Background(), states[0].Record.LoopID)
	if err != nil {
		t.Fatalf("get anomaly: %v", err)
	}
	if !found {
		t.Fatal("expected anomaly for created loop")
	}
	if anomaly.Metadata["project_id"] != "smith" {
		t.Fatalf("expected project_id metadata, got %#v", anomaly.Metadata)
	}
	if anomaly.Metadata["provider_profile_id"] != "codex-default" {
		t.Fatalf("expected provider_profile_id metadata, got %#v", anomaly.Metadata)
	}
	if anomaly.Metadata["github_repository"] != "https://github.com/callmeradical/smith" {
		t.Fatalf("expected github_repository metadata, got %#v", anomaly.Metadata)
	}
	if anomaly.Metadata["workspace_seed_image"] != "ghcr.io/callmeradical/smith-skills:v1" {
		t.Fatalf("expected workspace_seed_image metadata, got %#v", anomaly.Metadata)
	}
	if anomaly.Metadata["workspace_seed_pull_policy"] != "IfNotPresent" {
		t.Fatalf("expected workspace_seed_pull_policy metadata, got %#v", anomaly.Metadata)
	}
}

func TestHandleLoopsIncludesDisplayTitleAndProgress(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)

	state := model.State{
		LoopID:        "loop-progress",
		State:         model.LoopStateRunning,
		Attempt:       2,
		CorrelationID: "corr-progress",
		SchemaVersion: "v1",
		UpdatedAt:     time.Now().UTC(),
	}
	if _, err := ms.PutState(context.Background(), state, 0); err != nil {
		t.Fatalf("put state: %v", err)
	}
	if err := ms.PutAnomaly(context.Background(), model.Anomaly{
		ID:         state.LoopID,
		Title:      "Implement workflow",
		SourceType: "prd_story",
		SourceRef:  "prd:demo#US-002",
		Policy: model.LoopPolicy{
			MaxAttempts: 5,
		},
		Metadata: map[string]string{
			"prd_story_id": "US-002",
		},
		CorrelationID: "corr-progress",
		SchemaVersion: "v1",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("put anomaly: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/loops", nil)
	s.handleLoops(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload []api.LoopWithRevision
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected one loop, got %d", len(payload))
	}
	if payload[0].Record.DisplayTitle != "US-002: Implement workflow" {
		t.Fatalf("unexpected display_title: %q", payload[0].Record.DisplayTitle)
	}
	if payload[0].Record.CurrentCount != 2 || payload[0].Record.TargetCount != 5 {
		t.Fatalf("unexpected progress counts: current=%d target=%d", payload[0].Record.CurrentCount, payload[0].Record.TargetCount)
	}
}

func TestHandleLoopByIDIncludesDisplayTitleAndProgress(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)

	state := model.State{
		LoopID:        "loop-detail-progress",
		State:         model.LoopStateRunning,
		Attempt:       3,
		CorrelationID: "corr-detail",
		SchemaVersion: "v1",
		UpdatedAt:     time.Now().UTC(),
	}
	if _, err := ms.PutState(context.Background(), state, 0); err != nil {
		t.Fatalf("put state: %v", err)
	}
	if err := ms.PutAnomaly(context.Background(), model.Anomaly{
		ID:         state.LoopID,
		Title:      "Fallback Title",
		SourceType: "github_issue",
		SourceRef:  "callmeradical/smith#123",
		Policy: model.LoopPolicy{
			MaxAttempts: 1,
		},
		Metadata: map[string]string{
			"display_title": "Issue #123: Harden completion",
		},
		CorrelationID: "corr-detail",
		SchemaVersion: "v1",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("put anomaly: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/loops/loop-detail-progress", nil)
	s.handleLoopByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload api.LoopResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.State.DisplayTitle != "Issue #123: Harden completion" {
		t.Fatalf("unexpected display_title: %q", payload.State.DisplayTitle)
	}
	if payload.State.CurrentCount != 3 || payload.State.TargetCount != 3 {
		t.Fatalf("unexpected progress counts: current=%d target=%d", payload.State.CurrentCount, payload.State.TargetCount)
	}
}

func TestHandleLoopCleanupByStateSelector(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)

	now := time.Now().UTC()
	_, _ = ms.PutState(context.Background(), model.State{LoopID: "loop-flatline", State: model.LoopStateFlatline, CorrelationID: "corr-flatline", SchemaVersion: "v1", UpdatedAt: now}, 0)
	_, _ = ms.PutState(context.Background(), model.State{LoopID: "loop-synced", State: model.LoopStateSynced, CorrelationID: "corr-synced", SchemaVersion: "v1", UpdatedAt: now}, 0)
	_, _ = ms.PutState(context.Background(), model.State{LoopID: "loop-running", State: model.LoopStateRunning, CorrelationID: "corr-running", SchemaVersion: "v1", UpdatedAt: now}, 0)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/cleanup", strings.NewReader(`{"actor":"alice","states":["flatline","synced"]}`))
	s.handleLoopCleanup(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload api.LoopCleanupResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&payload))

	assert.Equal(t, "alice", payload.Actor)
	assert.Equal(t, 2, payload.MatchedCount)
	assert.Equal(t, 2, payload.DeletedCount)
	assert.ElementsMatch(t, []string{"loop-flatline", "loop-synced"}, payload.Deleted)
	assert.Empty(t, payload.SkippedActive)

	_, foundFlatline, err := ms.GetState(context.Background(), "loop-flatline")
	require.NoError(t, err)
	assert.False(t, foundFlatline)
	_, foundSynced, err := ms.GetState(context.Background(), "loop-synced")
	require.NoError(t, err)
	assert.False(t, foundSynced)
	_, foundRunning, err := ms.GetState(context.Background(), "loop-running")
	require.NoError(t, err)
	assert.True(t, foundRunning)
}

func TestHandleLoopCleanupByLoopIDsReportsSkippedAndMissing(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)

	now := time.Now().UTC()
	_, _ = ms.PutState(context.Background(), model.State{LoopID: "loop-flatline", State: model.LoopStateFlatline, CorrelationID: "corr-flatline", SchemaVersion: "v1", UpdatedAt: now}, 0)
	_, _ = ms.PutState(context.Background(), model.State{LoopID: "loop-running", State: model.LoopStateRunning, CorrelationID: "corr-running", SchemaVersion: "v1", UpdatedAt: now}, 0)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/cleanup", strings.NewReader(`{"loop_ids":["loop-flatline","loop-running","loop-missing"]}`))
	s.handleLoopCleanup(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload api.LoopCleanupResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&payload))

	assert.Equal(t, "operator", payload.Actor)
	assert.Equal(t, 2, payload.MatchedCount)
	assert.Equal(t, 1, payload.DeletedCount)
	assert.Equal(t, []string{"loop-flatline"}, payload.Deleted)
	assert.Equal(t, []string{"loop-running"}, payload.SkippedActive)
	assert.Equal(t, []string{"loop-missing"}, payload.NotFound)

	_, foundFlatline, err := ms.GetState(context.Background(), "loop-flatline")
	require.NoError(t, err)
	assert.False(t, foundFlatline)
	_, foundRunning, err := ms.GetState(context.Background(), "loop-running")
	require.NoError(t, err)
	assert.True(t, foundRunning)
}

func TestHandleIngressPRDRejectsUnknownProjectMetadata(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)
	s.projectStore = &memoryProjectStore{items: map[string]provider.Project{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/ingress/prd", strings.NewReader(`{
		"format":"json",
		"source_ref":"docs/prd.json",
		"metadata":{"project_id":"missing"},
		"prd":{
			"version":1,
			"project":"Validation",
			"overview":"Canonical PRD validation",
			"qualityGates":["go test ./..."],
			"stories":[
				{
					"id":"US-001",
					"title":"Define validation contract",
					"status":"open",
					"description":"As a maintainer, I want shared validation.",
					"acceptanceCriteria":["Validation report is shared."]
				}
			]
		}
	}`))
	s.handleIngressPRD(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 summary response, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Results []struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"results"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Results) != 1 || body.Results[0].Status != "error" || !strings.Contains(body.Results[0].Message, "project not found") {
		t.Fatalf("expected project-not-found ingress error result, got %+v", body.Results)
	}
}

func TestHandleIngressPRDRejectsInvalidCanonicalPRD(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/ingress/prd", strings.NewReader(`{
		"format":"json",
		"source_ref":"docs/prd.json",
		"prd":{
			"version":1,
			"project":"Validation",
			"overview":"Canonical PRD validation",
			"qualityGates":[],
			"stories":[
				{
					"id":"US-001",
					"title":"Oversized story",
					"status":"open",
					"description":"As an operator, I want a story that packs too much into one iteration.",
					"acceptanceCriteria":["one","two","three","four","five","six"]
				}
			]
		}
	}`))
	s.handleIngressPRD(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error  string                    `json:"error"`
		Report model.PRDValidationReport `json:"report"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error == "" || body.Report.Valid {
		t.Fatalf("expected validation failure payload, got %+v", body)
	}
	assertDiagnosticCodeInReport(t, body.Report.Errors, model.PRDDiagnosticMissingQualityGates)
	assertDiagnosticCodeInReport(t, body.Report.Warnings, model.PRDDiagnosticOversizedStory)
	states, err := ms.ListStates(context.Background())
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("expected no created loops, got %d", len(states))
	}
}

func TestHandlePRDValidateMarkdownReturnsDiagnostics(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/prd/validate", strings.NewReader(`{
		"format":"markdown",
		"markdown":"# Validation\n\n## Overview\n\nCanonical PRD validation"
	}`))
	s.handlePRDValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body api.PRDValidateResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Format != "markdown" {
		t.Fatalf("expected markdown format, got %q", body.Format)
	}
	if body.Report.Valid {
		t.Fatalf("expected invalid report, got %+v", body.Report)
	}
	assertDiagnosticCodeInReport(t, body.Report.Errors, model.PRDDiagnosticMissingQualityGates)
}

func TestHandlePRDValidateJSONReturnsCanonicalMarkdown(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/prd/validate", strings.NewReader(`{
		"format":"json",
		"json":"{\"version\":1,\"project\":\"Validation\",\"overview\":\"Canonical PRD validation\",\"qualityGates\":[\"go test ./...\"],\"stories\":[{\"id\":\"US-001\",\"title\":\"Define validation contract\",\"status\":\"open\",\"description\":\"As a maintainer, I want shared validation.\",\"acceptanceCriteria\":[\"Validation report is shared.\"]}]}"
	}`))
	s.handlePRDValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body api.PRDValidateResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Format != "json" {
		t.Fatalf("expected json format, got %q", body.Format)
	}
	if !body.Report.Valid {
		t.Fatalf("expected valid report, got %+v", body.Report)
	}
	if !strings.Contains(body.CanonicalMarkdown, "# Validation") {
		t.Fatalf("expected canonical markdown in response, got %q", body.CanonicalMarkdown)
	}
}

func TestHandleLoopCreateRejectsInvalidWorkspacePRD(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops", strings.NewReader(`{
		"title":"Loop from supplied PRD",
		"description":"Loop request from supplied PRD JSON",
		"source_type":"prompt",
		"source_ref":"prompt:smith",
		"metadata":{
			"workspace_prd_json":"{\"version\":1,\"project\":\"Validation\",\"overview\":\"Canonical PRD validation\",\"qualityGates\":[],\"stories\":[]}",
			"workspace_prd_path":".agents/tasks/prd.json"
		}
	}`))
	s.handleLoopCreate(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status           string                    `json:"status"`
		ValidationReport model.PRDValidationReport `json:"validation_report"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "error" || body.ValidationReport.Valid {
		t.Fatalf("expected validation failure result, got %+v", body)
	}
	assertDiagnosticCodeInReport(t, body.ValidationReport.Errors, model.PRDDiagnosticMissingQualityGates)
	states, err := ms.ListStates(context.Background())
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("expected no created loops, got %d", len(states))
	}
}

func TestHandleLoopCreateUsesIdempotencyKeyHeader(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)
	body := `{
		"title":"Header key loop",
		"description":"Header idempotency",
		"source_type":"manual",
		"source_ref":"manual:test"
	}`

	firstRec := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/loops", strings.NewReader(body))
	firstReq.Header.Set("Idempotency-Key", "idem-header-1")
	s.handleLoopCreate(firstRec, firstReq)
	require.Equal(t, http.StatusCreated, firstRec.Code)

	var first api.LoopCreateResult
	require.NoError(t, json.NewDecoder(firstRec.Body).Decode(&first))
	require.True(t, first.Created)

	secondRec := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/v1/loops", strings.NewReader(body))
	secondReq.Header.Set("Idempotency-Key", "idem-header-1")
	s.handleLoopCreate(secondRec, secondReq)
	require.Equal(t, http.StatusOK, secondRec.Code)

	var second api.LoopCreateResult
	require.NoError(t, json.NewDecoder(secondRec.Body).Decode(&second))
	assert.False(t, second.Created)
	assert.Equal(t, first.LoopID, second.LoopID)
}

func TestHandleLoopCreateFromApprovedTaskContract(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)
	require.NoError(t, ms.PutTaskContract(context.Background(), model.TaskContract{
		Kind:              "smith.task",
		ID:                "task-approved",
		ProjectID:         "smith",
		ProviderProfileID: "openai-work",
		Objective:         "Restore green CI",
		Validation:        []string{"go test ./..."},
		Status:            model.TaskContractStatusApproved,
		CorrelationID:     "task-corr-approved",
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops", strings.NewReader(`{
		"task_contract_id":"task-approved"
	}`))
	s.handleLoopCreate(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var result api.LoopCreateResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	require.True(t, result.Created)
	require.NotEmpty(t, result.LoopID)

	anomaly, found, err := ms.GetAnomaly(context.Background(), result.LoopID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "task_contract", anomaly.SourceType)
	assert.Equal(t, "task-approved", anomaly.Metadata["task_contract_id"])
	assert.Equal(t, `["go test ./..."]`, anomaly.Metadata["task_validation_commands_json"])

	task, found, err := ms.GetTaskContract(context.Background(), "task-approved")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskContractStatusRunning, task.Status)
}

func TestHandleLoopCreateRejectsUnapprovedTaskContract(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)
	require.NoError(t, ms.PutTaskContract(context.Background(), model.TaskContract{
		Kind:              "smith.task",
		ID:                "task-draft",
		ProjectID:         "smith",
		ProviderProfileID: "openai-work",
		Objective:         "Restore green CI",
		Validation:        []string{"go test ./..."},
		Status:            model.TaskContractStatusDraft,
		CorrelationID:     "task-corr-draft",
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops", strings.NewReader(`{
		"task_contract_id":"task-draft"
	}`))
	s.handleLoopCreate(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "approved")
}

func TestHandleLoopCreateResolvesProviderFromProfileMetadata(t *testing.T) {
	ms := store.NewMemStore()
	providerStore := provider.NewFileProviderProfileStore()
	require.NoError(t, providerStore.PutProviderProfile(context.Background(), provider.ProviderProfile{
		ID:           "codex-mini-profile",
		Name:         "Codex Mini",
		ProviderType: provider.ProviderCodex,
		DefaultModel: provider.CodexMiniModel,
	}))

	s := &server{
		store:       ms,
		providers:   providerStore,
		presets:     newPresetCatalog("standard"),
		skillPolicy: model.SkillPolicy{},
		term:        newTerminalSessionStore(),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops", strings.NewReader(`{
		"title":"Resolve provider from profile",
		"description":"Loop with provider profile",
		"source_type":"manual",
		"source_ref":"manual:test",
		"metadata":{"provider_profile_id":"codex-mini-profile"}
	}`))
	s.handleLoopCreate(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var result api.LoopCreateResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))

	anomaly, found, err := ms.GetAnomaly(context.Background(), result.LoopID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, provider.ProviderCodex, anomaly.ProviderID)
	assert.Equal(t, provider.CodexMiniModel, anomaly.Model)
}

func TestHandleLoopCreateRejectsUnknownProviderProfileMetadata(t *testing.T) {
	ms := store.NewMemStore()
	s := &server{
		store:       ms,
		providers:   provider.NewFileProviderProfileStore(),
		presets:     newPresetCatalog("standard"),
		skillPolicy: model.SkillPolicy{},
		term:        newTerminalSessionStore(),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops", strings.NewReader(`{
		"title":"Unknown profile",
		"description":"Loop with unknown provider profile",
		"source_type":"manual",
		"source_ref":"manual:test",
		"metadata":{"provider_profile_id":"missing-profile"}
	}`))
	s.handleLoopCreate(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "provider profile")
}

func TestHandleDocumentBuildRejectsInvalidPRD(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)
	if err := ms.PutDocument(context.Background(), model.Document{
		ID:        "doc-1",
		ProjectID: "proj-1",
		Title:     "Invalid PRD",
		Content:   `{"version":1,"project":"Validation","overview":"Canonical PRD validation","qualityGates":[],"stories":[]}`,
		Format:    "json",
	}); err != nil {
		t.Fatalf("put document: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/documents/doc-1/build", strings.NewReader(`{}`))
	s.handleDocumentByID(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	states, err := ms.ListStates(context.Background())
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("expected no created loops, got %d", len(states))
	}
}

func TestHandleDocumentBuildCreatesTaskBackedLoops(t *testing.T) {
	ms := store.NewMemStore()
	s := newPRDValidationTestServer(ms)
	if err := ms.PutDocument(context.Background(), model.Document{
		ID:        "doc-build-1",
		ProjectID: "proj-1",
		Title:     "Valid PRD",
		Content:   `{"version":1,"project":"Validation","overview":"Canonical PRD validation","qualityGates":["go test ./..."],"stories":[{"id":"US-001","title":"Define validation contract","status":"open","description":"As a maintainer, I want shared validation.","acceptanceCriteria":["Validation report is shared."]}]}`,
		Format:    "json",
	}); err != nil {
		t.Fatalf("put document: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/documents/doc-build-1/build", strings.NewReader(`{}`))
	s.handleDocumentByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	states, err := ms.ListStates(context.Background())
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected one created loop, got %d", len(states))
	}
	anomaly, found, err := ms.GetAnomaly(context.Background(), states[0].Record.LoopID)
	if err != nil {
		t.Fatalf("get anomaly: %v", err)
	}
	if !found {
		t.Fatalf("expected anomaly for loop %q", states[0].Record.LoopID)
	}
	taskID := strings.TrimSpace(anomaly.Metadata["task_contract_id"])
	if taskID == "" {
		t.Fatalf("expected task_contract_id metadata, got %#v", anomaly.Metadata)
	}
	task, found, err := ms.GetTaskContract(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task contract: %v", err)
	}
	if !found {
		t.Fatalf("expected task contract %q", taskID)
	}
	if task.Status != model.TaskContractStatusRunning {
		t.Fatalf("expected running task contract, got %q", task.Status)
	}
}

func TestHandleLoopLifecyclePauseResumeCancelWithStateGuards(t *testing.T) {
	ms := store.NewMemStore()
	_, err := ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-lifecycle",
		State:         model.LoopStateUnresolved,
		Reason:        "seed",
		CorrelationID: "corr-loop-lifecycle",
	}, 0)
	require.NoError(t, err)

	s := &server{store: ms}

	pauseRec := httptest.NewRecorder()
	pauseReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-lifecycle/pause", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopByID(pauseRec, pauseReq)
	require.Equal(t, http.StatusOK, pauseRec.Code)
	var pauseResp map[string]any
	require.NoError(t, json.NewDecoder(pauseRec.Body).Decode(&pauseResp))
	assert.Equal(t, "unresolved", pauseResp["state"])
	assert.Equal(t, true, pauseResp["idempotent"])

	resumeRec := httptest.NewRecorder()
	resumeReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-lifecycle/resume", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopByID(resumeRec, resumeReq)
	require.Equal(t, http.StatusOK, resumeRec.Code)
	var resumeResp map[string]any
	require.NoError(t, json.NewDecoder(resumeRec.Body).Decode(&resumeResp))
	assert.Equal(t, "running", resumeResp["state"])
	assert.Equal(t, false, resumeResp["idempotent"])

	resumeAgainRec := httptest.NewRecorder()
	resumeAgainReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-lifecycle/resume", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopByID(resumeAgainRec, resumeAgainReq)
	require.Equal(t, http.StatusOK, resumeAgainRec.Code)
	var resumeAgainResp map[string]any
	require.NoError(t, json.NewDecoder(resumeAgainRec.Body).Decode(&resumeAgainResp))
	assert.Equal(t, true, resumeAgainResp["idempotent"])

	cancelRec := httptest.NewRecorder()
	cancelReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-lifecycle/cancel", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopByID(cancelRec, cancelReq)
	require.Equal(t, http.StatusOK, cancelRec.Code)

	cancelAgainRec := httptest.NewRecorder()
	cancelAgainReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-lifecycle/cancel", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopByID(cancelAgainRec, cancelAgainReq)
	require.Equal(t, http.StatusOK, cancelAgainRec.Code)
	var cancelAgainResp map[string]any
	require.NoError(t, json.NewDecoder(cancelAgainRec.Body).Decode(&cancelAgainResp))
	assert.Equal(t, true, cancelAgainResp["idempotent"])

	invalidResumeRec := httptest.NewRecorder()
	invalidResumeReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-lifecycle/resume", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopByID(invalidResumeRec, invalidResumeReq)
	require.Equal(t, http.StatusConflict, invalidResumeRec.Code)
	assert.Contains(t, invalidResumeRec.Body.String(), "invalid transition")
}

func TestHandleLoopLifecycleSyncsTaskContractStatus(t *testing.T) {
	ms := store.NewMemStore()
	require.NoError(t, ms.PutTaskContract(context.Background(), model.TaskContract{
		Kind:              "smith.task",
		ID:                "task-lifecycle",
		ProjectID:         "smith",
		ProviderProfileID: "openai-work",
		Objective:         "Lifecycle task",
		Validation:        []string{"go test ./..."},
		Status:            model.TaskContractStatusApproved,
		CorrelationID:     "task-corr-lifecycle",
	}))
	require.NoError(t, ms.PutAnomaly(context.Background(), model.Anomaly{
		ID: "loop-task-lifecycle",
		Metadata: map[string]string{
			"task_contract_id": "task-lifecycle",
		},
		CorrelationID: "corr-loop-task-lifecycle",
	}))
	_, err := ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-task-lifecycle",
		State:         model.LoopStateUnresolved,
		Reason:        "seed",
		CorrelationID: "corr-loop-task-lifecycle",
	}, 0)
	require.NoError(t, err)

	s := &server{store: ms}

	resumeRec := httptest.NewRecorder()
	resumeReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-task-lifecycle/resume", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopByID(resumeRec, resumeReq)
	require.Equal(t, http.StatusOK, resumeRec.Code)

	task, found, err := ms.GetTaskContract(context.Background(), "task-lifecycle")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskContractStatusRunning, task.Status)

	cancelRec := httptest.NewRecorder()
	cancelReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-task-lifecycle/cancel", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopByID(cancelRec, cancelReq)
	require.Equal(t, http.StatusOK, cancelRec.Code)

	task, found, err = ms.GetTaskContract(context.Background(), "task-lifecycle")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskContractStatusBlocked, task.Status)
	assert.Equal(t, model.TaskTerminalOutcomeBlocked, task.TerminalOutcome)
	assert.Equal(t, "cancelled-via-api", task.TerminalReason)
	require.NotNil(t, task.TerminalAt)

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/tasks/task-lifecycle", nil)
	s.handleTaskByID(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)
	var payload api.TaskContract
	require.NoError(t, json.NewDecoder(getRec.Body).Decode(&payload))
	assert.Equal(t, api.TaskContractStatusBlocked, payload.Status)
	assert.Equal(t, "blocked", payload.TerminalOutcome)
	assert.Equal(t, "cancelled-via-api", payload.TerminalReason)
	require.NotNil(t, payload.TerminalAt)
}

func TestHandleLoopInterventionIdempotencyByEventID(t *testing.T) {
	ms := store.NewMemStore()
	_, err := ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-intervention",
		State:         model.LoopStateRunning,
		Reason:        "seed",
		CorrelationID: "corr-loop-intervention",
	}, 0)
	require.NoError(t, err)

	s := &server{store: ms}
	firstRec := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-intervention/interventions", strings.NewReader(`{
		"actor":"alice",
		"instruction":"avoid modifying authentication logic",
		"event_id":"evt-123"
	}`))
	s.handleLoopByID(firstRec, firstReq)
	require.Equal(t, http.StatusCreated, firstRec.Code)

	var firstBody api.LoopInterventionResponse
	require.NoError(t, json.NewDecoder(firstRec.Body).Decode(&firstBody))
	assert.Equal(t, "evt-123", firstBody.EventID)
	assert.False(t, firstBody.Idempotent)
	assert.Greater(t, firstBody.Sequence, int64(0))

	secondRec := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/api/loops/loop-intervention/interventions", strings.NewReader(`{
		"actor":"alice",
		"instruction":"avoid modifying authentication logic",
		"event_id":"evt-123"
	}`))
	s.handleLoopByID(secondRec, secondReq)
	require.Equal(t, http.StatusOK, secondRec.Code)

	var secondBody api.LoopInterventionResponse
	require.NoError(t, json.NewDecoder(secondRec.Body).Decode(&secondBody))
	assert.Equal(t, int64(firstBody.Sequence), secondBody.Sequence)
	assert.True(t, secondBody.Idempotent)

	entries, err := ms.ListJournal(context.Background(), "loop-intervention", 0)
	require.NoError(t, err)
	count := 0
	for _, entry := range entries {
		if entry.Metadata["intervention_event_id"] == "evt-123" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestPresetCatalogSupportsCRUDAndPolicy(t *testing.T) {
	catalog := newPresetCatalog("team-default")
	if !catalog.Has("team-default") {
		t.Fatal("expected custom default preset to be present")
	}
	if !catalog.Has("standard") {
		t.Fatal("expected builtin standard preset to be present")
	}
	if err := catalog.Upsert("analytics"); err != nil {
		t.Fatalf("upsert preset: %v", err)
	}
	if !catalog.Has("analytics") {
		t.Fatal("expected analytics preset after upsert")
	}
	list := catalog.List()
	if len(list) < 2 {
		t.Fatalf("expected non-empty preset list, got %#v", list)
	}
	policy := catalog.Policy()
	if policy.DefaultPreset != "team-default" {
		t.Fatalf("unexpected default policy preset: %s", policy.DefaultPreset)
	}
	if _, ok := policy.AllowedPresets["analytics"]; !ok {
		t.Fatalf("expected analytics in allowed presets: %#v", policy.AllowedPresets)
	}
}

func TestMaskCredentialValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "short", in: "sk-12", want: "*****"},
		{name: "normal", in: "sk-test-123456", want: "sk-t******3456"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskCredentialValue(tc.in); got != tc.want {
				t.Fatalf("maskCredentialValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitLoopRouteSupportsSlashLoopIDs(t *testing.T) {
	tests := []struct {
		path       string
		wantLoopID string
		wantRoute  string
	}{
		{
			path:       "/v1/loops/alpha/feat-132",
			wantLoopID: "alpha/feat-132",
			wantRoute:  "",
		},
		{
			path:       "/v1/loops/alpha/feat-132/journal",
			wantLoopID: "alpha/feat-132",
			wantRoute:  "journal",
		},
		{
			path:       "/v1/loops/team/alpha/feat-132/control/attach",
			wantLoopID: "team/alpha/feat-132",
			wantRoute:  "control/attach",
		},
		{
			path:       "/v1/loops/team/alpha/feat-132/runtime",
			wantLoopID: "team/alpha/feat-132",
			wantRoute:  "runtime",
		},
		{
			path:       "/v1/loops/team/alpha/feat-132/journal/stream",
			wantLoopID: "team/alpha/feat-132",
			wantRoute:  "journal/stream",
		},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			loopID, route := splitLoopRoute(tc.path)
			if loopID != tc.wantLoopID || route != tc.wantRoute {
				t.Fatalf("splitLoopRoute(%q) = (%q, %q), want (%q, %q)", tc.path, loopID, route, tc.wantLoopID, tc.wantRoute)
			}
		})
	}
}

func TestHandleJournalStreamReplaysOrderedEntriesWithSinceSeq(t *testing.T) {
	ms := store.NewMemStore()
	s := &server{store: ms}
	loopID := "loop-stream-replay"
	for _, message := range []string{"entry-one", "entry-two", "entry-three"} {
		err := ms.AppendJournal(context.Background(), model.JournalEntry{
			LoopID:  loopID,
			Message: message,
		})
		require.NoError(t, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/loops/"+loopID+"/journal/stream?since_seq=1", nil).WithContext(ctx)
	s.handleLoopByID(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "event: ready")
	assert.Contains(t, body, "event: entry")
	assert.NotContains(t, body, "entry-one")
	assert.Contains(t, body, "entry-two")
	assert.Contains(t, body, "entry-three")
	assert.Greater(t, strings.Index(body, "entry-three"), strings.Index(body, "entry-two"))
}

func TestResolveLoopRuntimeRunningPod(t *testing.T) {
	now := time.Now().UTC()
	reader := &fakeRuntimePodReader{
		podsByJob: map[string][]corev1.Pod{
			"smith-replica-loop-a-12345": {
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "smith-replica-loop-a-12345-abc",
						CreationTimestamp: metav1.NewTime(now),
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "replica"}},
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				},
			},
		},
	}
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: reader,
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-a", model.StateRecord{
		LoopID:        "loop-a",
		State:         model.LoopStateRunning,
		WorkerJobName: "smith-replica-loop-a-12345",
	})

	if !got.Attachable {
		t.Fatalf("expected attachable true, got false with reason %q", got.Reason)
	}
	if got.Namespace != "smith-system" || got.PodName != "smith-replica-loop-a-12345-abc" || got.ContainerName != "replica" {
		t.Fatalf("unexpected runtime target: %+v", got)
	}
	if got.Reason != "" {
		t.Fatalf("expected empty reason for attachable runtime, got %q", got.Reason)
	}
}

func TestResolveLoopRuntimePendingPod(t *testing.T) {
	reader := &fakeRuntimePodReader{
		podsByJob: map[string][]corev1.Pod{
			"smith-replica-loop-b-12345": {
				{
					ObjectMeta: metav1.ObjectMeta{Name: "smith-replica-loop-b-12345-def"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "replica"}},
					},
					Status: corev1.PodStatus{Phase: corev1.PodPending},
				},
			},
		},
	}
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: reader,
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-b", model.StateRecord{
		LoopID:        "loop-b",
		State:         model.LoopStateUnresolved,
		WorkerJobName: "smith-replica-loop-b-12345",
	})

	if got.Attachable {
		t.Fatalf("expected attachable false for pending pod, got true")
	}
	if got.Reason != "runtime pod not running" {
		t.Fatalf("expected reason runtime pod not running, got %q", got.Reason)
	}
	if got.PodPhase != string(corev1.PodPending) {
		t.Fatalf("expected pod phase Pending, got %q", got.PodPhase)
	}
}

func TestResolveLoopRuntimeTerminalLoop(t *testing.T) {
	reader := &fakeRuntimePodReader{}
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: reader,
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-c", model.StateRecord{
		LoopID: "loop-c",
		State:  model.LoopStateSynced,
	})

	if got.Attachable {
		t.Fatalf("expected attachable false for terminal loop, got true")
	}
	if got.Reason != "loop not active" {
		t.Fatalf("expected reason loop not active, got %q", got.Reason)
	}
	if reader.calls != 0 {
		t.Fatalf("expected no runtime pod lookup for terminal loop, got %d calls", reader.calls)
	}
}

func TestResolveLoopRuntimeMissingPod(t *testing.T) {
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: &fakeRuntimePodReader{podsByJob: map[string][]corev1.Pod{}},
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-d", model.StateRecord{
		LoopID:        "loop-d",
		State:         model.LoopStateRunning,
		WorkerJobName: "smith-replica-loop-d-12345",
	})

	if got.Attachable {
		t.Fatalf("expected attachable false when pod is missing, got true")
	}
	if got.Reason != "runtime pod not found" {
		t.Fatalf("expected reason runtime pod not found, got %q", got.Reason)
	}
}

func TestResolveLoopRuntimeFallsBackToFirstContainer(t *testing.T) {
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		runtimePods: &fakeRuntimePodReader{
			podsByJob: map[string][]corev1.Pod{
				"smith-replica-loop-e-12345": {
					{
						ObjectMeta: metav1.ObjectMeta{Name: "smith-replica-loop-e-12345-abc"},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "worker"}},
						},
						Status: corev1.PodStatus{Phase: corev1.PodRunning},
					},
				},
			},
		},
	}

	got := s.resolveLoopRuntime(context.Background(), "loop-e", model.StateRecord{
		LoopID:        "loop-e",
		State:         model.LoopStateRunning,
		WorkerJobName: "smith-replica-loop-e-12345",
	})

	if !got.Attachable {
		t.Fatalf("expected attachable true when fallback container exists, got false with reason %q", got.Reason)
	}
	if got.ContainerName != "worker" {
		t.Fatalf("expected fallback container worker, got %q", got.ContainerName)
	}
}

func TestHandleLoopAttachRejectsNonRunningRuntime(t *testing.T) {
	ms := store.NewMemStore()
	ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-pending",
		State:         model.LoopStateRunning,
		WorkerJobName: "smith-replica-loop-pending-12345",
	}, 0)
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		term:  newTerminalSessionStore(),
		store: ms,
		runtimePods: &fakeRuntimePodReader{
			podsByJob: map[string][]corev1.Pod{
				"smith-replica-loop-pending-12345": {
					{
						ObjectMeta: metav1.ObjectMeta{Name: "smith-replica-loop-pending-12345-abc"},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "replica"}},
						},
						Status: corev1.PodStatus{Phase: corev1.PodPending},
					},
				},
			},
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-pending/control/attach", strings.NewReader(`{"actor":"alice","terminal":"console-pods"}`))
	s.handleLoopAttach(rec, req, "loop-pending")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for non-running runtime pod, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "runtime pod not running" {
		t.Fatalf("expected runtime pod not running error, got %q", body["error"])
	}
	if s.term.IsAttached("loop-pending", "alice") {
		t.Fatal("expected no terminal session to be created for non-running runtime pod")
	}
}
func TestHandleLoopAttachRejectsUnauthorizedBeforeRuntimeResolution(t *testing.T) {
	ms := store.NewMemStore()
	runtimeReader := &fakeRuntimePodReader{}
	s := &server{
		cfg: config{
			operatorToken: "secret-token",
		},
		term:        newTerminalSessionStore(),
		store:       ms,
		runtimePods: runtimeReader,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-auth/control/attach", strings.NewReader(`{"actor":"alice","terminal":"console-pods"}`))
	s.handleLoopAttach(rec, req, "loop-auth")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized attach, got %d body=%s", rec.Code, rec.Body.String())
	}
	if runtimeReader.calls != 0 {
		t.Fatalf("expected no runtime resolution for unauthorized attach, got %d calls", runtimeReader.calls)
	}
	audits, _ := ms.ListAudit(context.Background(), "", 0)
	if len(audits) != 1 {
		t.Fatalf("expected one rejected attach audit entry, got %d", len(audits))
	}
	if audits[0].Action != "attach-terminal-rejected" {
		t.Fatalf("expected attach-terminal-rejected action, got %q", audits[0].Action)
	}
	if audits[0].Metadata["request_status"] != "rejected" {
		t.Fatalf("expected rejected status in audit metadata, got %q", audits[0].Metadata["request_status"])
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["code"] != terminalErrUnauthorized {
		t.Fatalf("expected unauthorized code %q, got %q", terminalErrUnauthorized, body["code"])
	}
}
func TestHandleLoopAttachDetachIncludeRuntimeMetadata(t *testing.T) {
	ms := store.NewMemStore()
	ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-running",
		State:         model.LoopStateRunning,
		WorkerJobName: "smith-replica-loop-running-12345",
		CorrelationID: "corr-attach-detach",
	}, 0)
	s := &server{
		cfg: config{
			runtimeNamespace:     "smith-system",
			runtimeContainerName: "replica",
		},
		term:  newTerminalSessionStore(),
		store: ms,
		runtimePods: &fakeRuntimePodReader{
			podsByJob: map[string][]corev1.Pod{
				"smith-replica-loop-running-12345": {
					{
						ObjectMeta: metav1.ObjectMeta{Name: "smith-replica-loop-running-12345-abc"},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "replica"}},
						},
						Status: corev1.PodStatus{Phase: corev1.PodRunning},
					},
				},
			},
		},
	}

	attachBody := strings.NewReader(`{"actor":"alice","terminal":"console-pods"}`)
	recAttach1 := httptest.NewRecorder()
	reqAttach1 := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-running/control/attach", attachBody)
	s.handleLoopAttach(recAttach1, reqAttach1, "loop-running")
	if recAttach1.Code != http.StatusOK {
		t.Fatalf("expected first attach success, got %d body=%s", recAttach1.Code, recAttach1.Body.String())
	}

	recAttach2 := httptest.NewRecorder()
	reqAttach2 := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-running/control/attach", strings.NewReader(`{"actor":"alice","terminal":"console-pods"}`))
	s.handleLoopAttach(recAttach2, reqAttach2, "loop-running")
	if recAttach2.Code != http.StatusOK {
		t.Fatalf("expected second attach success, got %d body=%s", recAttach2.Code, recAttach2.Body.String())
	}
	var attachResp map[string]any
	if err := json.NewDecoder(recAttach2.Body).Decode(&attachResp); err != nil {
		t.Fatalf("decode second attach response: %v", err)
	}
	if int(attachResp["attach_count"].(float64)) != 2 {
		t.Fatalf("expected actor attach_count to increment to 2, got %#v", attachResp["attach_count"])
	}

	recDetach := httptest.NewRecorder()
	reqDetach := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-running/control/detach", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopDetach(recDetach, reqDetach, "loop-running")
	if recDetach.Code != http.StatusOK {
		t.Fatalf("expected detach success, got %d body=%s", recDetach.Code, recDetach.Body.String())
	}
	if s.term.IsAttached("loop-running", "alice") {
		t.Fatal("expected actor to be detached")
	}

	allAudits, _ := ms.ListAudit(context.Background(), "", 0)
	// Reverse audits to match original test expectations (oldest first)
	audits := make([]store.AuditRecord, len(allAudits))
	for i := range allAudits {
		audits[i] = allAudits[len(allAudits)-1-i]
	}
	journals, _ := ms.ListJournal(context.Background(), "loop-running", 0)

	if len(audits) != 3 {
		t.Fatalf("expected 3 audit records (2 attach + 1 detach), got %d", len(audits))
	}
	if len(journals) != 3 {
		t.Fatalf("expected 3 journal records (2 attach + 1 detach), got %d", len(journals))
	}

	lastAttachAudit := audits[1]
	if lastAttachAudit.Action != "attach-terminal" {
		t.Fatalf("expected attach-terminal action, got %q", lastAttachAudit.Action)
	}
	assertTerminalMetadata(t, lastAttachAudit.Metadata, "alice", "console-pods", "smith-system/smith-replica-loop-running-12345-abc:replica")
	if lastAttachAudit.Metadata["attach_count"] != "2" {
		t.Fatalf("expected attach_count=2 in audit metadata, got %q", lastAttachAudit.Metadata["attach_count"])
	}
	if lastAttachAudit.Metadata["request_status"] != "accepted" {
		t.Fatalf("expected request_status=accepted in attach metadata, got %q", lastAttachAudit.Metadata["request_status"])
	}

	detachAudit := audits[2]
	if detachAudit.Action != "detach-terminal" {
		t.Fatalf("expected detach-terminal action, got %q", detachAudit.Action)
	}
	assertTerminalMetadata(t, detachAudit.Metadata, "alice", "console-pods", "smith-system/smith-replica-loop-running-12345-abc:replica")
	if detachAudit.Metadata["request_status"] != "accepted" {
		t.Fatalf("expected request_status=accepted in detach metadata, got %q", detachAudit.Metadata["request_status"])
	}

	detachJournal := journals[2]
	if detachJournal.Message != "terminal detached" {
		t.Fatalf("expected detach journal message, got %q", detachJournal.Message)
	}
	assertTerminalMetadata(t, detachJournal.Metadata, "alice", "console-pods", "smith-system/smith-replica-loop-running-12345-abc:replica")
}

func TestHandleLoopDetachOnlyRemovesTargetActor(t *testing.T) {
	ms := store.NewMemStore()
	_, _ = ms.PutState(context.Background(), model.StateRecord{
		LoopID: "loop-actor",
		State:  model.LoopStateRunning,
	}, 0)
	s := &server{
		term:  newTerminalSessionStore(),
		store: ms,
	}
	runtime := loopRuntimeResponse{Namespace: "smith-system",
		PodName:       "smith-replica-loop-actor-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-actor", "alice", "console-pods", runtime)
	s.term.Attach("loop-actor", "bob", "console-pods", runtime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-actor/control/detach", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopDetach(rec, req, "loop-actor")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected detach success for alice, got %d body=%s", rec.Code, rec.Body.String())
	}
	if s.term.IsAttached("loop-actor", "alice") {
		t.Fatal("expected alice to be detached")
	}
	if !s.term.IsAttached("loop-actor", "bob") {
		t.Fatal("expected bob to remain attached")
	}
}

func TestHandleLoopDetachRejectsActorNotAttached(t *testing.T) {
	ms := store.NewMemStore()
	_, _ = ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-actor",
		State:         model.LoopStateRunning,
		CorrelationID: "corr-detach-not-attached",
	}, 0)
	s := &server{
		term:  newTerminalSessionStore(),
		store: ms,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-actor/control/detach", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopDetach(rec, req, "loop-actor")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when actor is not attached, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "actor is not attached" {
		t.Fatalf("expected actor-not-attached error, got %q", body["error"])
	}
	audits, _ := ms.ListAudit(context.Background(), "loop-actor", 0)
	if len(audits) != 1 {
		t.Fatalf("expected one detach rejection audit record, got %d", len(audits))
	}
	if audits[0].Action != "detach-terminal-rejected" {
		t.Fatalf("expected detach-terminal-rejected action, got %q", audits[0].Action)
	}
	if audits[0].Metadata["error_code"] != terminalErrNotAttached {
		t.Fatalf("expected detach rejection error_code %q, got %q", terminalErrNotAttached, audits[0].Metadata["error_code"])
	}
}
func TestHandleLoopControlCommandExecutesAttachedRuntime(t *testing.T) {
	ms := store.NewMemStore()
	_, _ = ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-command",
		State:         model.LoopStateRunning,
		CorrelationID: "corr-command-success",
	}, 0)

	execRunner := &fakePodExecRunner{
		result: podExecResult{
			Stdout:   "hello\n",
			ExitCode: 0,
		},
	}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		store:   ms,
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-command-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo hello"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected command success, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 1 {
		t.Fatalf("expected exactly one pod exec call, got %d", execRunner.calls)
	}
	if execRunner.lastRequest.Command != "echo hello" {
		t.Fatalf("expected command payload echo hello, got %q", execRunner.lastRequest.Command)
	}
	if execRunner.lastRequest.Namespace != runtime.Namespace || execRunner.lastRequest.PodName != runtime.PodName || execRunner.lastRequest.ContainerName != runtime.ContainerName {
		t.Fatalf("unexpected runtime target: %+v", execRunner.lastRequest)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if delivered, ok := body["delivered"].(bool); !ok || !delivered {
		t.Fatalf("expected delivered=true, got %#v", body["delivered"])
	}
	if result, _ := body["result"].(string); result != "success" {
		t.Fatalf("expected result=success, got %#v", body["result"])
	}
	if int(body["exit_code"].(float64)) != 0 {
		t.Fatalf("expected exit_code=0, got %#v", body["exit_code"])
	}
	if stdout, _ := body["stdout"].(string); stdout != "hello\n" {
		t.Fatalf("expected stdout hello\\n, got %#v", body["stdout"])
	}

	journals, _ := ms.ListJournal(context.Background(), "loop-command", 0)
	foundHello := false
	for _, entry := range journals {
		if entry.Message == "hello" {
			foundHello = true
			break
		}
	}
	if !foundHello {
		t.Fatalf("expected journal output containing hello, got %+v", journals)
	}
	audits, _ := ms.ListAudit(context.Background(), "loop-command", 0)
	if len(audits) == 0 {
		t.Fatal("expected terminal command audit entry")
	}
	lastAudit := audits[0]
	if lastAudit.Action != "terminal-command" {
		t.Fatalf("expected terminal-command action, got %q", lastAudit.Action)
	}
	if lastAudit.Metadata["delivered"] != "true" {
		t.Fatalf("expected delivered audit metadata true, got %q", lastAudit.Metadata["delivered"])
	}
	if lastAudit.Metadata["exit_code"] != "0" {
		t.Fatalf("expected exit_code audit metadata 0, got %q", lastAudit.Metadata["exit_code"])
	}
	if lastAudit.Metadata["request_status"] != "accepted" {
		t.Fatalf("expected accepted request_status metadata, got %q", lastAudit.Metadata["request_status"])
	}
}

func TestHandleLoopControlCommandRequiresAttach(t *testing.T) {
	ms := store.NewMemStore()
	_, _ = ms.PutState(context.Background(), model.StateRecord{
		LoopID: "loop-command",
		State:  model.LoopStateUnresolved,
	}, 0)
	execRunner := &fakePodExecRunner{}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		store:   ms,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo hello"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when actor is not attached, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call without attachment, got %d", execRunner.calls)
	}
}
func TestHandleLoopControlCommandRejectsInvalidJSON(t *testing.T) {
	ms := store.NewMemStore()
	_, _ = ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-command",
		State:         model.LoopStateRunning,
		CorrelationID: "corr-command-invalid-json",
	}, 0)
	execRunner := &fakePodExecRunner{}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		store:   ms,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command"`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json payload, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call for invalid json, got %d", execRunner.calls)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["code"] != terminalErrInvalidJSON {
		t.Fatalf("expected invalid json code %q, got %q", terminalErrInvalidJSON, body["code"])
	}
	audits, _ := ms.ListAudit(context.Background(), "loop-command", 0)
	if len(audits) != 1 {
		t.Fatalf("expected one rejected audit entry, got %d", len(audits))
	}
	if audits[0].Metadata["error_code"] != terminalErrInvalidJSON {
		t.Fatalf("expected invalid-json audit error_code %q, got %q", terminalErrInvalidJSON, audits[0].Metadata["error_code"])
	}
}

func TestHandleLoopControlCommandRejectsRequiredCommand(t *testing.T) {
	ms := store.NewMemStore()
	_, _ = ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-command",
		State:         model.LoopStateRunning,
		CorrelationID: "corr-command-required",
	}, 0)
	execRunner := &fakePodExecRunner{}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		store:   ms,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"   "}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when command is required, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call for missing command, got %d", execRunner.calls)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["code"] != terminalErrRequiredCmd {
		t.Fatalf("expected required-command code %q, got %q", terminalErrRequiredCmd, body["code"])
	}
	audits, _ := ms.ListAudit(context.Background(), "loop-command", 0)
	if len(audits) != 1 {
		t.Fatalf("expected one rejected audit entry, got %d", len(audits))
	}
	if audits[0].Metadata["error_code"] != terminalErrRequiredCmd {
		t.Fatalf("expected required-command audit error_code %q, got %q", terminalErrRequiredCmd, audits[0].Metadata["error_code"])
	}
}

func TestHandleLoopControlCommandRejectsOversizedCommand(t *testing.T) {
	ms := store.NewMemStore()
	_, _ = ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-command",
		State:         model.LoopStateRunning,
		CorrelationID: "corr-command-rejected",
	}, 0)
	execRunner := &fakePodExecRunner{}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		store:   ms,
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-command-99999-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	oversized := strings.Repeat("x", terminalCommandMaxSize+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"`+oversized+`"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized command, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call for oversized command, got %d", execRunner.calls)
	}
	audits, _ := ms.ListAudit(context.Background(), "loop-command", 0)
	if len(audits) != 1 {
		t.Fatalf("expected one rejected audit entry, got %d", len(audits))
	}
	if audits[0].Action != "terminal-command-rejected" {
		t.Fatalf("expected terminal-command-rejected action, got %q", audits[0].Action)
	}
	if audits[0].Metadata["result"] != "rejected" {
		t.Fatalf("expected rejected metadata tag, got %q", audits[0].Metadata["result"])
	}
	if audits[0].Metadata["error_code"] != terminalErrTooLong {
		t.Fatalf("expected error_code %q, got %q", terminalErrTooLong, audits[0].Metadata["error_code"])
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["code"] != terminalErrTooLong {
		t.Fatalf("expected API error code %q, got %q", terminalErrTooLong, body["code"])
	}
}

func TestHandleLoopControlCommandRejectsUnauthorizedWithoutExec(t *testing.T) {
	ms := store.NewMemStore()
	execRunner := &fakePodExecRunner{}
	s := &server{
		cfg: config{
			operatorToken: "secret-token",
		},
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		store:   ms,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo hello"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized command, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != 0 {
		t.Fatalf("expected no pod exec call for unauthorized command, got %d", execRunner.calls)
	}
	audits, _ := ms.ListAudit(context.Background(), "loop-command", 0)
	if len(audits) != 1 {
		t.Fatalf("expected one rejected audit entry, got %d", len(audits))
	}
	if audits[0].Metadata["error_code"] != terminalErrUnauthorized {
		t.Fatalf("expected unauthorized error_code %q, got %q", terminalErrUnauthorized, audits[0].Metadata["error_code"])
	}
}

func TestHandleLoopDetachRejectsUnauthorizedBeforeStateLookup(t *testing.T) {
	ms := store.NewMemStore()
	s := &server{
		cfg: config{
			operatorToken: "secret-token",
		},
		term:  newTerminalSessionStore(),
		store: ms,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-auth/control/detach", strings.NewReader(`{"actor":"alice"}`))
	s.handleLoopDetach(rec, req, "loop-auth")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized detach, got %d body=%s", rec.Code, rec.Body.String())
	}
	audits, _ := ms.ListAudit(context.Background(), "loop-auth", 0)
	if len(audits) != 1 {
		t.Fatalf("expected one rejected detach audit entry, got %d", len(audits))
	}
	if audits[0].Action != "detach-terminal-rejected" {
		t.Fatalf("expected detach-terminal-rejected action, got %q", audits[0].Action)
	}
}

func TestHandleLoopControlCommandRateLimitPerSession(t *testing.T) {
	previousWindow := terminalCommandRateWindow
	terminalCommandRateWindow = 30 * time.Second
	t.Cleanup(func() {
		terminalCommandRateWindow = previousWindow
	})

	ms := store.NewMemStore()
	ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-command",
		State:         model.LoopStateRunning,
		CorrelationID: "corr-command-throttle",
	}, 0)

	execRunner := &fakePodExecRunner{
		result: podExecResult{
			Stdout:   "ok\n",
			ExitCode: 0,
		},
	}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		store:   ms,
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-command-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	for i := 0; i < terminalCommandRateMax; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo ok"}`))
		s.handleLoopControlCommand(rec, req, "loop-command")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected command %d to pass within rate limit, got %d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo burst"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d body=%s", rec.Code, rec.Body.String())
	}
	if execRunner.calls != terminalCommandRateMax {
		t.Fatalf("expected pod exec calls capped at %d, got %d", terminalCommandRateMax, execRunner.calls)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode throttled response: %v", err)
	}
	if body["code"] != terminalErrRateLimited {
		t.Fatalf("expected throttled code %q, got %q", terminalErrRateLimited, body["code"])
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header for throttled response")
	}

	audits, _ := ms.ListAudit(context.Background(), "", 0)
	if len(audits) < terminalCommandRateMax+1 {
		t.Fatalf("expected audit records for accepted and throttled commands, got %d", len(audits))
	}
	last := audits[0] // ListAudit returns newest first in MemStore implementation I wrote
	if last.Action != "terminal-command-rejected" {
		t.Fatalf("expected terminal-command-rejected audit action, got %q", last.Action)
	}
	if last.Metadata["rejection_reason"] != "command rate limit exceeded" {
		t.Fatalf("expected throttle rejection reason, got %q", last.Metadata["rejection_reason"])
	}
	if last.Metadata["request_status"] != "rejected" {
		t.Fatalf("expected rejected request_status metadata, got %q", last.Metadata["request_status"])
	}
}
func TestHandleLoopControlCommandHandlesNonZeroExitResult(t *testing.T) {
	ms := store.NewMemStore()
	ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-command",
		State:         model.LoopStateRunning,
		CorrelationID: "corr-command-nonzero",
	}, 0)

	execRunner := &fakePodExecRunner{
		result: podExecResult{
			Stdout:   "ok\n",
			Stderr:   "oops\n",
			ExitCode: 17,
		},
	}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		store:   ms,
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-command-nonzero-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo ok"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected command response status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["result"] != "failed" {
		t.Fatalf("expected result=failed for non-zero exit, got %#v", body["result"])
	}
	if int(body["exit_code"].(float64)) != 17 {
		t.Fatalf("expected exit_code=17, got %#v", body["exit_code"])
	}

	audits, _ := ms.ListAudit(context.Background(), "", 0)
	if len(audits) == 0 {
		t.Fatal("expected terminal command audit entry")
	}
	lastAudit := audits[0]
	if lastAudit.Metadata["result"] != "failed" {
		t.Fatalf("expected failed audit result metadata, got %q", lastAudit.Metadata["result"])
	}
	if lastAudit.Metadata["stderr_bytes"] != "5" {
		t.Fatalf("expected stderr_bytes=5 in audit metadata, got %q", lastAudit.Metadata["stderr_bytes"])
	}

	foundStdout := false
	foundStderr := false
	journals, _ := ms.ListJournal(context.Background(), "loop-command", 0)
	for _, entry := range journals {
		if entry.Message == "ok" && entry.Metadata["stream"] == "stdout" {
			foundStdout = true
		}
		if entry.Message == "oops" && entry.Metadata["stream"] == "stderr" {
			foundStderr = true
		}
	}
	if !foundStdout || !foundStderr {
		t.Fatalf("expected stdout+stderr lines in journal entries, got %+v", journals)
	}
}

func TestHandleLoopControlCommandHandlesExecError(t *testing.T) {
	ms := store.NewMemStore()
	ms.PutState(context.Background(), model.StateRecord{
		LoopID:        "loop-command",
		State:         model.LoopStateRunning,
		CorrelationID: "corr-command-exec-error",
	}, 0)

	execRunner := &fakePodExecRunner{
		result: podExecResult{
			Stdout: "partial\n",
		},
		err: errors.New("runtime transport interrupted"),
	}
	s := &server{
		term:    newTerminalSessionStore(),
		podExec: execRunner,
		store:   ms,
	}
	runtime := loopRuntimeResponse{
		Namespace:     "smith-system",
		PodName:       "smith-replica-loop-command-error-12345-abc",
		ContainerName: "replica",
		PodPhase:      string(corev1.PodRunning),
		Attachable:    true,
	}
	s.term.Attach("loop-command", "alice", "console-pods", runtime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/loops/loop-command/control/command", strings.NewReader(`{"actor":"alice","command":"echo ok"}`))
	s.handleLoopControlCommand(rec, req, "loop-command")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected command response status 200 on exec error reporting path, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["result"] != "error" {
		t.Fatalf("expected result=error when exec runner returns error, got %#v", body["result"])
	}
	if int(body["exit_code"].(float64)) != -1 {
		t.Fatalf("expected exit_code=-1 on exec error, got %#v", body["exit_code"])
	}
	if body["error"] != "runtime transport interrupted" {
		t.Fatalf("expected exec error message in response, got %#v", body["error"])
	}

	audits, _ := ms.ListAudit(context.Background(), "", 0)
	if len(audits) == 0 {
		t.Fatal("expected terminal command audit entry")
	}
	lastAudit := audits[0]
	if lastAudit.Metadata["result"] != "error" {
		t.Fatalf("expected error result in audit metadata, got %q", lastAudit.Metadata["result"])
	}
	if lastAudit.Metadata["exec_error"] != "runtime transport interrupted" {
		t.Fatalf("expected exec_error metadata to be set, got %q", lastAudit.Metadata["exec_error"])
	}
}

func assertTerminalMetadata(t *testing.T, metadata map[string]string, actor, terminal, runtimeRef string) {
	t.Helper()
	if metadata["actor"] != actor {
		t.Fatalf("expected metadata actor %q, got %q", actor, metadata["actor"])
	}
	if metadata["terminal"] != terminal {
		t.Fatalf("expected metadata terminal %q, got %q", terminal, metadata["terminal"])
	}
	if metadata["runtime_target_ref"] != runtimeRef {
		t.Fatalf("expected runtime_target_ref %q, got %q", runtimeRef, metadata["runtime_target_ref"])
	}
}

func TestHandleTasksCreateAndGet(t *testing.T) {
	ms := store.NewMemStore()
	s := &server{store: ms}

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(`{
		"project_id":"smith",
		"provider_profile_id":"openai-work",
		"source_document":"docs/task.md",
		"objective":"Restore green CI",
		"validation":["go test ./..."],
		"actor":"alice"
	}`))
	s.handleTasks(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created api.TaskContract
	require.NoError(t, json.NewDecoder(createRec.Body).Decode(&created))
	require.NotEmpty(t, created.ID)
	assert.Equal(t, "smith.task", created.Kind)
	assert.Equal(t, api.TaskContractStatusDraft, created.Status)

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/tasks/"+created.ID, nil)
	s.handleTaskByID(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)

	var fetched api.TaskContract
	require.NoError(t, json.NewDecoder(getRec.Body).Decode(&fetched))
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Restore green CI", fetched.Objective)
}

func TestHandleTaskPatchUpdatesMutableFieldsAndAudit(t *testing.T) {
	ms := store.NewMemStore()
	seed := model.TaskContract{
		Kind:               "smith.task",
		ID:                 "task-validated",
		ProjectID:          "smith",
		ProviderProfileID:  "openai-work",
		Objective:          "Initial objective",
		Validation:         []string{"go test ./..."},
		Status:             model.TaskContractStatusValidated,
		CorrelationID:      "task-corr-validated",
		AcceptanceCriteria: []string{"tests pass"},
	}
	require.NoError(t, ms.PutTaskContract(context.Background(), seed))

	s := &server{store: ms}
	patchRec := httptest.NewRecorder()
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/tasks/task-validated", strings.NewReader(`{
		"objective":"Updated objective",
		"status":"draft",
		"validation":["go test ./...", "npm --prefix frontend run check"],
		"actor":"alice"
	}`))
	s.handleTaskByID(patchRec, patchReq)
	require.Equal(t, http.StatusOK, patchRec.Code)

	var updated api.TaskContract
	require.NoError(t, json.NewDecoder(patchRec.Body).Decode(&updated))
	assert.Equal(t, "Updated objective", updated.Objective)
	assert.Equal(t, api.TaskContractStatusDraft, updated.Status)
	require.Len(t, updated.Validation, 2)

	audits, err := ms.ListAudit(context.Background(), "", 0)
	require.NoError(t, err)
	require.NotEmpty(t, audits)
	assert.Equal(t, "patch-task", audits[0].Action)
	assert.Contains(t, audits[0].Metadata["changed_fields"], "objective")
	assert.Contains(t, audits[0].Metadata["changed_fields"], "status")
}

func TestHandleTaskApproveRequiresValidatedStatus(t *testing.T) {
	ms := store.NewMemStore()
	require.NoError(t, ms.PutTaskContract(context.Background(), model.TaskContract{
		Kind:              "smith.task",
		ID:                "task-draft",
		ProjectID:         "smith",
		ProviderProfileID: "openai-work",
		Objective:         "Draft task",
		Validation:        []string{"go test ./..."},
		Status:            model.TaskContractStatusDraft,
		CorrelationID:     "task-corr-draft",
	}))

	s := &server{store: ms}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/task-draft/approve", strings.NewReader(`{"actor":"alice"}`))
	s.handleTaskByID(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "validated")
}

func TestHandleTaskApproveTransitionsAndAudit(t *testing.T) {
	ms := store.NewMemStore()
	require.NoError(t, ms.PutTaskContract(context.Background(), model.TaskContract{
		Kind:              "smith.task",
		ID:                "task-ready",
		ProjectID:         "smith",
		ProviderProfileID: "openai-work",
		Objective:         "Ready for approval",
		Validation:        []string{"go test ./..."},
		Status:            model.TaskContractStatusValidated,
		CorrelationID:     "task-corr-ready",
	}))

	s := &server{store: ms}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/task-ready/approve", strings.NewReader(`{"actor":"alice"}`))
	s.handleTaskByID(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var approved api.TaskContract
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&approved))
	assert.Equal(t, api.TaskContractStatusApproved, approved.Status)

	audits, err := ms.ListAudit(context.Background(), "", 0)
	require.NoError(t, err)
	require.NotEmpty(t, audits)
	assert.Equal(t, "approve-task", audits[0].Action)
	assert.Equal(t, "validated", audits[0].Metadata["status_from"])
	assert.Equal(t, "approved", audits[0].Metadata["status_to"])
}

func TestHandleProvidersReturnsDefaultProfile(t *testing.T) {
	s := &server{
		providers:    provider.NewFileProviderProfileStore(),
		projectStore: &memoryProjectStore{items: map[string]provider.Project{}},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	s.handleProviders(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var out []provider.ProviderProfile
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
	require.NotEmpty(t, out)
	ids := make([]string, 0, len(out))
	for _, profile := range out {
		ids = append(ids, profile.ID)
	}
	assert.Contains(t, ids, provider.DefaultProviderProfileID)
	assert.NotContains(t, ids, "claude-default")
	assert.NotContains(t, ids, "gemini-default")
}

func TestHandleProviderCatalogReturnsSupportedProviderSet(t *testing.T) {
	s := &server{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/catalog", nil)
	s.handleProviderCatalog(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var out []provider.CatalogEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
	require.Len(t, out, 1)
	ids := []string{out[0].ID}
	assert.Equal(t, []string{provider.ProviderCodex}, ids)
	assert.NotEmpty(t, out[0].RequiredConfigFields)
}

func TestHandleProviderCatalogIncludesFlaggedProvidersWhenEnabled(t *testing.T) {
	s := &server{cfg: config{providerClaudeEnabled: true, providerGeminiEnabled: true}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/catalog", nil)
	s.handleProviderCatalog(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var out []provider.CatalogEntry
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
	require.Len(t, out, 3)
	ids := []string{out[0].ID, out[1].ID, out[2].ID}
	assert.Equal(t, []string{provider.ProviderCodex, provider.ProviderClaude, provider.ProviderGemini}, ids)
}

func TestHandleProjectsAssignsDefaultProviderProfile(t *testing.T) {
	projectStore := &memoryProjectStore{items: map[string]provider.Project{}}
	s := &server{
		providers:    provider.NewFileProviderProfileStore(),
		projectStore: projectStore,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(`{
		"id":"proj-1",
		"name":"Project 1",
		"repo_url":"https://github.com/acme/project1"
	}`))
	s.handleProjects(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var saved provider.Project
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&saved))
	assert.Equal(t, provider.DefaultProviderProfileID, saved.ProviderProfileID)
	assert.Equal(t, "IfNotPresent", saved.RuntimePullPolicy)
	assert.Equal(t, "IfNotPresent", saved.SkillsPullPolicy)

	stored, ok := projectStore.items["proj-1"]
	require.True(t, ok)
	assert.Equal(t, provider.DefaultProviderProfileID, stored.ProviderProfileID)
}

func TestHandleProjectsRejectsUnknownProviderProfile(t *testing.T) {
	projectStore := &memoryProjectStore{items: map[string]provider.Project{}}
	s := &server{
		providers:    provider.NewFileProviderProfileStore(),
		projectStore: projectStore,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(`{
		"id":"proj-2",
		"name":"Project 2",
		"repo_url":"https://github.com/acme/project2",
		"provider_profile_id":"does-not-exist"
	}`))
	s.handleProjects(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "provider profile")
}

func TestOnboardingReadinessTransitionsFromIncompleteToComplete(t *testing.T) {
	credStore := provider.NewFileTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	projectStore := &memoryProjectStore{items: map[string]provider.Project{}}
	s := &server{
		providers:    provider.NewFileProviderProfileStore(),
		projectStore: projectStore,
		projectCred:  credStore,
	}

	initialRec := httptest.NewRecorder()
	initialReq := httptest.NewRequest(http.MethodGet, "/v1/onboarding/readiness?project_id=proj-onboard", nil)
	s.handleOnboardingReadiness(initialRec, initialReq)
	require.Equal(t, http.StatusOK, initialRec.Code)
	var initial api.OnboardingReadinessResponse
	require.NoError(t, json.NewDecoder(initialRec.Body).Decode(&initial))
	assert.False(t, initial.Ready)
	assert.Contains(t, initial.Missing, "project")

	repoRec := httptest.NewRecorder()
	repoReq := httptest.NewRequest(http.MethodPost, "/v1/onboarding/repository", strings.NewReader(`{
		"project_id":"proj-onboard",
		"repo_url":"https://github.com/acme/repo",
		"provider_profile_id":"codex-default"
	}`))
	s.handleOnboardingRepository(repoRec, repoReq)
	require.Equal(t, http.StatusOK, repoRec.Code)

	midRec := httptest.NewRecorder()
	midReq := httptest.NewRequest(http.MethodGet, "/v1/onboarding/readiness?project_id=proj-onboard", nil)
	s.handleOnboardingReadiness(midRec, midReq)
	require.Equal(t, http.StatusOK, midRec.Code)
	var mid api.OnboardingReadinessResponse
	require.NoError(t, json.NewDecoder(midRec.Body).Decode(&mid))
	assert.False(t, mid.Ready)
	assert.Contains(t, mid.Missing, "github_credential")

	credRec := httptest.NewRecorder()
	credReq := httptest.NewRequest(http.MethodPost, "/v1/projects/credentials/github", strings.NewReader(`{
		"project_id":"proj-onboard",
		"github_user":"alice",
		"credential":"ghp_test_1234567890"
	}`))
	s.handleProjectGitHubCredential(credRec, credReq)
	require.Equal(t, http.StatusOK, credRec.Code)

	finalRec := httptest.NewRecorder()
	finalReq := httptest.NewRequest(http.MethodGet, "/v1/onboarding/readiness?project_id=proj-onboard", nil)
	s.handleOnboardingReadiness(finalRec, finalReq)
	require.Equal(t, http.StatusOK, finalRec.Code)
	var final api.OnboardingReadinessResponse
	require.NoError(t, json.NewDecoder(finalRec.Body).Decode(&final))
	assert.True(t, final.Ready)
	assert.Empty(t, final.Missing)
	assert.True(t, final.Credential.Valid)
	assert.True(t, final.Credential.CredentialSet)
	assert.NotContains(t, final.Credential.CredentialMasked, "ghp_test_1234567890")
}

func TestOnboardingCredentialValidateReturnsMaskedStatus(t *testing.T) {
	credStore := provider.NewFileTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	s := &server{projectCred: credStore}

	missingRec := httptest.NewRecorder()
	missingReq := httptest.NewRequest(http.MethodPost, "/v1/onboarding/credentials/validate", strings.NewReader(`{"project_id":"proj-cred"}`))
	s.handleOnboardingCredentialValidate(missingRec, missingReq)
	require.Equal(t, http.StatusOK, missingRec.Code)
	var missing api.OnboardingCredentialStatus
	require.NoError(t, json.NewDecoder(missingRec.Body).Decode(&missing))
	assert.False(t, missing.Valid)
	assert.False(t, missing.CredentialSet)

	require.NoError(t, credStore.PutProjectCredential(context.Background(), "proj-cred", provider.ProjectCredential{
		GitHubUser: "bob",
		PAT:        "ghp_secret_token_12345",
		UpdatedAt:  time.Now().UTC(),
	}))

	validRec := httptest.NewRecorder()
	validReq := httptest.NewRequest(http.MethodPost, "/v1/onboarding/credentials/validate", strings.NewReader(`{"project_id":"proj-cred"}`))
	s.handleOnboardingCredentialValidate(validRec, validReq)
	require.Equal(t, http.StatusOK, validRec.Code)
	var valid api.OnboardingCredentialStatus
	require.NoError(t, json.NewDecoder(validRec.Body).Decode(&valid))
	assert.True(t, valid.Valid)
	assert.True(t, valid.CredentialSet)
	assert.NotContains(t, valid.CredentialMasked, "ghp_secret_token_12345")
}

func TestHandleProviderByIDSupportsAPIAliasAndAuditsConfigChanges(t *testing.T) {
	ms := store.NewMemStore()
	projectStore := &memoryProjectStore{items: map[string]provider.Project{}}
	s := &server{
		store:        ms,
		providers:    provider.NewFileProviderProfileStore(),
		projectStore: projectStore,
		secrets:      provider.NewFileSecretStore(),
	}
	require.NoError(t, s.secrets.PutSecret(context.Background(), provider.SettingsSecret{ID: "openai-key", Value: "sk-live-test"}))

	putRec := httptest.NewRecorder()
	putReq := httptest.NewRequest(http.MethodPut, "/api/providers/openai-work", strings.NewReader(`{
		"id":"openai-work",
		"provider_type":"openai",
		"secret_ref":"openai-key",
		"default_model":"gpt-5.4"
	}`))
	s.handleProviderByID(putRec, putReq)
	require.Equal(t, http.StatusOK, putRec.Code)

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/providers/openai-work", nil)
	s.handleProviderByID(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)
	assert.Contains(t, getRec.Body.String(), "openai-work")

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/providers/openai-work", nil)
	s.handleProviderByID(deleteRec, deleteReq)
	require.Equal(t, http.StatusNoContent, deleteRec.Code)

	audits, err := ms.ListAudit(context.Background(), "", 0)
	require.NoError(t, err)
	require.NotEmpty(t, audits)

	actions := map[string]bool{}
	for _, audit := range audits {
		actions[audit.Action] = true
	}
	assert.True(t, actions["update-provider-profile"])
	assert.True(t, actions["delete-provider-profile"])
}

func TestHandleProviderByIDModelsReturnsAccountScopedInventory(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("expected /v1/models path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-live-test" {
			t.Fatalf("expected bearer credential header, got %q", got)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"id": "ada:ft-org-2025-01-01", "owned_by": "openai", "created": 1600000000},
				{"id": "gpt-5-codex", "owned_by": "openai", "created": 1700000010},
				{"id": "gpt-4.1", "owned_by": "openai", "created": 1700000000},
			},
		})
	}))
	defer upstream.Close()

	providerStore := provider.NewFileProviderProfileStore()
	secretStore := provider.NewFileSecretStore()
	require.NoError(t, secretStore.PutSecret(context.Background(), provider.SettingsSecret{
		ID:    "openai-key",
		Name:  "OpenAI key",
		Value: "sk-live-test",
	}))
	require.NoError(t, providerStore.PutProviderProfile(context.Background(), provider.ProviderProfile{
		ID:           "openai-work",
		ProviderType: "openai",
		SecretRef:    "openai-key",
		DefaultModel: "gpt-5-codex",
		Endpoint:     upstream.URL + "/v1",
	}))

	s := &server{
		providers:    providerStore,
		secrets:      secretStore,
		projectStore: &memoryProjectStore{items: map[string]provider.Project{}},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/openai-work/models", nil)
	s.handleProviderByID(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var out api.ProviderModelsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
	assert.Equal(t, "openai-work", out.ProviderID)
	assert.Equal(t, "codex", out.ProviderType)
	assert.Equal(t, "account_scoped_chat", out.Source)
	if assert.Len(t, out.Models, 2) {
		assert.Equal(t, "gpt-4.1", out.Models[0].ID)
		assert.Equal(t, "gpt-5-codex", out.Models[1].ID)
	}
}

func TestHandleProviderByIDModelsSupportsIncludeAll(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"id": "ada:ft-org-2025-01-01", "owned_by": "openai", "created": 1600000000},
				{"id": "gpt-5-codex", "owned_by": "openai", "created": 1700000010},
			},
		})
	}))
	defer upstream.Close()

	providerStore := provider.NewFileProviderProfileStore()
	secretStore := provider.NewFileSecretStore()
	require.NoError(t, secretStore.PutSecret(context.Background(), provider.SettingsSecret{ID: "openai-key", Value: "sk-live-test"}))
	require.NoError(t, providerStore.PutProviderProfile(context.Background(), provider.ProviderProfile{
		ID:           "openai-work",
		ProviderType: "openai",
		SecretRef:    "openai-key",
		Endpoint:     upstream.URL + "/v1",
	}))

	s := &server{providers: providerStore, secrets: secretStore, projectStore: &memoryProjectStore{items: map[string]provider.Project{}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/openai-work/models?include=all", nil)
	s.handleProviderByID(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var out api.ProviderModelsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
	assert.Equal(t, "account_scoped_all", out.Source)
	if assert.Len(t, out.Models, 2) {
		assert.Equal(t, "ada:ft-org-2025-01-01", out.Models[0].ID)
		assert.Equal(t, "gpt-5-codex", out.Models[1].ID)
	}
}

func TestHandleProviderByIDModelsRejectsMissingCredentialSource(t *testing.T) {
	providerStore := provider.NewFileProviderProfileStore()
	require.NoError(t, providerStore.PutProviderProfile(context.Background(), provider.ProviderProfile{
		ID:           "openai-work",
		ProviderType: "openai",
		DefaultModel: "gpt-5-codex",
	}))

	s := &server{
		providers:    providerStore,
		projectStore: &memoryProjectStore{items: map[string]provider.Project{}},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/openai-work/models", nil)
	s.handleProviderByID(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "requires secret_ref")
}

func TestHandleProjectsSupportsAPIAliasAndAuditsConfigChanges(t *testing.T) {
	ms := store.NewMemStore()
	projectStore := &memoryProjectStore{items: map[string]provider.Project{}}
	s := &server{
		store:        ms,
		providers:    provider.NewFileProviderProfileStore(),
		projectStore: projectStore,
	}

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{
		"id":"proj-api",
		"name":"Project API",
		"repo_url":"https://github.com/acme/project-api"
	}`))
	s.handleProjects(createRec, createReq)
	require.Equal(t, http.StatusOK, createRec.Code)

	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPut, "/api/projects/proj-api", strings.NewReader(`{
		"id":"proj-api",
		"name":"Project API Updated",
		"repo_url":"https://github.com/acme/project-api",
		"provider_profile_id":"codex-default"
	}`))
	s.handleProjectByID(updateRec, updateReq)
	require.Equal(t, http.StatusOK, updateRec.Code)

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/projects/proj-api", nil)
	s.handleProjectByID(deleteRec, deleteReq)
	require.Equal(t, http.StatusNoContent, deleteRec.Code)

	audits, err := ms.ListAudit(context.Background(), "", 0)
	require.NoError(t, err)
	require.NotEmpty(t, audits)

	actions := map[string]bool{}
	for _, audit := range audits {
		actions[audit.Action] = true
	}
	assert.True(t, actions["create-project"])
	assert.True(t, actions["update-project"])
	assert.True(t, actions["delete-project"])
}

func TestHandleSecretsCRUDAndMasking(t *testing.T) {
	ms := store.NewMemStore()
	secretStore := provider.NewFileSecretStore()
	s := &server{
		store:     ms,
		providers: provider.NewFileProviderProfileStore(),
		secrets:   secretStore,
	}

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/secrets", strings.NewReader(`{
		"id":"openai-key",
		"name":"OpenAI Key",
		"description":"Primary key",
		"value":"sk-test-123456"
	}`))
	s.handleSecrets(createRec, createReq)
	require.Equal(t, http.StatusOK, createRec.Code)
	assert.NotContains(t, createRec.Body.String(), "sk-test-123456")

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/secrets", nil)
	s.handleSecrets(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)
	assert.Contains(t, listRec.Body.String(), "openai-key")
	assert.Contains(t, listRec.Body.String(), "value_masked")
	assert.NotContains(t, listRec.Body.String(), "sk-test-123456")

	stored, found, err := secretStore.GetSecret(context.Background(), "openai-key")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "sk-test-123456", stored.Value)

	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPut, "/v1/secrets/openai-key", strings.NewReader(`{
		"id":"openai-key",
		"name":"OpenAI Key",
		"description":"Primary key rotated",
		"value":"sk-test-rotated-abcdef"
	}`))
	s.handleSecretByID(updateRec, updateReq)
	require.Equal(t, http.StatusOK, updateRec.Code)
	assert.Contains(t, updateRec.Body.String(), "value_masked")
	assert.NotContains(t, updateRec.Body.String(), "sk-test-rotated-abcdef")

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/secrets/openai-key", nil)
	s.handleSecretByID(deleteRec, deleteReq)
	require.Equal(t, http.StatusNoContent, deleteRec.Code)

	_, found, err = secretStore.GetSecret(context.Background(), "openai-key")
	require.NoError(t, err)
	assert.False(t, found)

	audits, err := ms.ListAudit(context.Background(), "", 0)
	require.NoError(t, err)
	require.NotEmpty(t, audits)
	actions := map[string]bool{}
	for _, audit := range audits {
		actions[audit.Action] = true
	}
	assert.True(t, actions["create-secret"])
	assert.True(t, actions["update-secret"])
	assert.True(t, actions["delete-secret"])
}

func TestHandleProjectCredentialsAuditMutations(t *testing.T) {
	ms := store.NewMemStore()
	credStore := provider.NewFileTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	s := &server{
		store:       ms,
		projectCred: credStore,
	}

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/projects/credentials/github", strings.NewReader(`{
		"project_id":"proj-cred-audit",
		"github_user":"alice",
		"credential":"ghp_test_1234567890"
	}`))
	s.handleProjectGitHubCredential(createRec, createReq)
	require.Equal(t, http.StatusOK, createRec.Code)

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/projects/credentials/github?project_id=proj-cred-audit", nil)
	s.handleProjectGitHubCredential(deleteRec, deleteReq)
	require.Equal(t, http.StatusOK, deleteRec.Code)

	audits, err := ms.ListAudit(context.Background(), "", 0)
	require.NoError(t, err)
	require.NotEmpty(t, audits)
	actions := map[string]bool{}
	for _, audit := range audits {
		actions[audit.Action] = true
	}
	assert.True(t, actions["update-project-credential"])
	assert.True(t, actions["delete-project-credential"])
}

func TestHandleProjectGitHubCredentialTestReturnsActionableErrorsAndAudits(t *testing.T) {
	ms := store.NewMemStore()
	credStore := provider.NewFileTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	projectStore := &memoryProjectStore{items: map[string]provider.Project{
		"proj-test": {
			ID:      "proj-test",
			RepoURL: "https://github.com/acme/repo",
		},
	}}
	require.NoError(t, credStore.PutProjectCredential(context.Background(), "proj-test", provider.ProjectCredential{
		GitHubUser: "alice",
		PAT:        "ghp_test_1234567890",
		UpdatedAt:  time.Now().UTC(),
	}))

	s := &server{
		store:        ms,
		projectCred:  credStore,
		projectStore: projectStore,
		repoAccess: func(ctx context.Context, repoURL string, githubUser string, credential string) (bool, string, error) {
			return false, "credential rejected by GitHub (401 unauthorized)", nil
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/credentials/github/test", strings.NewReader(`{"project_id":"proj-test"}`))
	s.handleProjectGitHubCredentialTest(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var out api.ProjectCredentialTestResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
	assert.False(t, out.Valid)
	assert.Equal(t, "proj-test", out.ProjectID)
	assert.Contains(t, out.Message, "401")

	audits, err := ms.ListAudit(context.Background(), "", 0)
	require.NoError(t, err)
	require.NotEmpty(t, audits)
	assert.Equal(t, "test-project-credential", audits[0].Action)
	assert.Equal(t, "proj-test", audits[0].Metadata["project_id"])
	assert.Equal(t, "false", audits[0].Metadata["valid"])
}

func TestHandleProjectGitHubCredentialTestReturnsMissingCredentialMessage(t *testing.T) {
	ms := store.NewMemStore()
	credStore := provider.NewFileTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	projectStore := &memoryProjectStore{items: map[string]provider.Project{
		"proj-missing": {
			ID:      "proj-missing",
			RepoURL: "https://github.com/acme/repo",
		},
	}}

	s := &server{
		store:        ms,
		projectCred:  credStore,
		projectStore: projectStore,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/credentials/github/test", strings.NewReader(`{"project_id":"proj-missing"}`))
	s.handleProjectGitHubCredentialTest(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var out api.ProjectCredentialTestResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
	assert.False(t, out.Valid)
	assert.Equal(t, "credential missing", out.Message)
}

func TestHandleProvidersRejectsUnknownSecretRef(t *testing.T) {
	s := &server{
		providers: provider.NewFileProviderProfileStore(),
		secrets:   provider.NewFileSecretStore(),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/providers", strings.NewReader(`{
		"id":"openai-profile",
		"provider_type":"openai",
		"secret_ref":"missing-secret"
	}`))
	s.handleProviders(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "secret")
}

func TestHandleProvidersRejectsMissingSecretRefForCodex(t *testing.T) {
	s := &server{
		providers: provider.NewFileProviderProfileStore(),
		secrets:   provider.NewFileSecretStore(),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/providers", strings.NewReader(`{
		"id":"codex-team",
		"provider_type":"codex"
	}`))
	s.handleProviders(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "requires secret_ref")
}

func TestResolveProviderCredentialForModelInventoryRequiresSecretRefForCodex(t *testing.T) {
	s := &server{secrets: provider.NewFileSecretStore()}
	_, err := s.resolveProviderCredentialForModelInventory(context.Background(), provider.ProviderProfile{
		ID:           "codex-default",
		ProviderType: provider.ProviderCodex,
		SecretRef:    "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires secret_ref")
}

func TestHandleProvidersRejectsUnsupportedProviderType(t *testing.T) {
	s := &server{
		providers: provider.NewFileProviderProfileStore(),
		secrets:   provider.NewFileSecretStore(),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/providers", strings.NewReader(`{
		"id":"custom-provider",
		"provider_type":"custom"
	}`))
	s.handleProviders(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "unsupported provider_type")
}

func TestHandleProvidersRejectsDisabledProviderType(t *testing.T) {
	s := &server{
		providers: provider.NewFileProviderProfileStore(),
		secrets:   provider.NewFileSecretStore(),
		cfg:       config{providerClaudeEnabled: false, providerGeminiEnabled: false},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/providers", strings.NewReader(`{
		"id":"claude-team",
		"provider_type":"claude",
		"secret_ref":"claude-key"
	}`))
	s.handleProviders(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "disabled by feature flag")
}

func TestHandleSecretDeleteRejectsReferencedSecret(t *testing.T) {
	secretStore := provider.NewFileSecretStore()
	require.NoError(t, secretStore.PutSecret(context.Background(), provider.SettingsSecret{
		ID:    "openai-key",
		Name:  "OpenAI key",
		Value: "sk-test-123456",
	}))
	providerStore := provider.NewFileProviderProfileStore()
	require.NoError(t, providerStore.PutProviderProfile(context.Background(), provider.ProviderProfile{
		ID:           "openai-profile",
		Name:         "OpenAI profile",
		ProviderType: "openai",
		SecretRef:    "openai-key",
	}))

	s := &server{
		providers: providerStore,
		secrets:   secretStore,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/secrets/openai-key", nil)
	s.handleSecretByID(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "referenced")
}

func newPRDValidationTestServer(ms *store.MemStore) *server {
	return &server{
		store:       ms,
		presets:     newPresetCatalog("standard"),
		term:        newTerminalSessionStore(),
		skillPolicy: model.SkillPolicy{},
	}
}

func assertDiagnosticCodeInReport(t *testing.T, diagnostics []model.PRDValidationDiagnostic, want string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == want {
			return
		}
	}
	t.Fatalf("expected diagnostic code %q in %+v", want, diagnostics)
}

type fakeRuntimePodReader struct {
	podsByJob map[string][]corev1.Pod
	err       error
	calls     int
}

type memoryProjectStore struct {
	items map[string]provider.Project
}

func (m *memoryProjectStore) ListProjects(ctx context.Context) ([]provider.Project, error) {
	out := make([]provider.Project, 0, len(m.items))
	for _, item := range m.items {
		out = append(out, item)
	}
	return out, nil
}

func (m *memoryProjectStore) GetProject(ctx context.Context, id string) (provider.Project, bool, error) {
	item, ok := m.items[id]
	if !ok {
		return provider.Project{}, false, nil
	}
	return item, true, nil
}

func (m *memoryProjectStore) PutProject(ctx context.Context, project provider.Project) error {
	m.items[project.ID] = project
	return nil
}

func (m *memoryProjectStore) DeleteProject(ctx context.Context, id string) error {
	delete(m.items, id)
	return nil
}

func (f *fakeRuntimePodReader) List(_ context.Context, _ string, opts metav1.ListOptions) (*corev1.PodList, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	jobName := selectorValue(opts.LabelSelector, "job-name")
	return &corev1.PodList{Items: f.podsByJob[jobName]}, nil
}

func selectorValue(selector, key string) string {
	key = strings.TrimSpace(key)
	for _, segment := range strings.Split(selector, ",") {
		left, right, ok := strings.Cut(segment, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(left) == key {
			return strings.TrimSpace(right)
		}
	}
	return ""
}

type fakePodExecRunner struct {
	result      podExecResult
	err         error
	calls       int
	lastRequest podExecRequest
}

func (f *fakePodExecRunner) Execute(_ context.Context, req podExecRequest) (podExecResult, error) {
	f.calls++
	f.lastRequest = req
	return f.result, f.err
}

func TestDeriveLoopIDIsStable(t *testing.T) {
	tests := []struct {
		name           string
		projectID      string
		idempotencyKey string
		sourceType     string
		sourceRef      string
	}{
		{
			name:           "basic",
			projectID:      "smith",
			idempotencyKey: "key1",
			sourceType:     "type1",
			sourceRef:      "ref1",
		},
		{
			name:           "empty-idempotency",
			projectID:      "smith",
			idempotencyKey: "",
			sourceType:     "type1",
			sourceRef:      "ref1",
		},
		{
			name:           "special-chars",
			projectID:      "smith",
			idempotencyKey: "key with spaces / and dots.",
			sourceType:     "type1",
			sourceRef:      "ref1",
		},
		{
			name:           "no-project",
			projectID:      "",
			idempotencyKey: "key1",
			sourceType:     "type1",
			sourceRef:      "ref1",
		},
		{
			name:           "cleaned-to-empty",
			projectID:      "smith",
			idempotencyKey: "!!!",
			sourceType:     "!!!",
			sourceRef:      "!!!",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id1 := deriveLoopID(tc.projectID, tc.idempotencyKey, tc.sourceType, tc.sourceRef)
			id2 := deriveLoopID(tc.projectID, tc.idempotencyKey, tc.sourceType, tc.sourceRef)
			if id1 != id2 {
				t.Fatalf("deriveLoopID is not stable: %q != %q", id1, id2)
			}
			if strings.Contains(id1, " ") {
				t.Fatalf("generated ID contains spaces: %q", id1)
			}
		})
	}
}

func TestDeriveLoopIDDifferentInputs(t *testing.T) {
	id1 := deriveLoopID("proj1", "key1", "type1", "ref1")
	id2 := deriveLoopID("proj1", "key2", "type1", "ref1")
	if id1 == id2 {
		t.Fatalf("deriveLoopID collision for different keys: %q", id1)
	}

	id3 := deriveLoopID("proj1", "", "type1", "ref1")
	id4 := deriveLoopID("proj1", "", "type1", "ref2")
	if id3 == id4 {
		t.Fatalf("deriveLoopID collision for different source refs: %q", id3)
	}
}

func TestDeriveLoopIDDoesNotEndWithHyphen(t *testing.T) {
	inputs := []struct {
		projectID      string
		idempotencyKey string
		sourceType     string
		sourceRef      string
	}{
		{
			projectID:      "smith",
			idempotencyKey: "prd:smoke:autonomous-prd-json-meta#US-001",
			sourceType:     "prd_story",
			sourceRef:      "smoke:autonomous-prd-json-meta#US-001",
		},
		{
			projectID:      "smith",
			idempotencyKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-tail",
			sourceType:     "manual",
			sourceRef:      "manual/ref",
		},
	}

	for _, input := range inputs {
		loopID := deriveLoopID(input.projectID, input.idempotencyKey, input.sourceType, input.sourceRef)
		if strings.HasSuffix(loopID, "-") {
			t.Fatalf("loop id must not end with hyphen: %q", loopID)
		}
	}
}

func TestDeriveLoopIDAvoidsAdjacentDuplicateSegments(t *testing.T) {
	loopID := deriveLoopID("smith", "prd:prd:workspace-clone-smoke-1773759516-n#US-001", "prd_story", "prd:workspace-clone-smoke-1773759516-n#US-001")
	if strings.Contains(loopID, "-prd-prd-") {
		t.Fatalf("expected redundant adjacent segments to be collapsed, got %q", loopID)
	}
}

func TestCollapseRedundantIDSegments(t *testing.T) {
	got := collapseRedundantIDSegments("prd-prd-workspace---clone-clone-smoke")
	if got != "prd-workspace-clone-smoke" {
		t.Fatalf("unexpected collapsed segments: %q", got)
	}
}

func setupTestGRPC(t *testing.T) (store.StateStore, pb.SmithServiceClient, func()) {
	es := store.NewMemStore()

	gs := &grpcServer{
		store:       es,
		presets:     newPresetCatalog("standard"),
		skillPolicy: model.DefaultSkillPolicy(),
	}

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	pb.RegisterSmithServiceServer(server, gs)

	go func() {
		_ = server.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client := pb.NewSmithServiceClient(conn)

	return es, client, func() {
		conn.Close()
		server.Stop()
		lis.Close()
	}
}

func TestGRPCServer_ListLoops(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	es, client, cleanup := setupTestGRPC(t)
	defer cleanup()

	loopID := "loop-grpc-test"
	_, _ = es.PutState(ctx, model.StateRecord{
		LoopID: loopID,
		State:  model.LoopStateRunning,
	}, 0)

	res, err := client.ListLoops(ctx, &pb.ListLoopsRequest{})
	assert.NoError(t, err)
	require.NotNil(t, res)
	assert.NotEmpty(t, res.Loops)
	assert.Equal(t, loopID, res.Loops[0].Record.LoopId)
}

func TestGRPCServer_CreateLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, client, cleanup := setupTestGRPC(t)
	defer cleanup()

	req := &pb.LoopCreateRequest{
		Title:      "gRPC Created Loop",
		SourceType: "grpc",
		SourceRef:  "test",
		ProviderId: "codex",
		Model:      "gpt-5-codex",
	}

	res, err := client.CreateLoop(ctx, req)
	assert.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Created)
	assert.NotEmpty(t, res.LoopId)
}

func TestGRPCServer_GetLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	es, client, cleanup := setupTestGRPC(t)
	defer cleanup()

	loopID := "loop-get-test"
	_, _ = es.PutState(ctx, model.StateRecord{
		LoopID: loopID,
		State:  model.LoopStateSynced,
	}, 0)

	res, err := client.GetLoop(ctx, &pb.GetLoopRequest{LoopId: loopID})
	assert.NoError(t, err)
	assert.Equal(t, pb.LoopState_LOOP_STATE_SYNCED, res.State.State)
}

func TestGRPCServer_DeleteLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	es, client, cleanup := setupTestGRPC(t)
	defer cleanup()

	loopID := "loop-delete-test"
	_, _ = es.PutState(ctx, model.StateRecord{
		LoopID: loopID,
		State:  model.LoopStateRunning,
	}, 0)

	res, err := client.DeleteLoop(ctx, &pb.LoopDeleteRequest{LoopId: loopID, Actor: "test-actor"})
	assert.NoError(t, err)
	assert.Equal(t, "deleted", res.Status)

	state, _, _ := es.GetState(ctx, loopID)
	assert.Equal(t, model.LoopStateCancelled, state.Record.State)
}

func TestGRPCServer_OverrideLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	es, client, cleanup := setupTestGRPC(t)
	defer cleanup()

	loopID := "loop-override-test"
	_, _ = es.PutState(ctx, model.StateRecord{
		LoopID: loopID,
		State:  model.LoopStateRunning,
	}, 0)

	res, err := client.OverrideLoop(ctx, &pb.OverrideRequest{
		LoopId:      loopID,
		TargetState: pb.LoopState_LOOP_STATE_SYNCED,
		Reason:      "manual fix",
		Actor:       "operator",
	})
	assert.NoError(t, err)
	assert.Equal(t, "overridden", res.Status)
	assert.Equal(t, pb.LoopState_LOOP_STATE_SYNCED, res.State.State)
}
