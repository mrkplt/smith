package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"smith/internal/source/model"
	"smith/internal/source/store"
)

func TestSelectRetentionCandidates(t *testing.T) {
	now := time.Now().UTC()
	cfg := config{
		retentionFlatline:  2 * time.Hour,
		retentionCancelled: 4 * time.Hour,
		retentionSynced:    0,
	}
	states := []store.LoopWithRevision{
		{Record: model.StateRecord{LoopID: "new-flat", State: model.LoopStateFlatline, UpdatedAt: now.Add(-1 * time.Hour)}},
		{Record: model.StateRecord{LoopID: "old-flat", State: model.LoopStateFlatline, UpdatedAt: now.Add(-3 * time.Hour)}},
		{Record: model.StateRecord{LoopID: "old-cancel", State: model.LoopStateCancelled, UpdatedAt: now.Add(-5 * time.Hour)}},
		{Record: model.StateRecord{LoopID: "old-synced", State: model.LoopStateSynced, UpdatedAt: now.Add(-12 * time.Hour)}},
		{Record: model.StateRecord{LoopID: "old-running", State: model.LoopStateRunning, UpdatedAt: now.Add(-12 * time.Hour)}},
	}

	got := selectRetentionCandidates(states, now, cfg)
	if len(got) != 2 {
		t.Fatalf("expected 2 retention candidates, got %d", len(got))
	}
	if got[0].Record.LoopID != "old-cancel" || got[1].Record.LoopID != "old-flat" {
		t.Fatalf("unexpected candidate order: [%s, %s]", got[0].Record.LoopID, got[1].Record.LoopID)
	}
}

func TestRunCleanupPassDryRun(t *testing.T) {
	ms := store.NewMemStore()
	ctx := context.Background()

	putState(t, ms, "loop-flat", model.LoopStateFlatline)
	putState(t, ms, "loop-cancel", model.LoopStateCancelled)

	d := &daemon{cfg: config{
		cleanupDryRun:      true,
		cleanupMaxDeletes:  0,
		cleanupActor:       "smith-daemon",
		retentionFlatline:  1 * time.Millisecond,
		retentionCancelled: 1 * time.Millisecond,
	}, store: ms}

	time.Sleep(5 * time.Millisecond)
	stats, err := d.runCleanupPass(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("runCleanupPass returned error: %v", err)
	}
	if stats.Eligible != 2 || stats.Processed != 2 || stats.DryRun != 2 || stats.Deleted != 0 {
		t.Fatalf("unexpected cleanup stats: %+v", stats)
	}

	if _, found, _ := ms.GetState(ctx, "loop-flat"); !found {
		t.Fatalf("dry-run should not delete loop-flat")
	}
	if _, found, _ := ms.GetState(ctx, "loop-cancel"); !found {
		t.Fatalf("dry-run should not delete loop-cancel")
	}
}

func TestRunCleanupPassDeleteWithMaxLimit(t *testing.T) {
	ms := store.NewMemStore()
	ctx := context.Background()

	putState(t, ms, "loop-a", model.LoopStateFlatline)
	time.Sleep(2 * time.Millisecond)
	putState(t, ms, "loop-b", model.LoopStateCancelled)
	time.Sleep(2 * time.Millisecond)
	putState(t, ms, "loop-c", model.LoopStateFlatline)

	d := &daemon{cfg: config{
		cleanupDryRun:      false,
		cleanupMaxDeletes:  1,
		cleanupActor:       "smith-daemon",
		retentionFlatline:  1 * time.Millisecond,
		retentionCancelled: 1 * time.Millisecond,
	}, store: ms}

	time.Sleep(5 * time.Millisecond)
	stats, err := d.runCleanupPass(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("runCleanupPass returned error: %v", err)
	}
	if stats.Eligible != 3 || stats.Processed != 1 || stats.Deleted != 1 {
		t.Fatalf("unexpected cleanup stats: %+v", stats)
	}

	if _, found, _ := ms.GetState(ctx, "loop-a"); found {
		t.Fatalf("expected oldest loop to be deleted")
	}
	if _, found, _ := ms.GetState(ctx, "loop-b"); !found {
		t.Fatalf("expected loop-b to remain")
	}
	if _, found, _ := ms.GetState(ctx, "loop-c"); !found {
		t.Fatalf("expected loop-c to remain")
	}

	audits, err := ms.ListAudit(ctx, "", 0)
	if err != nil {
		t.Fatalf("ListAudit failed: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(audits))
	}
	if audits[0].Actor != "smith-daemon" || audits[0].Action != "delete-loop" {
		t.Fatalf("unexpected audit record: %+v", audits[0])
	}
}

func TestApplyRetentionPolicyFromFile(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	err := os.WriteFile(policyPath, []byte("retention:\n  flatline: 24h\n  cancelled: 12h\n  synced: 6h\n"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := config{
		policyPath:         policyPath,
		retentionFlatline:  48 * time.Hour,
		retentionCancelled: 48 * time.Hour,
		retentionSynced:    0,
	}
	if err := applyRetentionPolicyFromFile(&cfg); err != nil {
		t.Fatalf("applyRetentionPolicyFromFile failed: %v", err)
	}
	if cfg.retentionFlatline != 24*time.Hour {
		t.Fatalf("unexpected flatline retention: %s", cfg.retentionFlatline)
	}
	if cfg.retentionCancelled != 12*time.Hour {
		t.Fatalf("unexpected cancelled retention: %s", cfg.retentionCancelled)
	}
	if cfg.retentionSynced != 6*time.Hour {
		t.Fatalf("unexpected synced retention: %s", cfg.retentionSynced)
	}
}

func TestApplyRetentionPolicyFromFileInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	err := os.WriteFile(policyPath, []byte("retention:\n  flatline: nonsense\n"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := config{policyPath: policyPath}
	if err := applyRetentionPolicyFromFile(&cfg); err == nil {
		t.Fatal("expected parse error for invalid retention duration")
	}
}

func putState(t *testing.T, ms *store.MemStore, loopID string, state model.LoopState) {
	t.Helper()
	_, err := ms.PutState(context.Background(), model.StateRecord{
		LoopID:        loopID,
		State:         state,
		CorrelationID: "corr-" + loopID,
		SchemaVersion: model.SchemaVersion,
	}, 0)
	if err != nil {
		t.Fatalf("PutState failed for %s: %v", loopID, err)
	}
}
