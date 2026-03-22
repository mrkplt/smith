package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"smith/internal/source/model"
)

type smithTDRunnerCall struct {
	Dir  string
	Name string
	Args []string
}

type smithTDRunner struct {
	calls []smithTDRunnerCall
	err   error
}

func (f *smithTDRunner) Run(_ context.Context, dir string, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, smithTDRunnerCall{Dir: dir, Name: name, Args: append([]string(nil), args...)})
	if f.err != nil {
		return nil, f.err
	}
	return []byte("ok"), nil
}

func TestNewTaskWorkflowClientEnabledWithTaskID(t *testing.T) {
	t.Setenv("SMITH_TASK_ENABLED", "true")
	t.Setenv("SMITH_TASK_COMMAND", "task")
	original := lookPath
	lookPath = func(_ string) (string, error) { return "/usr/local/bin/task", nil }
	t.Cleanup(func() { lookPath = original })

	client := newTaskWorkflowClient(model.Anomaly{Metadata: map[string]string{"task_contract_id": "task-123"}}, "/workspace", commandRunner{})
	if !client.enabled {
		t.Fatalf("expected task client enabled")
	}
	if client.taskID != "task-123" {
		t.Fatalf("unexpected task id: %q", client.taskID)
	}
	if client.command != "task" {
		t.Fatalf("unexpected command: %q", client.command)
	}
}

func TestNewTaskWorkflowClientDisabledWhenBinaryMissing(t *testing.T) {
	t.Setenv("SMITH_TASK_ENABLED", "true")
	t.Setenv("SMITH_TASK_COMMAND", "task")
	original := lookPath
	lookPath = func(_ string) (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { lookPath = original })

	client := newTaskWorkflowClient(model.Anomaly{Metadata: map[string]string{"task_contract_id": "task-123"}}, "/workspace", commandRunner{})
	if client.enabled {
		t.Fatalf("expected task client disabled when command is missing")
	}
	if client.Warning() == "" {
		t.Fatalf("expected missing command warning")
	}
}

func TestNewTaskWorkflowClientDisabledWithoutTaskID(t *testing.T) {
	t.Setenv("SMITH_TASK_ENABLED", "true")
	t.Setenv("SMITH_TASK_COMMAND", "task")
	original := lookPath
	lookPathCalled := false
	lookPath = func(_ string) (string, error) {
		lookPathCalled = true
		return "/usr/local/bin/task", nil
	}
	t.Cleanup(func() { lookPath = original })

	client := newTaskWorkflowClient(model.Anomaly{}, "/workspace", commandRunner{})
	if client.enabled {
		t.Fatalf("expected task client disabled when task id is missing")
	}
	if lookPathCalled {
		t.Fatalf("did not expect command lookup without task id")
	}
}

func TestNewTaskWorkflowClientUsesTaskDefaultCommand(t *testing.T) {
	t.Setenv("SMITH_TASK_ENABLED", "true")
	t.Setenv("SMITH_TASK_COMMAND", "")
	original := lookPath
	lookPath = func(cmd string) (string, error) {
		if cmd != "task" {
			t.Fatalf("expected default command task, got %q", cmd)
		}
		return "/usr/local/bin/task", nil
	}
	t.Cleanup(func() { lookPath = original })

	client := newTaskWorkflowClient(model.Anomaly{Metadata: map[string]string{"task_contract_id": "task-123"}}, "/workspace", commandRunner{})
	if client.command != "task" {
		t.Fatalf("expected default task command, got %q", client.command)
	}
}

func TestTaskWorkflowClientCommands(t *testing.T) {
	t.Setenv("SMITH_TASK_ENABLED", "true")
	t.Setenv("SMITH_TASK_COMMAND", "task")
	original := lookPath
	lookPath = func(_ string) (string, error) { return "/usr/local/bin/task", nil }
	t.Cleanup(func() { lookPath = original })

	runner := &smithTDRunner{}
	client := newTaskWorkflowClient(model.Anomaly{Metadata: map[string]string{"task_contract_id": "task-123"}}, "/workspace", runner)

	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if err := client.Log(context.Background(), "phase complete"); err != nil {
		t.Fatalf("log failed: %v", err)
	}
	if err := client.Handoff(context.Background(), []string{"done a", "done b"}, []string{"next a"}); err != nil {
		t.Fatalf("handoff failed: %v", err)
	}

	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(runner.calls))
	}
	if runner.calls[0].Name != "task" || !reflect.DeepEqual(runner.calls[0].Args, []string{"start", "task-123"}) {
		t.Fatalf("unexpected start call: %+v", runner.calls[0])
	}
	if runner.calls[1].Name != "task" || !reflect.DeepEqual(runner.calls[1].Args, []string{"log", "task-123", "phase complete"}) {
		t.Fatalf("unexpected log call: %+v", runner.calls[1])
	}
	if runner.calls[2].Name != "task" || !reflect.DeepEqual(runner.calls[2].Args, []string{"handoff", "task-123", "--done", "done a,done b", "--remaining", "next a"}) {
		t.Fatalf("unexpected handoff call: %+v", runner.calls[2])
	}
}

func TestTaskWorkflowClientCommandFailure(t *testing.T) {
	t.Setenv("SMITH_TASK_ENABLED", "true")
	t.Setenv("SMITH_TASK_COMMAND", "task")
	original := lookPath
	lookPath = func(_ string) (string, error) { return "/usr/local/bin/task", nil }
	t.Cleanup(func() { lookPath = original })

	runner := &smithTDRunner{err: errors.New("boom")}
	client := newTaskWorkflowClient(model.Anomaly{Metadata: map[string]string{"task_contract_id": "task-123"}}, "/workspace", runner)
	if err := client.Start(context.Background()); err == nil {
		t.Fatal("expected start error")
	}
}

func TestTaskWorkflowClientNoopWhenDisabled(t *testing.T) {
	t.Setenv("SMITH_TASK_ENABLED", "false")
	t.Setenv("SMITH_TASK_COMMAND", "task")
	original := lookPath
	lookPath = func(_ string) (string, error) { return "/usr/local/bin/task", nil }
	t.Cleanup(func() { lookPath = original })

	runner := &smithTDRunner{}
	client := newTaskWorkflowClient(model.Anomaly{Metadata: map[string]string{"task_contract_id": "task-123"}}, "/workspace", runner)
	if client.enabled {
		t.Fatalf("expected client disabled by SMITH_TASK_ENABLED=false")
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("expected no-op start, got %v", err)
	}
	if err := client.Log(context.Background(), "ignored"); err != nil {
		t.Fatalf("expected no-op log, got %v", err)
	}
	if err := client.Handoff(context.Background(), []string{"done"}, []string{"next"}); err != nil {
		t.Fatalf("expected no-op handoff, got %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected no task commands for disabled client, got %+v", runner.calls)
	}
}
