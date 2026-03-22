package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"smith/internal/source/model"
	"smith/internal/source/store"
)

const (
	defaultEtcdEndpoints = "http://127.0.0.1:2379"
	defaultDialTimeout   = 5 * time.Second
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stderr)
		return 2
	}
	if isHelp(args[0]) {
		printHelp(stdout)
		return 0
	}

	cmd := strings.ToLower(strings.TrimSpace(args[0]))
	switch cmd {
	case "usage":
		return runUsage(context.Background(), args[1:], stdout, stderr)
	case "start":
		return runStart(context.Background(), args[1:], stdout, stderr)
	case "log":
		return runLog(context.Background(), args[1:], stdout, stderr)
	case "handoff":
		return runHandoff(context.Background(), args[1:], stdout, stderr)
	case "review":
		return runSetStatus(context.Background(), args[1:], model.TaskContractStatusValidated, "IN_REVIEW", stdout, stderr)
	case "approve":
		return runSetStatus(context.Background(), args[1:], model.TaskContractStatusCompleted, "CLOSED", stdout, stderr)
	case "current":
		return runCurrent(context.Background(), stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printHelp(stderr)
		return 2
	}
}

func runUsage(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("usage", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var quiet bool
	var newSession bool
	fs.BoolVar(&quiet, "q", false, "quiet output")
	fs.BoolVar(&newSession, "new-session", false, "mark a fresh session")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}

	agent := sessionAgent()
	branch := sessionBranch()
	sessionID := sanitizeSessionID(agent + "@" + branch)
	running := []model.TaskContract{}
	if st, err := openStore(ctx); err == nil {
		defer func() { _ = st.Close() }()
		if tasks, taskErr := runningTasks(ctx, st); taskErr == nil {
			running = tasks
		} else if !quiet {
			fmt.Fprintf(stderr, "warning: list running tasks: %v\n", taskErr)
		}
	} else if !quiet {
		fmt.Fprintf(stderr, "warning: connect etcd: %v\n", err)
	}

	if quiet {
		fmt.Fprintf(stdout, "CURRENT SESSION: %s on branch: %s\n", sessionID, branch)
		if len(running) > 0 {
			fmt.Fprintf(stdout, "FOCUSED TASK: %s\n", running[0].ID)
		}
		return 0
	}

	fmt.Fprintf(stdout, "task session: %s\n", sessionID)
	if newSession {
		fmt.Fprintln(stdout, "NEW SESSION: initialized")
	}
	fmt.Fprintf(stdout, "agent: %s\n", agent)
	fmt.Fprintf(stdout, "branch: %s\n", branch)
	if len(running) == 0 {
		fmt.Fprintln(stdout, "running tasks: none")
		return 0
	}
	fmt.Fprintln(stdout, "running tasks:")
	for _, task := range running {
		fmt.Fprintf(stdout, "- %s\n", task.ID)
	}
	return 0
}

