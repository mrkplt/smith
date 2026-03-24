package completion

import (
	"strings"
	"testing"
)

func TestBuildCommitMessageUsesTraceabilitySummary(t *testing.T) {
	msg := buildCommitMessage("loop-123", "US-001: Feature-gated Kanban visibility\n\nTraceability:\n- PRD: doc:abc#US-001")
	if !strings.Contains(msg, "feat(loop): US-001: Feature-gated Kanban visibility") {
		t.Fatalf("expected descriptive commit subject, got %q", msg)
	}
	if !strings.Contains(msg, "Loop-ID: loop-123") {
		t.Fatalf("expected loop id trailer, got %q", msg)
	}
	if !strings.Contains(msg, "- PRD: doc:abc#US-001") {
		t.Fatalf("expected PRD traceability details, got %q", msg)
	}
}

func TestBuildCommitMessageFallsBackWithoutSummary(t *testing.T) {
	msg := buildCommitMessage("loop-123", "")
	if !strings.Contains(msg, "feat(loop): autonomous implementation update") {
		t.Fatalf("expected fallback subject, got %q", msg)
	}
}

func TestDefaultGitCommitIdentity(t *testing.T) {
	if defaultGitCommitUserName != "SMITH" {
		t.Fatalf("expected default git user name SMITH, got %q", defaultGitCommitUserName)
	}
	if defaultGitCommitUserEmail != "smith@cromleylabs.com" {
		t.Fatalf("expected default git user email smith@cromleylabs.com, got %q", defaultGitCommitUserEmail)
	}
}

func TestGitAddAllArgsExcludingSkillMounts(t *testing.T) {
	t.Setenv("SMITH_SKILL_MOUNT_PATHS", "")
	args := gitAddAllArgsExcludingSkillMounts()
	if len(args) < 4 {
		t.Fatalf("expected add args, got %#v", args)
	}
	if args[0] != "add" || args[1] != "-A" {
		t.Fatalf("expected git add -A prefix, got %#v", args[:2])
	}
	joined := strings.Join(args, " ")
	for _, expected := range []string{
		":(exclude).agents/skills",
		":(exclude).agents/skills/**",
		":(exclude).claude/skills",
		":(exclude).claude/skills/**",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %q in args, got %#v", expected, args)
		}
	}
}

func TestGitAddAllArgsExcludingSkillMountsFromEnv(t *testing.T) {
	t.Setenv("SMITH_SKILL_MOUNT_PATHS", "/workspace/.agents/skills/commit,/workspace/custom/skills/review,/opt/not-workspace")
	args := gitAddAllArgsExcludingSkillMounts()
	joined := strings.Join(args, " ")
	for _, expected := range []string{
		":(exclude).agents/skills/commit",
		":(exclude).agents/skills/commit/**",
		":(exclude)custom/skills/review",
		":(exclude)custom/skills/review/**",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %q in args, got %#v", expected, args)
		}
	}
	if strings.Contains(joined, "/opt/not-workspace") {
		t.Fatalf("did not expect non-workspace mount path in args: %#v", args)
	}
}

func TestSkillMountExcludePathspecs(t *testing.T) {
	got := skillMountExcludePathspecs("/workspace/.claude/skills/review,/workspace/.agents/skills/commit,/workspace/.agents/skills/commit")
	joined := strings.Join(got, " ")
	for _, expected := range []string{
		":(exclude).claude/skills/review",
		":(exclude).claude/skills/review/**",
		":(exclude).agents/skills/commit",
		":(exclude).agents/skills/commit/**",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %q in pathspecs, got %#v", expected, got)
		}
	}
}
