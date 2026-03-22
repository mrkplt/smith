package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"smith/internal/source/completion"
	"smith/internal/source/model"
	"smith/internal/source/store"
)

const (
	defaultLoopMaxIterations = 25
	defaultLoopIterationWait = 2 * time.Second
	defaultCodexCLICommand   = "codex exec --yolo --skip-git-repo-check -"
	defaultPRDPath           = ".agents/tasks/prd.json"
	defaultPRDStoryCount     = 5
	defaultInteractivePRD    = true
	defaultInteractiveWait   = 0 * time.Second
	defaultInteractivePoll   = 2 * time.Second
	defaultIssuePRDPrompt    = ".smith/prompts/issue-prd.md"
)

type handoffFile struct {
	LoopID string `json:"loop_id"`
}

type startupContext struct {
	Anomaly      model.Anomaly
	PriorHandoff *model.Handoff
}

type loopExecutionConfig struct {
	ProviderID           string
	Model                string
	InvocationMethod     string
	SourceType           string
	SourceRef            string
	Stage                string
	MaxIterations        int
	IterationWait        time.Duration
	CodexCommand         string
	PRDPath              string
	PRDStoryCount        int
	InteractivePRD       bool
	InteractivePRDWait   time.Duration
	InteractivePRDPoll   time.Duration
	IssueWorkflowEnabled bool
}

func main() {

	loopID := strings.TrimSpace(os.Getenv("SMITH_LOOP_ID"))
	if loopID == "" {
		log.Fatal("SMITH_LOOP_ID is required")
	}
	correlationID := strings.TrimSpace(os.Getenv("SMITH_CORRELATION_ID"))

	handoffPath := strings.TrimSpace(os.Getenv("SMITH_HANDOFF_PATH"))
	if handoffPath == "" {
		handoffPath = "/smith/handoff/latest.json"
	}

	ctx := context.Background()
	storeClient, err := store.New(ctx, splitCSV(os.Getenv("SMITH_ETCD_ENDPOINTS")), 5*time.Second)
	if err != nil {
		log.Fatalf("failed to connect etcd: %v", err)
	}
	defer func() { _ = storeClient.Close() }()

	loadedFromFile, fileErr := readHandoffFile(handoffPath)
	if fileErr != nil {
		log.Printf("failed to parse handoff file at %s: %v", handoffPath, fileErr)
	} else if loadedFromFile != nil {
		log.Printf("loaded mounted handoff for loop_id=%s", loadedFromFile.LoopID)
	} else {
		log.Printf("handoff not found at %s; continuing", handoffPath)
	}

	startup, startupErr := loadStartupContext(ctx, storeClient, loopID)
	if startupErr != nil {
		recordStartupFailure(ctx, storeClient, loopID, correlationID, startupErr)
		log.Fatalf("startup context load failed: %v", startupErr)
	}
	if startup.PriorHandoff != nil {
		log.Printf("loaded prior handoff sequence=%d for loop_id=%s", startup.PriorHandoff.Sequence, loopID)
	} else {
		log.Printf("no prior handoff found in etcd for loop_id=%s; treating as first-run", loopID)
	}

	workspace := strings.TrimSpace(os.Getenv("SMITH_WORKSPACE"))
	if workspace == "" {
		workspace = "/workspace"
	}
	runner := commandRunner{}
	taskClient := newTaskWorkflowClient(startup.Anomaly, workspace, runner)
	if warning := strings.TrimSpace(taskClient.Warning()); warning != "" {
		appendTaskWorkflowWarning(ctx, storeClient, loopID, correlationID, errors.New(warning))
	}
	appendTaskWorkflowWarning(ctx, storeClient, loopID, correlationID, taskClient.Start(ctx))
	appendTaskWorkflowWarning(ctx, storeClient, loopID, correlationID, taskClient.Log(ctx, "replica execution starting"))
	loopCfg := loadLoopExecutionConfigFromEnv()
	envMeta, setupErr := setupLoopEnvironment(ctx, startup.Anomaly.Environment, workspace, runner)
	if setupErr != nil {
		appendTaskWorkflowWarning(ctx, storeClient, loopID, correlationID, taskClient.Log(ctx, "environment setup failed: "+setupErr.Error()))
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "environment",
			Level:         "error",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "loop environment setup failed",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"error": setupErr.Error(),
			},
		})
		_, _ = storeClient.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
			if current.State == model.LoopStateSynced || current.State == model.LoopStateFlatline || current.State == model.LoopStateCancelled {
				return current, nil
			}
			current.State = model.LoopStateFlatline
			current.Reason = "environment-setup-failed"
			return current, nil
		})
		syncTaskContractStatusFromAnomaly(ctx, storeClient, startup.Anomaly, model.LoopStateFlatline, "environment-setup-failed")
		log.Fatalf("environment setup failed: %v", setupErr)
	}
	loopMeta := loopExecutionMetadata(loopCfg)
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "environment",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "loop environment resolved",
		CorrelationID: correlationID,
		Metadata:      mergeMetadata(envMeta, loopMeta),
	})
	runtimeMeta := mergeMetadata(runtimeMetadataFromEnv(), loopMeta)

	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "replica execution started",
		CorrelationID: correlationID,
		Metadata:      runtimeMeta,
	})

	desiredState, finalizeReason, runErr := runLoopIterations(
		ctx,
		storeClient,
		loopID,
		correlationID,
		loopCfg,
		startup.Anomaly,
		workspace,
		runner,
	)
	if runErr != nil {
		appendTaskWorkflowWarning(ctx, storeClient, loopID, correlationID, taskClient.Log(ctx, "replica runtime failed: "+runErr.Error()))
		recordRuntimeFailure(ctx, storeClient, loopID, correlationID, runErr)
		syncTaskContractStatusFromAnomaly(ctx, storeClient, startup.Anomaly, model.LoopStateFlatline, "replica-runtime-failed")
		log.Fatalf("replica loop failed: %v", runErr)
	}

	finalState := desiredState
	var finalizeErr error

	if desiredState == model.LoopStateSynced {
		gitPAT := strings.TrimSpace(os.Getenv("SMITH_GIT_PAT"))
		gitRepo := strings.TrimSpace(os.Getenv("SMITH_GIT_REPOSITORY"))
		gitBranch := strings.TrimSpace(os.Getenv("SMITH_GIT_BRANCH"))
		gitUserName := strings.TrimSpace(os.Getenv("SMITH_GIT_USER_NAME"))
		gitUserEmail := strings.TrimSpace(os.Getenv("SMITH_GIT_USER_EMAIL"))
		createPR := parseBoolEnv(os.Getenv("SMITH_GIT_CREATE_PR"), true)
		completionBranch := resolveCompletionGitBranch(gitBranch, loopID, createPR)

		protocol := completion.NewProtocol(storeClient, &completion.RealGit{
			Workspace:  workspace,
			Repository: gitRepo,
			Branch:     completionBranch,
			PAT:        gitPAT,
			UserName:   gitUserName,
			UserEmail:  gitUserEmail,
		})

		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "completion",
			Level:         "info",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "starting completion saga (commit + sync)",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"repository":    gitRepo,
				"branch":        completionBranch,
				"target_branch": normalizeGitBranch(gitBranch),
			},
		})

		result, err := protocol.Execute(ctx, completion.CommitRequest{
			LoopID:        loopID,
			CorrelationID: correlationID,
			FinalDiff:     "autonomous completion",
			PullRequest:   createPR,
			PRTitle:       "chore(loop): autonomous update for loop " + loopID,
			PRBody:        "This Pull Request was autonomously generated by Smith for loop " + loopID + ".",
		})

		if err != nil {
			finalizeErr = err
			finalState = model.LoopStateFlatline
			finalizeReason = "completion-saga-failed"
		} else {
			finalState = model.LoopStateSynced
			finalizeReason = "completion-saga-succeeded"
			log.Printf("completion saga succeeded: commit=%s", result.CommitSHA)
		}
	} else {
		finalState, finalizeErr = finalizeLoopState(ctx, storeClient, loopID, desiredState, finalizeReason)
	}

	if finalizeErr != nil {
		appendTaskWorkflowWarning(ctx, storeClient, loopID, correlationID, taskClient.Log(ctx, "replica finalize failed: "+finalizeErr.Error()))
		recordRuntimeFailure(ctx, storeClient, loopID, correlationID, finalizeErr)
		syncTaskContractStatusFromAnomaly(ctx, storeClient, startup.Anomaly, model.LoopStateFlatline, "replica-runtime-failed")
		log.Fatalf("failed to finalize state: %v", finalizeErr)
	}
	syncTaskContractStatusFromAnomaly(ctx, storeClient, startup.Anomaly, finalState, finalizeReason)

	handoffMetadata := map[string]string{
		"executor": hostnameOr("smith-replica"),
	}
	for k, v := range runtimeMeta {
		handoffMetadata[k] = v
	}

	handoffSummary := "replica completed autonomous cycle"
	validationState := "passed"
	nextSteps := "operator review optional"
	if finalState == model.LoopStateCancelled {
		handoffSummary = "replica loop cancelled by operator"
		validationState = "cancelled"
		nextSteps = "loop cancelled; rerun if additional work is needed"
	} else if finalState == model.LoopStateFlatline {
		handoffSummary = "replica loop terminated"
		validationState = "failed"
		nextSteps = "investigate runtime failure and retry loop"
	}
	handoffMetadata["final_state"] = string(finalState)

	_ = storeClient.AppendHandoff(ctx, model.Handoff{
		LoopID:           loopID,
		FinalDiffSummary: handoffSummary,
		ValidationState:  validationState,
		NextSteps:        nextSteps,
		CorrelationID:    correlationID,
		Metadata:         handoffMetadata,
	})
	appendTaskWorkflowWarning(ctx, storeClient, loopID, correlationID, taskClient.Handoff(ctx, []string{handoffSummary}, []string{nextSteps}))

	finalMessage := "replica execution completed"
	if finalState == model.LoopStateCancelled {
		finalMessage = "replica execution cancelled"
	} else if finalState == model.LoopStateFlatline {
		finalMessage = "replica execution terminated"
	}
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       finalMessage,
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"final_state":  string(finalState),
			"final_reason": finalizeReason,
			"token_total":  "0",
			"token_prompt": "0",
			"token_output": "0",
			"cost_usd":     "0",
		},
	})
	appendTaskWorkflowWarning(ctx, storeClient, loopID, correlationID, taskClient.Log(ctx, finalMessage))

	log.Printf("smith-replica startup complete for loop_id=%s final_state=%s", loopID, finalState)
}

