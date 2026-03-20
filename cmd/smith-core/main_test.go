package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"smith/internal/source/gitpolicy"
	"smith/internal/source/journalpolicy"
	"smith/internal/source/model"
	"smith/internal/source/replica"
	"smith/internal/source/store"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func BenchmarkEnsureSkillSourcesExist(b *testing.B) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "skill-test",
			Namespace: "default",
		},
	})
	orch := &orchestrator{
		kube: client,
		cfg: config{
			namespace: "default",
		},
	}

	// Create an array with many duplicate mounts to trigger the N+1 issue
	var mounts []replica.SkillMount
	for i := 0; i < 100; i++ {
		mounts = append(mounts, replica.SkillMount{
			Name:   "test",
			Source: "local://skills/test",
		})
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = orch.ensureSkillSourcesExist(ctx, mounts)
	}
}

func TestResolveExecutionImageSelectionDefaults(t *testing.T) {
	cfg := config{replicaImage: "ghcr.io/smith/replica:v1", replicaPullPolicy: "IfNotPresent"}
	got, err := resolveExecutionImageSelection(context.Background(), nil, cfg, "loop-1")
	if err != nil {
		t.Fatalf("resolveExecutionImageSelection error: %v", err)
	}

	if got.Ref != "ghcr.io/smith/replica:v1" {
		t.Fatalf("ref: got %q", got.Ref)
	}
	if got.PullPolicy != "IfNotPresent" {
		t.Fatalf("pull policy: got %q", got.PullPolicy)
	}
	if got.Source != "core_default" {
		t.Fatalf("source: got %q", got.Source)
	}
	if got.Digest != "" {
		t.Fatalf("digest: got %q", got.Digest)
	}
}

func TestResolveExecutionImageSelectionFromLoopEnvironment(t *testing.T) {
	cfg := config{replicaImage: "ghcr.io/smith/replica:v1", replicaPullPolicy: "IfNotPresent"}
	anomaly := &model.Anomaly{
		Environment: model.LoopEnvironment{
			ContainerImage: &model.ContainerImageProfile{
				Ref:        "ghcr.io/custom/replica@sha256:deadbeef",
				PullPolicy: "Always",
			},
		},
	}
	got, err := resolveExecutionImageSelection(context.Background(), anomaly, cfg, "loop-1")
	if err != nil {
		t.Fatalf("resolveExecutionImageSelection error: %v", err)
	}

	if got.Ref != "ghcr.io/custom/replica@sha256:deadbeef" {
		t.Fatalf("ref: got %q", got.Ref)
	}
	if got.PullPolicy != "Always" {
		t.Fatalf("pull policy: got %q", got.PullPolicy)
	}
	if got.Source != "loop_environment_container_image" {
		t.Fatalf("source: got %q", got.Source)
	}
	if got.Digest != "sha256:deadbeef" {
		t.Fatalf("digest: got %q", got.Digest)
	}
}

func TestResolveExecutionImageSelectionOverrideUsesDefaultPullPolicyWhenUnset(t *testing.T) {
	cfg := config{replicaImage: "ghcr.io/smith/replica:v1", replicaPullPolicy: "IfNotPresent"}
	anomaly := &model.Anomaly{
		Environment: model.LoopEnvironment{
			ContainerImage: &model.ContainerImageProfile{Ref: "ghcr.io/custom/replica:v2"},
		},
	}
	got, err := resolveExecutionImageSelection(context.Background(), anomaly, cfg, "loop-1")
	if err != nil {
		t.Fatalf("resolveExecutionImageSelection error: %v", err)
	}
	if got.PullPolicy != "IfNotPresent" {
		t.Fatalf("pull policy: got %q", got.PullPolicy)
	}
}

func TestResolveExecutionImageSelectionDockerfileDisabled(t *testing.T) {
	cfg := config{
		replicaImage:      "ghcr.io/smith/replica:v1",
		replicaPullPolicy: "IfNotPresent",
		dockerfileBuild:   false,
	}
	anomaly := &model.Anomaly{
		Environment: model.LoopEnvironment{
			Dockerfile: &model.DockerfileProfile{
				ContextDir:     "workspace",
				DockerfilePath: "Dockerfile",
			},
		},
	}
	_, err := resolveExecutionImageSelection(context.Background(), anomaly, cfg, "loop-1")
	if err == nil {
		t.Fatal("expected dockerfile resolution error when build path disabled")
	}
}