func runCurrent(ctx context.Context, stdout, stderr io.Writer) int {
	st, err := openStore(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "connect etcd: %v\n", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	running, err := runningTasks(ctx, st)
	if err != nil {
		fmt.Fprintf(stderr, "list running tasks: %v\n", err)
		return 1
	}
	if len(running) == 0 {
		fmt.Fprintln(stdout, "no running tasks")
		return 0
	}
	for _, task := range running {
		fmt.Fprintf(stdout, "%s\n", task.ID)
	}
	return 0
}

func runStart(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runSetStatus(ctx, args, model.TaskContractStatusRunning, "STARTED", stdout, stderr)
}

func runSetStatus(ctx context.Context, args []string, target model.TaskContractStatus, label string, stdout, stderr io.Writer) int {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(stderr, "task id is required")
		return 2
	}
	st, err := openStore(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "connect etcd: %v\n", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	taskID := strings.TrimSpace(args[0])
	task, found, err := st.GetTaskContract(ctx, taskID)
	if err != nil {
		fmt.Fprintf(stderr, "read task %s: %v\n", taskID, err)
		return 1
	}
	if !found {
		fmt.Fprintf(stderr, "task contract not found: %s\n", taskID)
		return 1
	}
	before := task.Status
	task.Status = target
	if task.Metadata == nil {
		task.Metadata = map[string]string{}
	}
	task.Metadata["smith_td_actor"] = sessionAgent()
	if err := st.PutTaskContract(ctx, task); err != nil {
		fmt.Fprintf(stderr, "write task %s: %v\n", taskID, err)
		return 1
	}
	_ = st.AppendJournal(ctx, model.JournalEntry{
		LoopID:    taskID,
		Phase:     "task",
		Level:     "info",
		ActorType: "operator",
		ActorID:   sessionAgent(),
		Message:   "task status updated",
		Metadata: map[string]string{
			"status_from": string(before),
			"status_to":   string(target),
		},
		CorrelationID: task.CorrelationID,
	})
	fmt.Fprintf(stdout, "%s %s\n", label, taskID)
	return 0
}

func runLog(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: task log <task-id> <message>")
		return 2
	}
	st, err := openStore(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "connect etcd: %v\n", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	taskID := strings.TrimSpace(args[0])
	message := strings.TrimSpace(strings.Join(args[1:], " "))
	if taskID == "" || message == "" {
		fmt.Fprintln(stderr, "task id and message are required")
		return 2
	}
	_ = st.AppendJournal(ctx, model.JournalEntry{
		LoopID:    taskID,
		Phase:     "task",
		Level:     "info",
		ActorType: "operator",
		ActorID:   sessionAgent(),
		Message:   message,
	})
	fmt.Fprintf(stdout, "LOGGED %s\n", taskID)
	return 0
}

func runHandoff(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(stderr, "task id is required")
		return 2
	}
	st, err := openStore(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "connect etcd: %v\n", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	taskID := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("handoff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var doneRaw string
	var remainingRaw string
	fs.StringVar(&doneRaw, "done", "", "comma-separated completed items")
	fs.StringVar(&remainingRaw, "remaining", "", "comma-separated remaining items")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	done := splitCSV(doneRaw)
	remaining := splitCSV(remainingRaw)

	summary := "task handoff submitted"
	if len(done) > 0 {
		summary = strings.Join(done, "; ")
	}
	next := "none"
	if len(remaining) > 0 {
		next = strings.Join(remaining, "; ")
	}
	_ = st.AppendHandoff(ctx, model.Handoff{
		LoopID:           taskID,
		FinalDiffSummary: summary,
		ValidationState:  "review_pending",
		NextSteps:        next,
		Metadata: map[string]string{
			"done_csv":      strings.Join(done, ","),
			"remaining_csv": strings.Join(remaining, ","),
			"actor":         sessionAgent(),
		},
	})
	_ = st.AppendJournal(ctx, model.JournalEntry{
		LoopID:    taskID,
		Phase:     "task",
		Level:     "info",
		ActorType: "operator",
		ActorID:   sessionAgent(),
		Message:   "task handoff submitted",
		Metadata: map[string]string{
			"done_items":      strings.Join(done, ","),
			"remaining_items": strings.Join(remaining, ","),
		},
	})
	fmt.Fprintf(stdout, "HANDOFF %s\n", taskID)
	return 0
}

func runningTasks(ctx context.Context, st *store.Store) ([]model.TaskContract, error) {
	all, err := st.ListTaskContracts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.TaskContract, 0, len(all))
	for _, task := range all {
		if task.Status == model.TaskContractStatusRunning {
			out = append(out, task)
		}
	}
	return out, nil
}

func openStore(ctx context.Context) (*store.Store, error) {
	endpoints := splitCSV(os.Getenv("SMITH_ETCD_ENDPOINTS"))
	if len(endpoints) == 0 {
		endpoints = []string{defaultEtcdEndpoints}
	}
	timeout := defaultDialTimeout
	if raw := strings.TrimSpace(os.Getenv("SMITH_ETCD_DIAL_TIMEOUT")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return nil, err
		}
		if parsed > 0 {
			timeout = parsed
		}
	}
	return store.New(ctx, endpoints, timeout)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func sessionAgent() string {
	if value := strings.TrimSpace(os.Getenv("SMITH_AGENT_TYPE")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("USER")); value != "" {
		return value
	}
	return "unknown-agent"
}

func sessionBranch() string {
	if value := strings.TrimSpace(os.Getenv("SMITH_GIT_BRANCH")); value != "" {
		return value
	}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "unknown"
	}
	return branch
}

func sanitizeSessionID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "session-unknown"
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "session-unknown"
	}
	return result
}

func isHelp(raw string) bool {
	value := strings.TrimSpace(raw)
	return value == "-h" || value == "--help" || strings.EqualFold(value, "help")
}

func printHelp(w io.Writer) {
	_, _ = io.WriteString(w, strings.TrimSpace(`task - Smith task workflow CLI

Usage:
  task usage [-q] [--new-session]
  task current
  task start <task-id>
  task log <task-id> <message>
  task handoff <task-id> --done a,b --remaining c,d
  task review <task-id>
  task approve <task-id>

Environment:
  SMITH_ETCD_ENDPOINTS (default: http://127.0.0.1:2379)
  SMITH_ETCD_DIAL_TIMEOUT (default: 5s)
  SMITH_AGENT_TYPE (optional session actor override)
`)+"\n")
}