func loadLoopExecutionConfigFromEnv() loopExecutionConfig {
	providerID := strings.ToLower(strings.TrimSpace(os.Getenv("SMITH_LOOP_PROVIDER")))
	if providerID == "" {
		providerID = model.DefaultProviderID
	}
	modelID := strings.TrimSpace(os.Getenv("SMITH_LOOP_MODEL"))
	method := normalizeInvocationMethod(os.Getenv("SMITH_LOOP_INVOCATION_METHOD"))
	sourceType := strings.TrimSpace(os.Getenv("SMITH_LOOP_SOURCE_TYPE"))
	sourceRef := strings.TrimSpace(os.Getenv("SMITH_LOOP_SOURCE_REF"))
	stage := strings.TrimSpace(os.Getenv("SMITH_LOOP_STAGE"))
	maxIterations, iterationWait := defaultLoopProfileForMethod(method)
	codexCommand := resolveAgentCommand(providerID)
	prdPath := strings.TrimSpace(os.Getenv("SMITH_LOOP_PRD_PATH"))
	if prdPath == "" {
		prdPath = defaultPRDPath
	}
	prdStoryCount := defaultPRDStoryCount
	if raw := strings.TrimSpace(os.Getenv("SMITH_LOOP_PRD_STORY_COUNT")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			prdStoryCount = parsed
		}
	}
	interactivePRD := parseBoolEnv(os.Getenv("SMITH_ISSUE_PRD_INTERACTIVE"), defaultInteractivePRD)
	interactiveWait := defaultInteractiveWait
	if raw := strings.TrimSpace(os.Getenv("SMITH_ISSUE_PRD_INTERACTIVE_WAIT")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed >= 0 {
			interactiveWait = parsed
		}
	}
	interactivePoll := defaultInteractivePoll
	if raw := strings.TrimSpace(os.Getenv("SMITH_ISSUE_PRD_INTERACTIVE_POLL")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			interactivePoll = parsed
		}
	}
	issueWorkflowEnabled := parseBoolEnv(os.Getenv("SMITH_ISSUE_WORKFLOW_ENABLED"), true)
	cfg := loopExecutionConfig{
		ProviderID:           providerID,
		Model:                modelID,
		InvocationMethod:     method,
		SourceType:           sourceType,
		SourceRef:            sourceRef,
		Stage:                stage,
		MaxIterations:        maxIterations,
		IterationWait:        iterationWait,
		CodexCommand:         codexCommand,
		PRDPath:              prdPath,
		PRDStoryCount:        prdStoryCount,
		InteractivePRD:       interactivePRD,
		InteractivePRDWait:   interactiveWait,
		InteractivePRDPoll:   interactivePoll,
		IssueWorkflowEnabled: issueWorkflowEnabled,
	}

	if raw := strings.TrimSpace(os.Getenv("SMITH_LOOP_MAX_ITERATIONS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			cfg.MaxIterations = parsed
		}
	}
	if raw := strings.TrimSpace(os.Getenv("SMITH_LOOP_ITERATION_WAIT")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			cfg.IterationWait = parsed
		}
	}
	return cfg
}

func loopExecutionMetadata(cfg loopExecutionConfig) map[string]string {
	return map[string]string{
		"loop_provider":          cfg.ProviderID,
		"loop_model":             cfg.Model,
		"loop_invocation_method": cfg.InvocationMethod,
		"loop_stage":             cfg.Stage,
		"loop_source_type":       cfg.SourceType,
		"loop_source_ref":        cfg.SourceRef,
		"loop_max_iterations":    strconv.Itoa(cfg.MaxIterations),
		"loop_iteration_wait":    cfg.IterationWait.String(),
		"loop_prd_path":          cfg.PRDPath,
		"loop_prd_story_count":   strconv.Itoa(cfg.PRDStoryCount),
		"loop_codex_command":     cfg.CodexCommand,
		"loop_agent_command":     cfg.CodexCommand,
		"loop_prd_interactive":   strconv.FormatBool(cfg.InteractivePRD),
		"loop_prd_wait":          interactiveWaitLabel(cfg.InteractivePRDWait),
		"loop_prd_poll":          cfg.InteractivePRDPoll.String(),
	}
}
func runLoopIterations(
	ctx context.Context,
	storeClient store.StateStore,
	loopID, correlationID string,
	cfg loopExecutionConfig,
	anomaly model.Anomaly,
	workspace string,
	runner execRunner,
) (model.LoopState, string, error) {
	if shouldRunIssueWorkflow(cfg, anomaly) {
		return runIssueWorkflow(ctx, storeClient, loopID, correlationID, cfg, anomaly, workspace, runner)
	}
	return runIterativeLoop(ctx, storeClient, loopID, correlationID, cfg)
}

func runIterativeLoop(ctx context.Context, storeClient store.StateStore, loopID, correlationID string, cfg loopExecutionConfig) (model.LoopState, string, error) {
	for iteration := 1; iteration <= cfg.MaxIterations; iteration++ {
		state, found, err := storeClient.GetState(ctx, loopID)
		if err != nil {
			return model.LoopStateFlatline, "state-read-failed", err
		}
		if !found {
			return model.LoopStateFlatline, "state-missing", fmt.Errorf("state not found for loop_id=%s", loopID)
		}

		switch state.Record.State {
		case model.LoopStateCancelled:
			_ = storeClient.AppendJournal(ctx, model.JournalEntry{
				LoopID:        loopID,
				Phase:         "replica",
				Level:         "warn",
				ActorType:     "replica",
				ActorID:       hostnameOr("smith-replica"),
				Message:       "replica loop observed cancellation",
				CorrelationID: correlationID,
				Metadata: map[string]string{
					"iteration": strconv.Itoa(iteration),
				},
			})
			return model.LoopStateCancelled, "operator-cancelled", nil
		case model.LoopStateFlatline:
			_ = storeClient.AppendJournal(ctx, model.JournalEntry{
				LoopID:        loopID,
				Phase:         "replica",
				Level:         "warn",
				ActorType:     "replica",
				ActorID:       hostnameOr("smith-replica"),
				Message:       "replica loop observed termination",
				CorrelationID: correlationID,
				Metadata: map[string]string{
					"iteration": strconv.Itoa(iteration),
				},
			})
			return model.LoopStateFlatline, "operator-terminated", nil
		case model.LoopStateSynced:
			return model.LoopStateSynced, "already-synced", nil
		case model.LoopStateRunning:
			// Continue below.
		default:
			return state.Record.State, "loop-not-active", nil
		}

		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "info",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       fmt.Sprintf("replica iteration %d/%d", iteration, cfg.MaxIterations),
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"iteration":              strconv.Itoa(iteration),
				"max_iterations":         strconv.Itoa(cfg.MaxIterations),
				"iteration_wait":         cfg.IterationWait.String(),
				"workflow_profile":       "upstream-tooling-like",
				"loop_invocation_method": cfg.InvocationMethod,
			},
		})

		if iteration >= cfg.MaxIterations {
			break
		}
		select {
		case <-ctx.Done():
			return model.LoopStateFlatline, "replica-context-cancelled", ctx.Err()
		case <-time.After(cfg.IterationWait):
		}
	}

	return model.LoopStateSynced, "replica-iterations-complete", nil
}