func TestDockerfileBuildTagStable(t *testing.T) {
	profile := model.DockerfileProfile{
		ContextDir:     "workspace",
		DockerfilePath: "Dockerfile",
		BuildArgs: map[string]string{
			"GO_VERSION": "1.22",
			"TARGETOS":   "linux",
		},
	}
	a := dockerfileBuildTag("loop-abc", profile)
	b := dockerfileBuildTag("loop-abc", profile)
	if a != b {
		t.Fatalf("expected stable tag, got %q vs %q", a, b)
	}
}

func TestParseImageDigest(t *testing.T) {
	if got := parseImageDigest("ghcr.io/acme/replica:v1"); got != "" {
		t.Fatalf("expected empty digest, got %q", got)
	}
	if got := parseImageDigest("ghcr.io/acme/replica@sha256:abc123"); got != "sha256:abc123" {
		t.Fatalf("unexpected digest: %q", got)
	}
}

func TestGitContextForGitHubIssueFallback(t *testing.T) {
	anomaly := model.Anomaly{
		SourceType: "github_issue",
		SourceRef:  "acme/smith#42",
	}
	got := gitContextFor(anomaly)
	if got.Repository != "acme/smith" {
		t.Fatalf("repo: got %q", got.Repository)
	}
	if got.Branch != "main" {
		t.Fatalf("branch: got %q", got.Branch)
	}
	if got.CommitSHA != "unknown" {
		t.Fatalf("commit: got %q", got.CommitSHA)
	}
}

func TestHandoffConfigMapNameSanitizesAndBounds(t *testing.T) {
	got := handoffConfigMapName("LOOP/With Spaces.and_extra_chars___abcdefghijklmnopqrstuvwxyz")
	if got == "" {
		t.Fatal("expected non-empty configmap name")
	}
	if len(got) > 48 {
		t.Fatalf("expected bounded configmap name, got len=%d (%q)", len(got), got)
	}
	if got[:8] != "handoff-" {
		t.Fatalf("expected handoff prefix, got %q", got)
	}
}

func TestWorkspacePRDConfigMapNameSanitizesAndBounds(t *testing.T) {
	got := workspacePRDConfigMapName("LOOP/With Spaces.and_extra_chars___abcdefghijklmnopqrstuvwxyz")
	if got == "" {
		t.Fatal("expected non-empty configmap name")
	}
	if len(got) > 58 {
		t.Fatalf("expected bounded configmap name, got len=%d (%q)", len(got), got)
	}
	if !strings.HasPrefix(got, "workspace-prd-") {
		t.Fatalf("expected workspace-prd prefix, got %q", got)
	}
}

func TestHandoffConfigMapNameNoTrailingHyphenAfterTruncate(t *testing.T) {
	loopID := "smi-9668e-prd-smoke-autonomous-prd-json-me"
	got := handoffConfigMapName(loopID)
	if strings.HasSuffix(got, "-") {
		t.Fatalf("handoff configmap name must not end with hyphen: %q", got)
	}
}

func TestWorkspacePRDConfigMapNameNoTrailingHyphenAfterTruncate(t *testing.T) {
	loopID := "smi-9668e-prd-smoke-autonomous-prd-json-me"
	got := workspacePRDConfigMapName(loopID)
	if strings.HasSuffix(got, "-") {
		t.Fatalf("workspace PRD configmap name must not end with hyphen: %q", got)
	}
}

