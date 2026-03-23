package replica

import (
	"context"
	"fmt"
	"os"
	"strings"

	"smith/internal/source/model"
	"smith/internal/source/store"
)

type taskWorkflowClient struct {
	enabled   bool
	command   string
	taskID    string
	workspace string
	runner    execRunner
	warning   string
}

func newTaskWorkflowClient(anomaly model.Anomaly, workspace string, runner execRunner) taskWorkflowClient {
	taskID := ""
	if anomaly.Metadata != nil {
		taskID = strings.TrimSpace(anomaly.Metadata["task_contract_id"])
	}
	command := strings.TrimSpace(os.Getenv("SMITH_TASK_COMMAND"))
	if command == "" {
		command = "task"
	}
	enabled := parseBoolEnv(os.Getenv("SMITH_TASK_ENABLED"), true) && taskID != ""
	client := taskWorkflowClient{
		enabled:   enabled,
		command:   command,
		taskID:    taskID,
		workspace: workspace,
		runner:    runner,
	}
	if !enabled {
		return client
	}
	if _, err := lookPath(command); err != nil {
		client.enabled = false
		client.warning = fmt.Sprintf("task command not found: %s", command)
	}
	return client
}

func (c taskWorkflowClient) Warning() string {
	return c.warning
}

func (c taskWorkflowClient) Session(ctx context.Context) error {
	if !c.enabled {
		return nil
	}
	if _, err := c.runner.Run(ctx, c.workspace, c.command, "usage", "--new-session"); err != nil {
		return fmt.Errorf("task usage --new-session: %w", err)
	}
	return nil
}

func (c taskWorkflowClient) Start(ctx context.Context) error {
	if !c.enabled {
		return nil
	}
	_, err := c.runner.Run(ctx, c.workspace, c.command, "start", c.taskID)
	if err != nil {
		return fmt.Errorf("task start %s: %w", c.taskID, err)
	}
	return nil
}

func (c taskWorkflowClient) Log(ctx context.Context, message string) error {
	if !c.enabled {
		return nil
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return nil
	}
	_, err := c.runner.Run(ctx, c.workspace, c.command, "log", c.taskID, msg)
	if err != nil {
		return fmt.Errorf("task log %s: %w", c.taskID, err)
	}
	return nil
}

func (c taskWorkflowClient) Handoff(ctx context.Context, done []string, remaining []string) error {
	if !c.enabled {
		return nil
	}
	args := []string{"handoff", c.taskID}
	if joined := strings.Join(normalizeList(done), ","); joined != "" {
		args = append(args, "--done", joined)
	}
	if joined := strings.Join(normalizeList(remaining), ","); joined != "" {
		args = append(args, "--remaining", joined)
	}
	_, err := c.runner.Run(ctx, c.workspace, c.command, args...)
	if err != nil {
		return fmt.Errorf("task handoff %s: %w", c.taskID, err)
	}
	return nil
}

func normalizeList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func appendTaskWorkflowWarning(ctx context.Context, storeClient store.StateStore, loopID, correlationID string, err error) {
	if err == nil {
		return
	}
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "warn",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "task integration warning",
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"error": err.Error(),
		},
	})
}