func shouldRunIssueWorkflow(cfg loopExecutionConfig, anomaly model.Anomaly) bool {
	if !cfg.IssueWorkflowEnabled {
		return false
	}
	method := normalizeInvocationMethod(cfg.InvocationMethod)
	sourceType := strings.ToLower(strings.TrimSpace(cfg.SourceType))
	if sourceType == "" {
		sourceType = strings.ToLower(strings.TrimSpace(anomaly.SourceType))
	}
	metadataPrompt := ""
	if anomaly.Metadata != nil {
		metadataPrompt = strings.TrimSpace(anomaly.Metadata["workspace_prompt"])
	}
	if sourceType == "github_issue" || sourceType == "prompt" || sourceType == "interactive_prompt" || sourceType == "prd" || sourceType == "prd_story" {
		return true
	}
	if metadataPrompt != "" {
		return true
	}
	switch method {
	case "github_issue", "issue", "issue_based", "prompt", "interactive_prompt", "generate_prd", "prd", "prd_story":
		return true
	default:
		return false
	}
}

func runIssueWorkflow(
	ctx context.Context,
	storeClient store.StateStore,
	loopID, correlationID string,
	cfg loopExecutionConfig,
	anomaly model.Anomaly,
	workspace string,
	runner execRunner,
) (model.LoopState, string, error) {
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return model.LoopStateFlatline, "workspace-create-failed", err
	}
	repository := strings.TrimSpace(os.Getenv("SMITH_GIT_REPOSITORY"))
	branch := strings.TrimSpace(os.Getenv("SMITH_GIT_BRANCH"))
	gitPAT := strings.TrimSpace(os.Getenv("SMITH_GIT_PAT"))
	gitDirPresent, err := ensureGitWorkspace(ctx, runner, workspace, repository, branch, gitPAT)
	if err != nil {
		return model.LoopStateFlatline, "workspace-clone-failed", err
	}
	appendWorkflowStep(ctx, storeClient, loopID, correlationID, "clone", "repository workspace prepared", map[string]string{
		"workspace":       workspace,
		"git_dir_present": strconv.FormatBool(gitDirPresent),
		"repository":      repository,
		"branch":          normalizeGitBranch(branch),
	})

	taskContract, validationCommands, err := resolveTaskContractAndValidationCommands(ctx, storeClient, anomaly)
	if err != nil {
		return model.LoopStateFlatline, "task-contract-read-failed", err
	}
	if taskContract != nil {
		appendWorkflowStep(ctx, storeClient, loopID, correlationID, "read-task", "task contract loaded", map[string]string{
			"task_contract_id": taskContract.ID,
			"task_status":      string(taskContract.Status),
			"validation_count": strconv.Itoa(len(validationCommands)),
		})
	} else {
		appendWorkflowStep(ctx, storeClient, loopID, correlationID, "read-task", "task contract not bound; continuing with metadata defaults", map[string]string{
			"validation_count": strconv.Itoa(len(validationCommands)),
		})
	}
	if err := ensureCodexLogin(ctx, runner, workspace, cfg, loopID, correlationID, storeClient); err != nil {
		return model.LoopStateFlatline, "provider-auth-setup-failed", err
	}
	prdPath := cfg.PRDPath
	if anomaly.Metadata != nil {
		if configured := strings.TrimSpace(anomaly.Metadata["workspace_prd_path"]); configured != "" {
			prdPath = configured
		}
	}
	if !filepath.IsAbs(prdPath) {
		prdPath = filepath.Join(workspace, prdPath)
	}
	if err := os.MkdirAll(filepath.Dir(prdPath), 0o755); err != nil {
		return model.LoopStateFlatline, "prd-path-create-failed", err
	}
	if written, writeErr := materializePRDFromMetadata(prdPath, anomaly.Metadata); writeErr != nil {
		return model.LoopStateFlatline, "issue-prd-materialize-failed", writeErr
	} else if written {
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "info",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "PRD materialized from loop metadata",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"prd_path": prdPath,
			},
		})
	}

	prdStoryCount := expectedPRDStoryCount(cfg.PRDStoryCount, anomaly)
	resolvedPRDPath, resolveSource, resolveErr := resolveIssuePRDPath(prdPath)
	if resolveErr == nil {
		prdPath = resolvedPRDPath
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "info",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "existing PRD detected; skipping PRD generation phase",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"prd_path":          prdPath,
				"prd_story_count":   strconv.Itoa(prdStoryCount),
				"resolution_source": resolveSource,
			},
		})
	} else if !errors.Is(resolveErr, os.ErrNotExist) {
		return model.LoopStateFlatline, "issue-prd-check-failed", resolveErr
	}

	if state, reason, done, err := loopTerminalDecision(ctx, storeClient, loopID); err != nil {
		return model.LoopStateFlatline, "state-read-failed", err
	} else if done {
		return state, reason, nil
	}

	if errors.Is(resolveErr, os.ErrNotExist) {
		prdPrompt := buildIssuePRDPrompt(anomaly, prdPath, prdStoryCount)
		interactivePromptPath := ""
		interactiveCommandHint := ""
		interactivePRDEnabled := shouldUseInteractivePRDGate(cfg, anomaly)
		if cfg.InteractivePRD && !interactivePRDEnabled {
			appendWorkflowStep(ctx, storeClient, loopID, correlationID, "prd", "interactive PRD gate skipped for autonomous PRD ingress", map[string]string{
				"source_type":      strings.TrimSpace(anomaly.SourceType),
				"loop_source_type": strings.TrimSpace(cfg.SourceType),
				"ingress_mode":     strings.TrimSpace(anomaly.Metadata["ingress_mode"]),
			})
		}
		if interactivePRDEnabled {
			promptPath := filepath.Join(workspace, defaultIssuePRDPrompt)
			if err := writePromptFileAtPath(promptPath, prdPrompt); err != nil {
				return model.LoopStateFlatline, "issue-prd-prompt-prepare-failed", err
			}
			interactivePromptPath = promptPath
			interactiveCommandHint = interactivePRDCommandHint(promptPath, prdPath, prdStoryCount, cfg.CodexCommand)
		}

		if interactivePRDEnabled {
			resolvedInteractivePath, waitReason, err := waitForInteractivePRD(ctx, storeClient, loopID, correlationID, cfg, prdPath, interactivePromptPath, interactiveCommandHint)
			if err != nil {
				return model.LoopStateFlatline, waitReason, err
			}
			if strings.TrimSpace(resolvedInteractivePath) != "" {
				prdPath = resolvedInteractivePath
			}
			if waitReason == "operator-cancelled" {
				return model.LoopStateCancelled, waitReason, nil
			}
			if waitReason == "operator-terminated" {
				return model.LoopStateFlatline, waitReason, nil
			}
			if waitReason == "already-synced" {
				return model.LoopStateSynced, waitReason, nil
			}
			if waitReason == "loop-not-active" {
				return model.LoopStateFlatline, waitReason, nil
			}
		}

		resolvedPRDPath, _, resolveErr = resolveIssuePRDPath(prdPath)
		if errors.Is(resolveErr, os.ErrNotExist) {
			if err := runCodexStep(ctx, runner, workspace, cfg.CodexCommand, "prd", prdPrompt, loopID, correlationID, storeClient); err != nil {
				return model.LoopStateFlatline, "issue-prd-step-failed", err
			}
		} else if resolveErr != nil {
			return model.LoopStateFlatline, "issue-prd-check-failed", resolveErr
		} else {
			prdPath = resolvedPRDPath
		}
	}

	resolvedPRDPath, resolveSource, resolveErr = resolveIssuePRDPath(prdPath)
	if resolveErr != nil {
		if errors.Is(resolveErr, os.ErrNotExist) {
			return model.LoopStateFlatline, "issue-prd-missing", fmt.Errorf("prd file not found after prd step: %s", prdPath)
		}
		return model.LoopStateFlatline, "issue-prd-check-failed", resolveErr
	}
	if strings.TrimSpace(resolvedPRDPath) != "" && resolvedPRDPath != prdPath {
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "info",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "PRD auto-detected from tasks directory",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"expected_prd_path": prdPath,
				"resolved_prd_path": resolvedPRDPath,
				"resolution_source": resolveSource,
			},
		})
	}
	prdPath = resolvedPRDPath

	// Sync PRD with etcd for durability
	if err := syncDocumentWithEtcd(ctx, storeClient, "prd-"+loopID, prdPath, loopID, correlationID); err != nil {
		log.Printf("failed to sync PRD with etcd: %v", err)
	}

	if err := ensurePRDStoryCount(ctx, storeClient, loopID, correlationID, prdPath, prdStoryCount); err != nil {
		return model.LoopStateFlatline, "issue-prd-story-count-invalid", err
	}

	if cfg.Stage == "prd" {
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "info",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "PRD generation stage complete; stopping as requested",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"stage":    cfg.Stage,
				"prd_path": prdPath,
			},
		})
		return model.LoopStateSynced, "stage-prd-complete", nil
	}

	if state, reason, done, err := loopTerminalDecision(ctx, storeClient, loopID); err != nil {
		return model.LoopStateFlatline, "state-read-failed", err
	} else if done {
		return state, reason, nil
	}

	appendWorkflowStep(ctx, storeClient, loopID, correlationID, "plan", "starting planning stage", map[string]string{
		"stage": cfg.Stage,
	})
	techSpecPath, implementationPlanPath, err := runTechPlanning(ctx, storeClient, loopID, correlationID, cfg, anomaly, workspace, runner, prdPath)
	if err != nil {
		return model.LoopStateFlatline, "tech-planning-failed", err
	}
	appendWorkflowStep(ctx, storeClient, loopID, correlationID, "plan", "planning stage complete", map[string]string{
		"tech_spec_path":      techSpecPath,
		"implementation_plan": implementationPlanPath,
	})

	if cfg.Stage == "tech_planning" || cfg.Stage == "planning" {
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "info",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "technical planning stage complete; stopping as requested",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"stage":               cfg.Stage,
				"tech_spec_path":      techSpecPath,
				"implementation_plan": implementationPlanPath,
			},
		})
		return model.LoopStateSynced, "stage-tech-planning-complete", nil
	}

	if state, reason, done, err := loopTerminalDecision(ctx, storeClient, loopID); err != nil {
		return model.LoopStateFlatline, "state-read-failed", err
	} else if done {
		return state, reason, nil
	}

	appendWorkflowStep(ctx, storeClient, loopID, correlationID, "edit", "starting implementation stage", map[string]string{
		"implementation_plan": implementationPlanPath,
	})
	buildState, buildReason, buildErr := runIssueBuildIterations(ctx, storeClient, loopID, correlationID, cfg, anomaly, workspace, runner, prdPath, implementationPlanPath, validationCommands)
	if buildErr != nil {
		return buildState, buildReason, buildErr
	}
	if buildState != model.LoopStateSynced {
		return buildState, buildReason, nil
	}
	appendWorkflowStep(ctx, storeClient, loopID, correlationID, "edit", "implementation stage complete", map[string]string{
		"build_reason": buildReason,
	})

	if err := runTaskValidationCommands(ctx, runner, workspace, loopID, correlationID, storeClient, validationCommands); err != nil {
		return model.LoopStateFlatline, "task-validation-failed", err
	}

	return model.LoopStateSynced, buildReason, nil
}