func TestWorkspacePRDPayload(t *testing.T) {
	payload, ok, err := workspacePRDPayload(map[string]string{
		"workspace_prd_json": `{"stories":[{"id":"US-001","status":"open"}],"meta":{"x":1}}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected payload to be present")
	}
	if !strings.Contains(payload, `"stories"`) {
		t.Fatalf("expected serialized payload with stories, got %s", payload)
	}
}

func TestWorkspacePRDPayloadRejectsInvalid(t *testing.T) {
	_, _, err := workspacePRDPayload(map[string]string{
		"workspace_prd_json": `{"stories":`,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid json")
	}
	_, _, err = workspacePRDPayload(map[string]string{
		"workspace_prd_json": `{"stories":[]}`,
	})
	if err == nil {
		t.Fatal("expected validation error for empty stories")
	}
}

func TestWorkspacePRDPathFor(t *testing.T) {
	if got := workspacePRDPathFor(nil); got != defaultWorkspacePRDPath {
		t.Fatalf("expected default prd path %q, got %q", defaultWorkspacePRDPath, got)
	}
	if got := workspacePRDPathFor(map[string]string{"workspace_prd_path": ".agents/tasks/prd-custom.json"}); got != ".agents/tasks/prd-custom.json" {
		t.Fatalf("unexpected prd path %q", got)
	}
	if got := workspacePRDPathFor(map[string]string{"workspace_prd_path": "../secrets/prd.json"}); got != defaultWorkspacePRDPath {
		t.Fatalf("expected unsafe path fallback, got %q", got)
	}
	if got := workspacePRDPathFor(map[string]string{"workspace_prd_path": "/tmp/prd.json"}); got != defaultWorkspacePRDPath {
		t.Fatalf("expected absolute path fallback, got %q", got)
	}
}

func TestResolveSkillMountsFromAnomaly(t *testing.T) {
	readOnly := true
	anomaly := &model.Anomaly{
		ID: "loop-1",
		Skills: []model.LoopSkillMount{
			{
				Name:      "commit",
				Source:    "local://skills/commit",
				MountPath: "/smith/skills/commit",
				ReadOnly:  &readOnly,
			},
		},
	}
	mounts, names, err := resolveSkillMounts(anomaly)
	if err != nil {
		t.Fatalf("resolveSkillMounts error: %v", err)
	}
	if len(mounts) != 1 || len(names) != 1 {
		t.Fatalf("expected one skill mount, got mounts=%d names=%d", len(mounts), len(names))
	}
	if mounts[0].Source != "local://skills/commit" || mounts[0].MountPath != "/smith/skills/commit" || !mounts[0].ReadOnly {
		t.Fatalf("unexpected mount %+v", mounts[0])
	}
}

func TestSkillSourceConfigMapName(t *testing.T) {
	if got := skillSourceConfigMapName("local://skills/commit"); got != "skill-commit" {
		t.Fatalf("unexpected configmap name %q", got)
	}
	if got := skillSourceConfigMapName("http://example.com/skill"); got != "" {
		t.Fatalf("expected unsupported source to return empty name, got %q", got)
	}
}

func TestLoopInvocationMethodFor(t *testing.T) {
	if got := loopInvocationMethodFor(model.Anomaly{
		SourceType: "manual",
		Metadata: map[string]string{
			"invocation_method": "console_issue",
			"ingress_mode":      "prd",
		},
	}); got != "console_issue" {
		t.Fatalf("expected invocation_method override, got %q", got)
	}

	if got := loopInvocationMethodFor(model.Anomaly{
		SourceType: "github_issue",
		Metadata: map[string]string{
			"ingress_mode": "prd",
		},
	}); got != "prd" {
		t.Fatalf("expected ingress_mode fallback, got %q", got)
	}

	if got := loopInvocationMethodFor(model.Anomaly{SourceType: "github_issue"}); got != "github_issue" {
		t.Fatalf("expected source type fallback, got %q", got)
	}

	if got := loopInvocationMethodFor(model.Anomaly{}); got != "unknown" {
		t.Fatalf("expected unknown fallback, got %q", got)
	}
}

func TestLoopProviderFor(t *testing.T) {
	if got := loopProviderFor(model.Anomaly{
		ProviderID: "codex",
		Metadata: map[string]string{
			"workspace_provider": "claude",
			"workspace_agent":    "droid",
		},
	}); got != "codex" {
		t.Fatalf("expected provider_id precedence, got %q", got)
	}

	if got := loopProviderFor(model.Anomaly{
		Metadata: map[string]string{
			"workspace_provider": "claude",
			"workspace_agent":    "droid",
		},
	}); got != "claude" {
		t.Fatalf("expected workspace_provider fallback, got %q", got)
	}

	if got := loopProviderFor(model.Anomaly{
		Metadata: map[string]string{
			"workspace_agent": "droid",
		},
	}); got != "droid" {
		t.Fatalf("expected workspace_agent fallback, got %q", got)
	}

	if got := loopProviderFor(model.Anomaly{}); got != model.DefaultProviderID {
		t.Fatalf("expected default provider %q, got %q", model.DefaultProviderID, got)
	}
}

func TestGitAuthFor(t *testing.T) {
	t.Run("returns PAT auth when configured", func(t *testing.T) {
		auth := gitAuthFor(config{gitPATSecretName: "smith-runtime", gitPATSecretKey: "git_pat"})
		if auth == nil {
			t.Fatal("expected git auth config")
		}
		if auth.Provider != replica.GitAuthProviderPAT {
			t.Fatalf("expected PAT provider, got %q", auth.Provider)
		}
		if auth.PATSecretName != "smith-runtime" || auth.PATSecretKey != "git_pat" {
			t.Fatalf("unexpected PAT secret config: %+v", auth)
		}
	})

	t.Run("returns nil when secret name missing", func(t *testing.T) {
		auth := gitAuthFor(config{gitPATSecretName: "", gitPATSecretKey: "git_pat"})
		if auth != nil {
			t.Fatalf("expected nil auth, got %+v", auth)
		}
	})

	t.Run("returns nil when secret key missing", func(t *testing.T) {
		auth := gitAuthFor(config{gitPATSecretName: "smith-runtime", gitPATSecretKey: ""})
		if auth != nil {
			t.Fatalf("expected nil auth, got %+v", auth)
		}
	})
}

func TestTaskStatusForLoopState(t *testing.T) {
	tests := []struct {
		state model.LoopState
		want  model.TaskContractStatus
		ok    bool
	}{
		{state: model.LoopStateRunning, want: model.TaskContractStatusRunning, ok: true},
		{state: model.LoopStateSynced, want: model.TaskContractStatusCompleted, ok: true},
		{state: model.LoopStateCancelled, want: model.TaskContractStatusBlocked, ok: true},
		{state: model.LoopStateFlatline, want: model.TaskContractStatusBlocked, ok: true},
		{state: model.LoopStateUnresolved, want: "", ok: false},
	}
	for _, tc := range tests {
		got, ok := taskStatusForLoopState(tc.state)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("state=%s expected (%s,%t), got (%s,%t)", tc.state, tc.want, tc.ok, got, ok)
		}
	}
}

func TestSyncTaskContractStatus(t *testing.T) {
	ms := store.NewMemStore()
	ctx := context.Background()
	if err := ms.PutTaskContract(ctx, model.TaskContract{
		Kind:              "smith.task",
		ID:                "task-core",
		ProjectID:         "smith",
		ProviderProfileID: "openai-work",
		Objective:         "Core sync",
		Validation:        []string{"go test ./..."},
		Status:            model.TaskContractStatusApproved,
		CorrelationID:     "task-corr-core",
	}); err != nil {
		t.Fatalf("put task: %v", err)
	}
	orch := &orchestrator{
		store: ms,
		cfg:   config{holderID: "smith-core"},
	}
	anomaly := model.Anomaly{
		ID:            "loop-core",
		CorrelationID: "corr-core",
		Metadata: map[string]string{
			"task_contract_id": "task-core",
		},
	}

	orch.syncTaskContractStatus(ctx, anomaly, model.LoopStateRunning, "scheduled-by-core")
	task, found, err := ms.GetTaskContract(ctx, "task-core")
	if err != nil || !found {
		t.Fatalf("get task after running sync: found=%t err=%v", found, err)
	}
	if task.Status != model.TaskContractStatusRunning {
		t.Fatalf("expected running status, got %s", task.Status)
	}

	orch.syncTaskContractStatus(ctx, anomaly, model.LoopStateFlatline, "replica-job-create-failed")
	task, found, err = ms.GetTaskContract(ctx, "task-core")
	if err != nil || !found {
		t.Fatalf("get task after blocked sync: found=%t err=%v", found, err)
	}
	if task.Status != model.TaskContractStatusBlocked {
		t.Fatalf("expected blocked status, got %s", task.Status)
	}
}

func TestLoadConfigGitPolicyDefaults(t *testing.T) {
	t.Setenv("SMITH_GIT_POLICY_CONFIG_ENABLED", "")
	t.Setenv("SMITH_GIT_PAT_SECRET_NAME", "")
	t.Setenv("SMITH_GIT_PAT_SECRET_KEY", "")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.gitPolicyConfig {
		t.Fatal("expected git policy config flag to default false")
	}
	def := gitpolicy.DefaultPolicy()
	if cfg.gitPolicy.BranchCleanup != def.BranchCleanup {
		t.Fatalf("expected default branch cleanup %q got %q", def.BranchCleanup, cfg.gitPolicy.BranchCleanup)
	}
	if cfg.gitPolicy.ConflictPolicy != def.ConflictPolicy {
		t.Fatalf("expected default conflict policy %q got %q", def.ConflictPolicy, cfg.gitPolicy.ConflictPolicy)
	}
	if cfg.gitPATSecretName != "" {
		t.Fatalf("expected empty git PAT secret name, got %q", cfg.gitPATSecretName)
	}
	if cfg.gitPATSecretKey != "git_pat" {
		t.Fatalf("expected default git PAT secret key git_pat, got %q", cfg.gitPATSecretKey)
	}
}

func TestLoadConfigGitPATSecretOverrides(t *testing.T) {
	t.Setenv("SMITH_GIT_PAT_SECRET_NAME", "smith-runtime")
	t.Setenv("SMITH_GIT_PAT_SECRET_KEY", "github_pat")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.gitPATSecretName != "smith-runtime" {
		t.Fatalf("expected git PAT secret name override, got %q", cfg.gitPATSecretName)
	}
	if cfg.gitPATSecretKey != "github_pat" {
		t.Fatalf("expected git PAT secret key override, got %q", cfg.gitPATSecretKey)
	}
}

func TestLoadConfigRuntimeCredentialDefaults(t *testing.T) {
	t.Setenv("SMITH_RUNTIME_SECRET_NAME", "")
	t.Setenv("SMITH_RUNTIME_CREDENTIALS_KEY", "")
	t.Setenv("SMITH_RUNTIME_CREDENTIALS", "")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.runtimeSecretName != "" {
		t.Fatalf("expected empty runtime secret name, got %q", cfg.runtimeSecretName)
	}
	if cfg.runtimeSecretKey != "runtime_credentials" {
		t.Fatalf("expected default runtime credentials key runtime_credentials, got %q", cfg.runtimeSecretKey)
	}
	if cfg.runtimeCredentials != "" {
		t.Fatalf("expected empty inline runtime credentials fallback, got %q", cfg.runtimeCredentials)
	}
}

func TestLoadConfigRuntimeCredentialOverrides(t *testing.T) {
	t.Setenv("SMITH_RUNTIME_SECRET_NAME", "smith-runtime")
	t.Setenv("SMITH_RUNTIME_CREDENTIALS_KEY", "runtime_credentials_v2")
	t.Setenv("SMITH_RUNTIME_CREDENTIALS", "sk-inline-fallback")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.runtimeSecretName != "smith-runtime" {
		t.Fatalf("expected runtime secret name override, got %q", cfg.runtimeSecretName)
	}
	if cfg.runtimeSecretKey != "runtime_credentials_v2" {
		t.Fatalf("expected runtime credentials key override, got %q", cfg.runtimeSecretKey)
	}
	if cfg.runtimeCredentials != "sk-inline-fallback" {
		t.Fatalf("expected inline runtime credentials fallback override, got %q", cfg.runtimeCredentials)
	}
}

func TestLoadConfigGitPolicyOverrides(t *testing.T) {
	t.Setenv("SMITH_GIT_POLICY_CONFIG_ENABLED", "true")
	t.Setenv("SMITH_GIT_POLICY_BRANCH_CLEANUP", "never")
	t.Setenv("SMITH_GIT_POLICY_CONFLICT_POLICY", "fail_fast")
	t.Setenv("SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE", "false")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if !cfg.gitPolicyConfig {
		t.Fatal("expected git policy config flag true")
	}
	if cfg.gitPolicy.BranchCleanup != gitpolicy.BranchCleanupNever {
		t.Fatalf("unexpected branch cleanup %q", cfg.gitPolicy.BranchCleanup)
	}
	if cfg.gitPolicy.ConflictPolicy != gitpolicy.ConflictPolicyFailFast {
		t.Fatalf("unexpected conflict policy %q", cfg.gitPolicy.ConflictPolicy)
	}
	if cfg.gitPolicy.DeleteBranchOnMerge {
		t.Fatal("expected delete branch on merge false")
	}
}

func TestLoadConfigRejectsInvalidGitPolicyOverrides(t *testing.T) {
	t.Setenv("SMITH_GIT_POLICY_CONFIG_ENABLED", "true")
	t.Setenv("SMITH_GIT_POLICY_BRANCH_CLEANUP", "on_merge")
	t.Setenv("SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE", "false")
	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected invalid git policy config error")
	}
}

func TestLoadConfigJournalPolicyDefaults(t *testing.T) {
	t.Setenv("SMITH_JOURNAL_POLICY_CONFIG_ENABLED", "")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.journalPolicyConfig {
		t.Fatal("expected journal policy config flag to default false")
	}
	def := journalpolicy.DefaultPolicy()
	if cfg.journalPolicy.RetentionMode != def.RetentionMode {
		t.Fatalf("expected default retention mode %q got %q", def.RetentionMode, cfg.journalPolicy.RetentionMode)
	}
	if cfg.journalPolicy.ArchiveMode != def.ArchiveMode {
		t.Fatalf("expected default archive mode %q got %q", def.ArchiveMode, cfg.journalPolicy.ArchiveMode)
	}
}

func TestLoadConfigJournalPolicyOverrides(t *testing.T) {
	t.Setenv("SMITH_JOURNAL_POLICY_CONFIG_ENABLED", "true")
	t.Setenv("SMITH_JOURNAL_RETENTION_MODE", "ttl")
	t.Setenv("SMITH_JOURNAL_RETENTION_TTL", "168h")
	t.Setenv("SMITH_JOURNAL_ARCHIVE_MODE", "s3")
	t.Setenv("SMITH_JOURNAL_ARCHIVE_BUCKET", "smith-journal-archive")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if !cfg.journalPolicyConfig {
		t.Fatal("expected journal policy config flag true")
	}
	if cfg.journalPolicy.RetentionMode != journalpolicy.RetentionTTL {
		t.Fatalf("unexpected retention mode %q", cfg.journalPolicy.RetentionMode)
	}
	if cfg.journalPolicy.RetentionTTL != 168*time.Hour {
		t.Fatalf("unexpected retention ttl %s", cfg.journalPolicy.RetentionTTL)
	}
	if cfg.journalPolicy.ArchiveMode != journalpolicy.ArchiveS3 {
		t.Fatalf("unexpected archive mode %q", cfg.journalPolicy.ArchiveMode)
	}
	if cfg.journalPolicy.ArchiveBucket != "smith-journal-archive" {
		t.Fatalf("unexpected archive bucket %q", cfg.journalPolicy.ArchiveBucket)
	}
}

func TestLoadConfigRejectsInvalidJournalPolicyOverrides(t *testing.T) {
	t.Setenv("SMITH_JOURNAL_POLICY_CONFIG_ENABLED", "true")
	t.Setenv("SMITH_JOURNAL_RETENTION_MODE", "ttl")
	t.Setenv("SMITH_JOURNAL_RETENTION_TTL", "")
	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected invalid journal policy config error")
	}
}