func ensureGitWorkspace(ctx context.Context, runner execRunner, workspace, repository, branch, gitPAT string) (bool, error) {
	if stat, err := os.Stat(filepath.Join(workspace, ".git")); err == nil && stat.IsDir() {
		return true, nil
	}
	repo := strings.TrimSpace(repository)
	if repo == "" || strings.EqualFold(repo, "unknown") {
		return false, errors.New("git repository is required to prepare workspace")
	}
	branchName := normalizeGitBranch(branch)
	fetchURL := repo
	if strings.TrimSpace(gitPAT) != "" && strings.HasPrefix(repo, "https://") {
		fetchURL = "https://" + strings.TrimSpace(gitPAT) + "@" + strings.TrimPrefix(repo, "https://")
	}
	if output, err := runner.Run(ctx, workspace, "git", "config", "--global", "--add", "safe.directory", workspace); err != nil {
		return false, formatCommandError("git safe.directory config failed", output, err)
	}

	if output, err := runner.Run(ctx, workspace, "git", "init"); err != nil {
		return false, formatCommandError("git init failed", output, err)
	}
	_, _ = runner.Run(ctx, workspace, "git", "remote", "remove", "origin")
	if output, err := runner.Run(ctx, workspace, "git", "remote", "add", "origin", repo); err != nil {
		if setOutput, setErr := runner.Run(ctx, workspace, "git", "remote", "set-url", "origin", repo); setErr != nil {
			return false, formatCommandError("git remote add failed", output, err)
		} else if strings.TrimSpace(string(setOutput)) != "" {
			log.Printf("git remote set-url output: %s", strings.TrimSpace(string(setOutput)))
		}
	}
	if output, err := runner.Run(ctx, workspace, "git", "fetch", "--depth", "1", fetchURL, branchName); err != nil {
		return false, formatCommandError("git fetch failed", output, err)
	}
	if output, err := runner.Run(ctx, workspace, "git", "checkout", "-B", branchName, "FETCH_HEAD"); err != nil {
		return false, formatCommandError("git checkout failed", output, err)
	}
	return true, nil
}

func formatCommandError(prefix string, output []byte, err error) error {
	if err == nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	return fmt.Errorf("%s: %w (output: %s)", prefix, err, trimmed)
}

func normalizeGitBranch(branch string) string {
	value := strings.TrimSpace(branch)
	if value == "" || strings.EqualFold(value, "unknown") {
		return "main"
	}
	return value
}

func resolveCompletionGitBranch(branch, loopID string, createPR bool) string {
	baseBranch := normalizeGitBranch(branch)
	if !createPR {
		return baseBranch
	}
	lower := strings.ToLower(strings.TrimSpace(baseBranch))
	if lower != "main" && lower != "master" {
		return baseBranch
	}
	suffix := sanitizeGitBranchToken(loopID)
	if suffix == "" {
		return "smith-loop"
	}
	return "smith-loop-" + suffix
}

func sanitizeGitBranchToken(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	lastDash := false
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	sanitized := strings.Trim(builder.String(), "-")
	if len(sanitized) > 56 {
		sanitized = strings.Trim(sanitized[:56], "-")
	}
	return sanitized
}

func shouldUseInteractivePRDGate(cfg loopExecutionConfig, anomaly model.Anomaly) bool {
	if !cfg.InteractivePRD {
		return false
	}
	sourceType := strings.ToLower(strings.TrimSpace(cfg.SourceType))
	if sourceType == "" {
		sourceType = strings.ToLower(strings.TrimSpace(anomaly.SourceType))
	}
	if sourceType == "prd" || sourceType == "prd_story" {
		return false
	}
	method := strings.ToLower(strings.TrimSpace(cfg.InvocationMethod))
	if method == "prd" || method == "prd_story" {
		return false
	}
	ingressMode := ""
	if anomaly.Metadata != nil {
		ingressMode = strings.ToLower(strings.TrimSpace(anomaly.Metadata["ingress_mode"]))
	}
	if ingressMode == "prd" {
		return false
	}
	return true
}

func ensureCodexLogin(
	ctx context.Context,
	runner execRunner,
	workspace string,
	cfg loopExecutionConfig,
	loopID, correlationID string,
	storeClient store.StateStore,
) error {
	if !shouldPrimeCodexLogin(cfg) {
		return nil
	}
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" && strings.TrimSpace(os.Getenv("SMITH_RUNTIME_CREDENTIALS")) == "" {
		appendWorkflowStep(ctx, storeClient, loopID, correlationID, "auth", "codex login skipped: runtime credential missing", nil)
		return nil
	}
	output, err := runner.Run(ctx, workspace, "sh", "-lc", "if [ -z \"$OPENAI_API_KEY\" ]; then export OPENAI_API_KEY=\"$SMITH_RUNTIME_CREDENTIALS\"; fi; if [ -n \"$OPENAI_API_KEY\" ]; then printf '%s' \"$OPENAI_API_KEY\" | codex login --with-api-key; fi")
	if err != nil {
		return formatCommandError("codex login failed", output, err)
	}
	appendWorkflowStep(ctx, storeClient, loopID, correlationID, "auth", "codex login prepared", nil)
	return nil
}

func shouldPrimeCodexLogin(cfg loopExecutionConfig) bool {
	provider := strings.ToLower(strings.TrimSpace(cfg.ProviderID))
	if provider == "codex" {
		return true
	}
	command := strings.ToLower(strings.TrimSpace(cfg.CodexCommand))
	return strings.HasPrefix(command, "codex ") || command == "codex"
}

type prdProgress struct {
	Total      int
	Open       int
	InProgress int
	Done       int
}

func runTechPlanning(
	ctx context.Context,
	storeClient store.StateStore,
	loopID, correlationID string,
	cfg loopExecutionConfig,
	anomaly model.Anomaly,
	workspace string,
	runner execRunner,
	prdPath string,
) (string, string, error) {
	techSpecPath := filepath.Join(workspace, ".agents", "tasks", "tech-spec.md")
	implementationPlanPath := filepath.Join(workspace, ".agents", "tasks", "implementation-plan.md")

	if err := os.MkdirAll(filepath.Dir(techSpecPath), 0o755); err != nil {
		return "", "", err
	}

	// 1. Generate Tech Spec
	techSpecPrompt := buildTechSpecPrompt(anomaly, prdPath, techSpecPath)
	if err := runCodexStep(ctx, runner, workspace, cfg.CodexCommand, "tech-spec", techSpecPrompt, loopID, correlationID, storeClient); err != nil {
		return "", "", err
	}
	if err := syncDocumentWithEtcd(ctx, storeClient, "tech-spec-"+loopID, techSpecPath, loopID, correlationID); err != nil {
		log.Printf("failed to sync tech spec with etcd: %v", err)
	}

	// 2. Generate Implementation Plan
	planPrompt := buildImplementationPlanPrompt(anomaly, prdPath, techSpecPath, implementationPlanPath)
	if err := runCodexStep(ctx, runner, workspace, cfg.CodexCommand, "implementation-plan", planPrompt, loopID, correlationID, storeClient); err != nil {
		return "", "", err
	}
	if err := syncDocumentWithEtcd(ctx, storeClient, "implementation-plan-"+loopID, implementationPlanPath, loopID, correlationID); err != nil {
		log.Printf("failed to sync implementation plan with etcd: %v", err)
	}

	return techSpecPath, implementationPlanPath, nil
}

func buildTechSpecPrompt(anomaly model.Anomaly, prdPath, techSpecPath string) string {
	return strings.Join([]string{
		"You are an autonomous coding agent.",
		"Use the PRD at: " + prdPath,
		"Generate a technical specification for implementing the requirements.",
		"Include:",
		"- Architecture overview",
		"- Detailed component design",
		"- API definitions (if applicable)",
		"- Data model changes",
		"- Security considerations",
		"Save the tech spec to: " + techSpecPath,
		"Do NOT implement anything.",
		"User request: " + anomaly.Title,
	}, "\n")
}

func buildImplementationPlanPrompt(anomaly model.Anomaly, prdPath, techSpecPath, planPath string) string {
	return strings.Join([]string{
		"You are an autonomous coding agent.",
		"Use the PRD at: " + prdPath,
		"Use the technical specification at: " + techSpecPath,
		"Generate a hierarchical implementation plan (Phases -> Tasks -> Sub-tasks).",
		"Each task should be granular and actionable.",
		"Save the implementation plan to: " + planPath,
		"Do NOT implement anything.",
		"User request: " + anomaly.Title,
	}, "\n")
}

func runIssueBuildIterations(
	ctx context.Context,
	storeClient store.StateStore,
	loopID, correlationID string,
	cfg loopExecutionConfig,
	anomaly model.Anomaly,
	workspace string,
	runner execRunner,
	prdPath string,
	implementationPlanPath string,
	validationCommands []string,
) (model.LoopState, string, error) {

	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 1
	}
	for iteration := 1; iteration <= maxIterations; iteration++ {
		if state, reason, done, err := loopTerminalDecision(ctx, storeClient, loopID); err != nil {
			return model.LoopStateFlatline, "state-read-failed", err
		} else if done {
			return state, reason, nil
		}

		before, err := readPRDProgress(prdPath)
		if err != nil {
			return model.LoopStateFlatline, "issue-prd-progress-read-failed", err
		}
		if before.Total > 0 && before.Done >= before.Total {
			return model.LoopStateSynced, "issue-prd-build-complete", nil
		}

		buildPrompt := buildIssueBuildPrompt(anomaly, prdPath, implementationPlanPath, validationCommands, iteration, maxIterations, before)

		stepName := fmt.Sprintf("build-%02d", iteration)
		if err := runCodexStep(ctx, runner, workspace, cfg.CodexCommand, stepName, buildPrompt, loopID, correlationID, storeClient); err != nil {
			return model.LoopStateFlatline, "issue-build-step-failed", err
		}

		after, err := readPRDProgress(prdPath)
		if err != nil {
			return model.LoopStateFlatline, "issue-prd-progress-read-failed", err
		}
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "info",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "issue build iteration completed",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"iteration":             strconv.Itoa(iteration),
				"max_iterations":        strconv.Itoa(maxIterations),
				"prd_path":              prdPath,
				"stories_total":         strconv.Itoa(after.Total),
				"stories_open_before":   strconv.Itoa(before.Open),
				"stories_open_after":    strconv.Itoa(after.Open),
				"stories_done_before":   strconv.Itoa(before.Done),
				"stories_done_after":    strconv.Itoa(after.Done),
				"stories_inprog_before": strconv.Itoa(before.InProgress),
				"stories_inprog_after":  strconv.Itoa(after.InProgress),
			},
		})
		if after.Total > 0 && after.Done >= after.Total {
			return model.LoopStateSynced, "issue-prd-build-complete", nil
		}
		if iteration < maxIterations {
			select {
			case <-ctx.Done():
				return model.LoopStateFlatline, "replica-context-cancelled", ctx.Err()
			case <-time.After(cfg.IterationWait):
			}
		}
	}

	return model.LoopStateFlatline, "issue-build-max-iterations-reached", nil
}

func appendWorkflowStep(
	ctx context.Context,
	storeClient store.StateStore,
	loopID, correlationID, step, message string,
	metadata map[string]string,
) {
	meta := map[string]string{
		"workflow_step": strings.TrimSpace(step),
	}
	for key, value := range metadata {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		meta[key] = trimmed
	}
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       message,
		CorrelationID: correlationID,
		Metadata:      meta,
	})
}

func resolveTaskContractAndValidationCommands(ctx context.Context, storeClient store.StateStore, anomaly model.Anomaly) (*model.TaskContract, []string, error) {
	commands, err := validationCommandsFromMetadata(anomaly.Metadata)
	if err != nil {
		return nil, nil, err
	}

	taskID := ""
	if anomaly.Metadata != nil {
		taskID = strings.TrimSpace(anomaly.Metadata["task_contract_id"])
	}
	if taskID == "" {
		return nil, normalizeValidationCommands(commands), nil
	}

	task, found, err := storeClient.GetTaskContract(ctx, taskID)
	if err != nil {
		return nil, nil, fmt.Errorf("read task contract %q: %w", taskID, err)
	}
	if !found {
		return nil, nil, fmt.Errorf("task contract %q not found", taskID)
	}

	resolved := normalizeValidationCommands(task.Validation)
	if len(resolved) == 0 {
		resolved = normalizeValidationCommands(commands)
	}
	return &task, resolved, nil
}

func validationCommandsFromMetadata(metadata map[string]string) ([]string, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	rawJSON := strings.TrimSpace(metadata["task_validation_commands_json"])
	if rawJSON != "" {
		var parsed []string
		if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
			return nil, fmt.Errorf("decode task_validation_commands_json: %w", err)
		}
		return normalizeValidationCommands(parsed), nil
	}
	raw := strings.TrimSpace(metadata["task_validation_commands"])
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, "\n")
	return normalizeValidationCommands(parts), nil
}

func normalizeValidationCommands(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func runTaskValidationCommands(
	ctx context.Context,
	runner execRunner,
	workspace, loopID, correlationID string,
	storeClient store.StateStore,
	commands []string,
) error {
	commands = normalizeValidationCommands(commands)
	if len(commands) == 0 {
		appendWorkflowStep(ctx, storeClient, loopID, correlationID, "validate", "no task validation commands configured; skipping", nil)
		return nil
	}

	appendWorkflowStep(ctx, storeClient, loopID, correlationID, "validate", "starting task validation commands", map[string]string{
		"validation_count": strconv.Itoa(len(commands)),
	})

	for idx, command := range commands {
		started := time.Now().UTC()
		output, err := runner.Run(ctx, workspace, "sh", "-lc", command)
		duration := time.Since(started)
		exitCode := commandExitCode(err)
		level := "info"
		status := "passed"
		message := "validation command passed"
		if err != nil {
			level = "warn"
			status = "failed"
			message = "validation command failed"
		}

		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         level,
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       message,
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"workflow_step":     "validate",
				"command":           command,
				"command_index":     strconv.Itoa(idx + 1),
				"command_total":     strconv.Itoa(len(commands)),
				"duration_ms":       strconv.FormatInt(duration.Milliseconds(), 10),
				"exit_code":         strconv.Itoa(exitCode),
				"validation_status": status,
			},
		})

		for _, line := range commandOutputLines(string(output), 40) {
			_ = storeClient.AppendJournal(ctx, model.JournalEntry{
				LoopID:        loopID,
				Phase:         "replica",
				Level:         level,
				ActorType:     "replica",
				ActorID:       hostnameOr("smith-replica"),
				Message:       fmt.Sprintf("[validate %d] %s", idx+1, line),
				CorrelationID: correlationID,
				Metadata: map[string]string{
					"workflow_step": "validate",
					"command_index": strconv.Itoa(idx + 1),
				},
			})
		}

		if err != nil {
			return fmt.Errorf("validation command %d failed (exit=%d): %w", idx+1, exitCode, err)
		}
	}

	appendWorkflowStep(ctx, storeClient, loopID, correlationID, "validate", "task validation stage complete", map[string]string{
		"validation_count": strconv.Itoa(len(commands)),
	})
	return nil
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func waitForInteractivePRD(
	ctx context.Context,
	storeClient store.StateStore,
	loopID, correlationID string,
	cfg loopExecutionConfig,
	prdPath string,
	interactivePromptPath string,
	interactiveCommand string,
) (string, string, error) {
	metadata := map[string]string{
		"prd_path":        prdPath,
		"interactive_for": interactiveWaitLabel(cfg.InteractivePRDWait),
		"continue_hint":   "create PRD file to continue immediately",
	}
	if strings.TrimSpace(interactivePromptPath) != "" {
		metadata["interactive_prompt_path"] = interactivePromptPath
	}
	if strings.TrimSpace(interactiveCommand) != "" {
		metadata["interactive_command"] = interactiveCommand
	}
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "interactive PRD gate open; attach terminal for clarifications",
		CorrelationID: correlationID,
		Metadata:      metadata,
	})

	timeoutEnabled := cfg.InteractivePRDWait > 0
	deadline := time.Time{}
	if timeoutEnabled {
		deadline = time.Now().UTC().Add(cfg.InteractivePRDWait)
	}
	for {
		resolvedPRDPath, resolveSource, resolveErr := resolveIssuePRDPath(prdPath)
		if resolveErr == nil {
			meta := map[string]string{
				"prd_path": prdPath,
			}
			if resolvedPRDPath != prdPath {
				meta["resolved_prd_path"] = resolvedPRDPath
				meta["resolution_source"] = resolveSource
			}
			_ = storeClient.AppendJournal(ctx, model.JournalEntry{
				LoopID:        loopID,
				Phase:         "replica",
				Level:         "info",
				ActorType:     "replica",
				ActorID:       hostnameOr("smith-replica"),
				Message:       "interactive PRD gate satisfied by existing PRD file",
				CorrelationID: correlationID,
				Metadata:      meta,
			})
			return resolvedPRDPath, "", nil
		}
		if resolveErr != nil && !errors.Is(resolveErr, os.ErrNotExist) {
			return "", "issue-prd-check-failed", resolveErr
		}
		_, reason, done, err := loopTerminalDecision(ctx, storeClient, loopID)
		if err != nil {
			return "", "state-read-failed", err
		}
		if done {
			return "", reason, nil
		}
		if timeoutEnabled && time.Now().UTC().After(deadline) {
			_ = storeClient.AppendJournal(ctx, model.JournalEntry{
				LoopID:        loopID,
				Phase:         "replica",
				Level:         "warn",
				ActorType:     "replica",
				ActorID:       hostnameOr("smith-replica"),
				Message:       "interactive PRD gate timed out; continuing with automated PRD generation",
				CorrelationID: correlationID,
				Metadata: map[string]string{
					"prd_path": prdPath,
				},
			})
			return "", "", nil
		}
		select {
		case <-ctx.Done():
			return "", "replica-context-cancelled", ctx.Err()
		case <-time.After(cfg.InteractivePRDPoll):
		}
	}
}

func interactiveWaitLabel(wait time.Duration) string {
	if wait <= 0 {
		return "none"
	}
	return wait.String()
}

func interactivePRDCommandHint(promptPath, prdPath string, storyCount int, codexCommand string) string {
	if strings.TrimSpace(promptPath) == "" {
		return ""
	}
	parts := []string{
		"smith --prompt " + shellQuote(promptPath),
		"--out " + shellQuote(prdPath),
	}
	if storyCount > 0 {
		parts = append(parts, "--stories "+strconv.Itoa(storyCount))
	}
	if cmd := strings.TrimSpace(codexCommand); cmd != "" {
		parts = append(parts, "--agent-cmd "+shellQuote(cmd))
	}
	return strings.Join(parts, " ")
}

func writePromptFile(workspace, loopID, step, prompt string) (string, error) {
	promptsDir := filepath.Join(workspace, ".smith", "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		return "", fmt.Errorf("create prompt dir: %w", err)
	}
	promptPath := filepath.Join(promptsDir, fmt.Sprintf("%s-%d-%s.md", sanitizePromptID(loopID), time.Now().UTC().UnixNano(), step))
	if err := writePromptFileAtPath(promptPath, prompt); err != nil {
		return "", err
	}
	return promptPath, nil
}

func writePromptFileAtPath(promptPath, prompt string) error {
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		return fmt.Errorf("create prompt dir: %w", err)
	}
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("write prompt file: %w", err)
	}
	return nil
}

func resolveIssuePRDPath(expectedPath string) (string, string, error) {
	if expectedPath == "" {
		return "", "", errors.New("expected prd path is empty")
	}
	if stat, err := os.Stat(expectedPath); err == nil && !stat.IsDir() {
		return expectedPath, "expected_path", nil
	}
	tasksDir := filepath.Dir(expectedPath)
	pattern := filepath.Join(tasksDir, "*.json")
	candidates, err := filepath.Glob(pattern)
	if err != nil {
		return "", "", fmt.Errorf("glob prd files: %w", err)
	}
	files := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		stat, statErr := os.Stat(candidate)
		if statErr != nil || stat.IsDir() {
			continue
		}
		files = append(files, candidate)
	}
	switch len(files) {
	case 0:
		return "", "", os.ErrNotExist
	case 1:
		return files[0], "single_json_in_tasks", nil
	default:
		return "", "", os.ErrNotExist
	}
}

func expectedPRDStoryCount(defaultCount int, anomaly model.Anomaly) int {
	if anomaly.Metadata != nil {
		if parsed, ok := parsePositiveInt(anomaly.Metadata["prd_story_count"]); ok {
			return parsed
		}
	}
	if defaultCount > 0 {
		return defaultCount
	}
	return defaultPRDStoryCount
}

func syncDocumentWithEtcd(ctx context.Context, storeClient store.StateStore, docID, path, loopID, correlationID string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("document path not found for sync: %s", path)
			return nil
		}
		return err
	}
	return storeClient.PutDocument(ctx, model.Document{

		ID:            docID,
		Content:       string(payload),
		UpdatedAt:     time.Now().UTC(),
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"loop_id": loopID,
			"path":    path,
		},
	})
}

func materializePRDFromMetadata(prdPath string, metadata map[string]string) (bool, error) {

	if metadata == nil {
		return false, nil
	}
	raw := strings.TrimSpace(metadata["workspace_prd_json"])
	if raw == "" {
		return false, nil
	}
	if !json.Valid([]byte(raw)) {
		return false, errors.New("workspace_prd_json is not valid json")
	}
	if err := os.MkdirAll(filepath.Dir(prdPath), 0o755); err != nil {
		return false, fmt.Errorf("prepare prd dir: %w", err)
	}
	if err := os.WriteFile(prdPath, []byte(raw), 0o644); err != nil {
		return false, fmt.Errorf("write prd file: %w", err)
	}
	return true, nil
}

func ensurePRDStoryCount(
	ctx context.Context,
	storeClient store.StateStore,
	loopID, correlationID, prdPath string,
	expected int,
) error {
	actual, err := readPRDStoryCount(prdPath)
	if err != nil {
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "error",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "PRD validation failed",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"prd_path":             prdPath,
				"expected_story_count": strconv.Itoa(expected),
				"error":                err.Error(),
			},
		})
		return err
	}
	if actual != expected {
		err := fmt.Errorf("prd stories count mismatch: expected %d, found %d", expected, actual)
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "error",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "PRD validation failed",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"prd_path":             prdPath,
				"expected_story_count": strconv.Itoa(expected),
				"actual_story_count":   strconv.Itoa(actual),
			},
		})
		return err
	}
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "PRD validated",
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"prd_path":             prdPath,
			"expected_story_count": strconv.Itoa(expected),
			"actual_story_count":   strconv.Itoa(actual),
		},
	})
	return nil
}

func readPRDStoryCount(prdPath string) (int, error) {
	progress, err := readPRDProgress(prdPath)
	if err != nil {
		return 0, err
	}
	return progress.Total, nil
}

func readPRDProgress(prdPath string) (prdProgress, error) {
	payload, err := os.ReadFile(prdPath)
	if err != nil {
		return prdProgress{}, fmt.Errorf("read prd file: %w", err)
	}
	var doc struct {
		Stories []struct {
			Status string `json:"status"`
		} `json:"stories"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		return prdProgress{}, fmt.Errorf("parse prd json: %w", err)
	}
	if doc.Stories == nil {
		return prdProgress{}, errors.New("prd json must include a stories array")
	}
	progress := prdProgress{
		Total: len(doc.Stories),
	}
	for _, story := range doc.Stories {
		switch strings.ToLower(strings.TrimSpace(story.Status)) {
		case "done":
			progress.Done++
		case "in_progress":
			progress.InProgress++
		default:
			progress.Open++
		}
	}
	return progress, nil
}

func parsePositiveInt(raw string) (int, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func loopTerminalDecision(ctx context.Context, storeClient store.StateStore, loopID string) (model.LoopState, string, bool, error) {
	state, found, err := storeClient.GetState(ctx, loopID)
	if err != nil {
		return model.LoopStateFlatline, "state-read-failed", false, err
	}
	if !found {
		return model.LoopStateFlatline, "state-missing", false, fmt.Errorf("state not found for loop_id=%s", loopID)
	}
	switch state.Record.State {
	case model.LoopStateRunning:
		return model.LoopStateRunning, "", false, nil
	case model.LoopStateCancelled:
		return model.LoopStateCancelled, "operator-cancelled", true, nil
	case model.LoopStateFlatline:
		return model.LoopStateFlatline, "operator-terminated", true, nil
	case model.LoopStateSynced:
		return model.LoopStateSynced, "already-synced", true, nil
	default:
		return state.Record.State, "loop-not-active", true, nil
	}
}

func runCodexStep(
	ctx context.Context,
	runner execRunner,
	workspace, codexCommand, step, prompt, loopID, correlationID string,
	storeClient store.StateStore,
) error {
	codexCommand = strings.TrimSpace(codexCommand)
	if codexCommand == "" {
		return errors.New("agent command is empty")
	}
	fields := strings.Fields(codexCommand)
	if len(fields) == 0 {
		return errors.New("agent command is invalid")
	}
	if _, err := lookPath(fields[0]); err != nil {
		return fmt.Errorf("agent command %q not found in PATH", fields[0])
	}

	promptPath, err := writePromptFile(workspace, loopID, step, prompt)
	if err != nil {
		return err
	}

	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "agent step started: " + step,
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"step":        step,
			"prompt_path": promptPath,
			"command":     codexCommand,
		},
	})

	shellCmd := fmt.Sprintf("cat %s | %s", shellQuote(promptPath), codexCommand)
	output, err := runner.Run(ctx, workspace, "sh", "-lc", shellCmd)
	lines := commandOutputLines(string(output), 80)
	for _, line := range lines {
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "replica",
			Level:         "info",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "[" + step + "] " + line,
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"step": step,
			},
		})
	}
	if err != nil {
		return fmt.Errorf("agent step %q failed: %w", step, err)
	}
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "agent step completed: " + step,
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"step": step,
		},
	})
	return nil
}

func buildIssuePRDPrompt(anomaly model.Anomaly, prdPath string, storyCount int) string {
	if storyCount <= 0 {
		storyCount = defaultPRDStoryCount
	}
	lines := []string{
		"You are an autonomous coding agent.",
		"Use the $prd skill to create a Product Requirements Document in JSON.",
		"Save the PRD to: " + prdPath,
		"Do NOT implement anything.",
		"Requirements:",
		fmt.Sprintf("- Include exactly %d user stories in the stories array.", storyCount),
		"- Each story should be actionable, independently testable, and have status set to \"open\".",
		"After creating the PRD, end with:",
		"PRD JSON saved to " + prdPath + ". Close this chat and run `smith build`.",
		"",
		"User request:",
		"- title: " + strings.TrimSpace(anomaly.Title),
		"- description:",
		strings.TrimSpace(anomaly.Description),
	}

	interactivePrompt := ""
	if anomaly.Metadata != nil {
		interactivePrompt = strings.TrimSpace(anomaly.Metadata["workspace_prompt"])
	}
	if interactivePrompt != "" {
		lines = append(lines, "", "Operator Prompt:", interactivePrompt)
	}

	if fullContext := fullIssueContextForPrompt(anomaly.Metadata); fullContext != "" {
		lines = append(lines, "", "GitHub Issue Full Context (JSON):", fullContext)
	}
	return strings.Join(lines, "\n")
}

func buildIssueBuildPrompt(anomaly model.Anomaly, prdPath, planPath string, validationCommands []string, iteration, maxIterations int, progress prdProgress) string {
	lines := []string{
		"You are an autonomous coding agent.",
		"Use the PRD JSON at: " + prdPath,
		"Use the implementation plan at: " + planPath,
		fmt.Sprintf("Current build iteration: %d of %d", iteration, maxIterations),

		"",
		"Instructions:",
		"1. Read the PRD to identify the next 'open' user story.",
		"2. Implement the changes required for that story.",
		"3. Run verification (tests, linting) to ensure correctness.",
		"4. Update the story status to 'done' in the PRD JSON.",
		"5. Repeat for the next story if time permits.",
		"",
		"Do NOT implement anything not defined in the PRD.",
		"If you encounter a blocker, record it in the PRD and stop.",
		"",
		"Issue Context:",
		"- title: " + strings.TrimSpace(anomaly.Title),
	}
	if len(validationCommands) > 0 {
		lines = append(lines, "", "Task Validation Commands (must pass before completion):")
		for _, command := range normalizeValidationCommands(validationCommands) {
			lines = append(lines, "- "+command)
		}
	}
	if fullContext := fullIssueContextForPrompt(anomaly.Metadata); fullContext != "" {
		lines = append(lines, "", "GitHub Issue Full Context (JSON):", fullContext)
	}
	return strings.Join(lines, "\n")
}
func fullIssueContextForPrompt(metadata map[string]string) string {
	if metadata == nil {
		return ""
	}
	raw := strings.TrimSpace(metadata["github_issue_context_json"])
	if raw == "" {
		return ""
	}
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		if len(raw) > 20000 {
			return raw[:20000] + "\n...[truncated]"
		}
		return raw
	}
	formatted, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		if len(raw) > 20000 {
			return raw[:20000] + "\n...[truncated]"
		}
		return raw
	}
	if len(formatted) > 20000 {
		return string(formatted[:20000]) + "\n...[truncated]"
	}
	return string(formatted)
}

func parseBoolEnv(raw string, fallback bool) bool {
	text := strings.TrimSpace(strings.ToLower(raw))
	if text == "" {
		return fallback
	}
	switch text {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func commandOutputLines(raw string, maxLines int) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line == "" {
			continue
		}
		if len(line) > 400 {
			line = line[:400]
		}
		out = append(out, line)
		if maxLines > 0 && len(out) >= maxLines {
			break
		}
	}
	return out
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func sanitizePromptID(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.NewReplacer("/", "-", "\\", "-", " ", "-", "_", "-").Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "loop"
	}
	if len(value) > 48 {
		return value[:48]
	}
	return value
}

func normalizeInvocationMethod(raw string) string {
	method := strings.ToLower(strings.TrimSpace(raw))
	if method == "" {
		return "unknown"
	}
	return method
}

var agentCommandMap = map[string]string{
	"codex":            "codex exec --yolo --skip-git-repo-check -",
	"claude":           "claude -p --dangerously-skip-permissions",
	"gemini":           "gemini -p --yolo",
	"upstream-tooling": "upstream-tooling build --yolo -",
	"openai":           "openai-agent exec -",
}

func resolveAgentCommand(providerID string) string {
	// 1. Global override via environment variable
	if command := strings.TrimSpace(os.Getenv("SMITH_AGENT_CLI_CMD")); command != "" {
		return command
	}

	providerID = strings.ToLower(strings.TrimSpace(providerID))
	if providerID == "" {
		providerID = "codex" // default fallback
	}

	// 2. Provider-specific override via environment variable (e.g. SMITH_AGENT_CLI_CMD_RALPH)
	if providerEnv := providerCommandEnvVar(providerID); providerEnv != "" {
		if command := strings.TrimSpace(os.Getenv(providerEnv)); command != "" {
			return command
		}
	}

	// 3. Resolve from internal agent map
	if cmd, ok := agentCommandMap[providerID]; ok {
		return cmd
	}

	// 4. Final fallback to Codex for backward compatibility
	return agentCommandMap["codex"]
}

func providerCommandEnvVar(providerID string) string {
	raw := strings.ToUpper(strings.TrimSpace(providerID))
	if raw == "" {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("SMITH_AGENT_CLI_CMD_")
	lastUnderscore := false
	for _, r := range raw {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if lastUnderscore {
			continue
		}
		builder.WriteByte('_')
		lastUnderscore = true
	}
	return strings.Trim(builder.String(), "_")
}
func defaultLoopProfileForMethod(method string) (int, time.Duration) {
	switch normalizeInvocationMethod(method) {
	case "manual", "console", "interactive":
		return 120, 2 * time.Second
	case "github_issue", "issue", "issue_based":
		return 90, 2 * time.Second
	case "prd", "prd_task", "markdown_prd":
		return 20, 1 * time.Second
	default:
		return defaultLoopMaxIterations, defaultLoopIterationWait
	}
}

func finalizeLoopState(ctx context.Context, storeClient store.StateStore, loopID string, desired model.LoopState, reason string) (model.LoopState, error) {
	updated, err := storeClient.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		if isTerminalState(current.State) {
			current.LockHolder = ""
			if strings.TrimSpace(current.Reason) == "" && strings.TrimSpace(reason) != "" {
				current.Reason = reason
			}
			return current, nil
		}

		if current.State != model.LoopStateRunning {
			return current, nil
		}

		switch desired {
		case model.LoopStateSynced, model.LoopStateCancelled, model.LoopStateFlatline:
			current.State = desired
		default:
			current.State = model.LoopStateFlatline
		}
		current.Reason = reason
		current.LockHolder = ""
		return current, nil
	})
	if err != nil {
		return model.LoopStateFlatline, err
	}
	return updated.Record.State, nil
}

func isTerminalState(state model.LoopState) bool {
	switch state {
	case model.LoopStateSynced, model.LoopStateFlatline, model.LoopStateCancelled:
		return true
	default:
		return false
	}
}

func mergeMetadata(left, right map[string]string) map[string]string {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range left {
		out[key] = value
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"http://127.0.0.1:2379"}
	}
	return out
}

func hostnameOr(fallback string) string {
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		return fallback
	}
	return h
}

func runtimeMetadataFromEnv() map[string]string {
	out := map[string]string{}
	ref := strings.TrimSpace(os.Getenv("SMITH_EXECUTION_IMAGE_REF"))
	if ref != "" {
		out["execution_image_ref"] = ref
	}
	source := strings.TrimSpace(os.Getenv("SMITH_EXECUTION_IMAGE_SOURCE"))
	if source != "" {
		out["execution_image_source"] = source
	}
	digest := strings.TrimSpace(os.Getenv("SMITH_EXECUTION_IMAGE_DIGEST"))
	if digest != "" {
		out["execution_image_digest"] = digest
	}
	pullPolicy := strings.TrimSpace(os.Getenv("SMITH_EXECUTION_IMAGE_PULL_POLICY"))
	if pullPolicy != "" {
		out["execution_image_pull_policy"] = pullPolicy
	}
	invocationMethod := strings.TrimSpace(os.Getenv("SMITH_LOOP_INVOCATION_METHOD"))
	if invocationMethod != "" {
		out["loop_invocation_method"] = invocationMethod
	}
	provider := strings.TrimSpace(os.Getenv("SMITH_LOOP_PROVIDER"))
	if provider != "" {
		out["loop_provider"] = provider
	}
	modelID := strings.TrimSpace(os.Getenv("SMITH_LOOP_MODEL"))
	if modelID != "" {
		out["loop_model"] = modelID
	}
	sourceType := strings.TrimSpace(os.Getenv("SMITH_LOOP_SOURCE_TYPE"))
	if sourceType != "" {
		out["loop_source_type"] = sourceType
	}
	sourceRef := strings.TrimSpace(os.Getenv("SMITH_LOOP_SOURCE_REF"))
	if sourceRef != "" {
		out["loop_source_ref"] = sourceRef
	}
	journalRetentionMode := strings.TrimSpace(os.Getenv("SMITH_JOURNAL_RETENTION_MODE"))
	if journalRetentionMode != "" {
		out["journal_retention_mode"] = journalRetentionMode
	}
	journalRetentionTTL := strings.TrimSpace(os.Getenv("SMITH_JOURNAL_RETENTION_TTL"))
	if journalRetentionTTL != "" {
		out["journal_retention_ttl"] = journalRetentionTTL
	}
	journalArchiveMode := strings.TrimSpace(os.Getenv("SMITH_JOURNAL_ARCHIVE_MODE"))
	if journalArchiveMode != "" {
		out["journal_archive_mode"] = journalArchiveMode
	}
	journalArchiveBucket := strings.TrimSpace(os.Getenv("SMITH_JOURNAL_ARCHIVE_BUCKET"))
	if journalArchiveBucket != "" {
		out["journal_archive_bucket"] = journalArchiveBucket
	}
	skillCount := strings.TrimSpace(os.Getenv("SMITH_SKILL_MOUNT_COUNT"))
	if skillCount != "" {
		out["skill_mount_count"] = skillCount
	}
	skillMounts := strings.TrimSpace(os.Getenv("SMITH_SKILL_MOUNTS"))
	if skillMounts != "" {
		out["skill_mounts"] = skillMounts
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func readHandoffFile(path string) (*handoffFile, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var parsed handoffFile
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func loadStartupContext(ctx context.Context, storeClient store.StateStore, loopID string) (startupContext, error) {
	anomaly, found, err := storeClient.GetAnomaly(ctx, loopID)
	if err != nil {
		return startupContext{}, err
	}
	if !found {
		return startupContext{}, fmt.Errorf("anomaly not found for loop_id=%s", loopID)
	}
	handoff, foundHandoff, err := storeClient.GetLatestHandoff(ctx, loopID)
	if err != nil {
		return startupContext{}, err
	}
	startup := startupContext{Anomaly: anomaly}
	if foundHandoff {
		startup.PriorHandoff = &handoff
	}
	return startup, nil
}

func recordStartupFailure(ctx context.Context, storeClient store.StateStore, loopID, correlationID string, startupErr error) {
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "startup",
		Level:         "error",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "replica startup context load failed",
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"error": startupErr.Error(),
		},
	})
	if _, err := storeClient.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		if current.State == model.LoopStateSynced || current.State == model.LoopStateFlatline || current.State == model.LoopStateCancelled {
			return current, nil
		}
		current.State = model.LoopStateFlatline
		current.Reason = "startup-context-load-failed"
		return current, nil
	}); err == nil {
		return
	}

	_, _ = storeClient.PutState(ctx, model.StateRecord{
		LoopID:        loopID,
		State:         model.LoopStateFlatline,
		Reason:        "startup-context-load-failed",
		CorrelationID: correlationID,
	}, 0)
}

func recordRuntimeFailure(ctx context.Context, storeClient store.StateStore, loopID, correlationID string, runtimeErr error) {
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "error",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "replica runtime loop failed",
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"error": runtimeErr.Error(),
		},
	})
	_, _ = storeClient.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		if isTerminalState(current.State) {
			return current, nil
		}
		current.State = model.LoopStateFlatline
		current.Reason = "replica-runtime-failed"
		return current, nil
	})
}

func syncTaskContractStatusFromAnomaly(ctx context.Context, storeClient store.StateStore, anomaly model.Anomaly, loopState model.LoopState, reason string) {
	taskID := strings.TrimSpace(anomaly.Metadata["task_contract_id"])
	if taskID == "" {
		return
	}
	task, found, err := storeClient.GetTaskContract(ctx, taskID)
	if err != nil || !found {
		return
	}
	target, ok := taskContractStatusForLoopState(loopState)
	if !ok || task.Status == target {
		return
	}
	before := task.Status
	task.Status = target
	if strings.TrimSpace(task.CorrelationID) == "" {
		task.CorrelationID = anomaly.CorrelationID
	}
	if err := storeClient.PutTaskContract(ctx, task); err != nil {
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        anomaly.ID,
			Phase:         "replica",
			Level:         "warn",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "failed to synchronize task contract status",
			CorrelationID: anomaly.CorrelationID,
			Metadata: map[string]string{
				"task_contract_id": taskID,
				"loop_state":       string(loopState),
				"target_status":    string(target),
				"error":            err.Error(),
			},
		})
		return
	}
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        anomaly.ID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "task contract status synchronized",
		CorrelationID: anomaly.CorrelationID,
		Metadata: map[string]string{
			"task_contract_id": taskID,
			"status_from":      string(before),
			"status_to":        string(target),
			"loop_state":       string(loopState),
			"reason":           reason,
		},
	})
}

func taskContractStatusForLoopState(loopState model.LoopState) (model.TaskContractStatus, bool) {
	switch loopState {
	case model.LoopStateRunning:
		return model.TaskContractStatusRunning, true
	case model.LoopStateSynced:
		return model.TaskContractStatusCompleted, true
	case model.LoopStateCancelled, model.LoopStateFlatline:
		return model.TaskContractStatusBlocked, true
	default:
		return "", false
	}
}
