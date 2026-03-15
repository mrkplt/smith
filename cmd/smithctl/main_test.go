package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"smith/internal/source/model"
)

func TestResolveConfigFromFileAndOverrides(t *testing.T) {
	t.Setenv("SMITH_API_URL", "")
	t.Setenv("SMITH_OPERATOR_TOKEN", "")
	t.Setenv("SMITH_CONTEXT", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"dev","contexts":{"dev":{"server":"http://dev.local:8080","token":"dev-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := resolveConfig(rootFlags{Config: cfgPath, Output: "json"})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if resolved.Server != "http://dev.local:8080" {
		t.Fatalf("unexpected server: %s", resolved.Server)
	}
	if resolved.Token != "dev-token" {
		t.Fatalf("unexpected token: %s", resolved.Token)
	}
}

func TestResolveConfigEnvAndFlagPrecedence(t *testing.T) {
	t.Setenv("SMITH_API_URL", "http://env.local:8080")
	t.Setenv("SMITH_OPERATOR_TOKEN", "env-token")

	resolved, err := resolveConfig(rootFlags{Server: "http://flag.local:8080", Token: "flag-token", Output: "text"})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if resolved.Server != "http://flag.local:8080" {
		t.Fatalf("unexpected server: %s", resolved.Server)
	}
	if resolved.Token != "flag-token" {
		t.Fatalf("unexpected token: %s", resolved.Token)
	}
}

func TestLoopListUsesSMITH_CONTEXTForNonConfigCommands(t *testing.T) {
	t.Setenv("SMITH_API_URL", "")
	t.Setenv("SMITH_OPERATOR_TOKEN", "")
	t.Setenv("SMITH_CONTEXT", "staging")

	stagingHits := 0
	staging := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stagingHits++
		if r.Method != http.MethodGet || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "loop-staging"}})
	}))
	defer staging.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"default","contexts":{"default":{"server":"http://127.0.0.1:1","token":"default-token"},"staging":{"server":"` + staging.URL + `","token":"staging-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "--output", "json", "loop", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stagingHits != 1 {
		t.Fatalf("expected one request to staging server, got %d", stagingHits)
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(got) != 1 || got[0]["id"] != "loop-staging" {
		t.Fatalf("unexpected output: %#v", got)
	}
}

func TestLoopListEnvOverridesFileBackedServerAndToken(t *testing.T) {
	t.Setenv("SMITH_CONTEXT", "")

	var authHeader string
	overrideHits := 0
	override := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		overrideHits++
		if r.Method != http.MethodGet || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		authHeader = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "loop-env"}})
	}))
	defer override.Close()

	t.Setenv("SMITH_API_URL", override.URL)
	t.Setenv("SMITH_OPERATOR_TOKEN", "env-token")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"default","contexts":{"default":{"server":"http://127.0.0.1:1","token":"file-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "--output", "json", "loop", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if overrideHits != 1 {
		t.Fatalf("expected one request to override server, got %d", overrideHits)
	}
	if authHeader != "Bearer env-token" {
		t.Fatalf("expected env token auth header, got %q", authHeader)
	}
}

func TestPRDSubmitRootFlagsStillOverrideEnvAndConfig(t *testing.T) {
	var authHeader string
	flagServerHits := 0
	flagServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flagServerHits++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/ingress/prd" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		authHeader = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer flagServer.Close()

	t.Setenv("SMITH_API_URL", "http://127.0.0.1:1")
	t.Setenv("SMITH_OPERATOR_TOKEN", "env-token")
	t.Setenv("SMITH_CONTEXT", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfgContent := `{"current_context":"default","contexts":{"default":{"server":"http://127.0.0.1:2","token":"file-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	prdPath := filepath.Join(dir, "prd.json")
	prdContent := `{"version":1,"project":"Validation","overview":"Compatibility","qualityGates":["go test ./..."],"stories":[]}`
	if err := os.WriteFile(prdPath, []byte(prdContent), 0o600); err != nil {
		t.Fatalf("write prd file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--config", cfgPath,
		"--server", flagServer.URL,
		"--token", "flag-token",
		"--output", "json",
		"prd", "submit", "--file", prdPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if flagServerHits != 1 {
		t.Fatalf("expected one request to flag server, got %d", flagServerHits)
	}
	if authHeader != "Bearer flag-token" {
		t.Fatalf("expected flag token auth header, got %q", authHeader)
	}
}

func TestHelpListsConfigResource(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "config  Manage smithctl configuration") {
		t.Fatalf("expected config resource in help, got %q", stdout.String())
	}
}

func TestConfigWithoutSubcommandPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"config"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Usage: smithctl config <command>",
		"view",
		"get-contexts",
		"use-context",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output %q", want, output)
		}
	}
}

func TestConfigUnknownCommandFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"config", "unknown"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	errOutput := stderr.String()
	if !strings.Contains(errOutput, `unknown config command "unknown"`) {
		t.Fatalf("expected unknown command error, got %q", errOutput)
	}
	if !strings.Contains(errOutput, "Usage: smithctl config <command>") {
		t.Fatalf("expected config help in stderr, got %q", errOutput)
	}
}

func TestConfigViewPrintsYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"default","contexts":{"default":{"server":"http://default.local:8080","token":"default-token"},"staging":{"server":"http://staging.local:8080","token":"staging-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "view"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}

	var got fileConfig
	if err := yaml.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal yaml: %v\noutput=%s", err, stdout.String())
	}
	if got.CurrentContext != "default" {
		t.Fatalf("unexpected current context: %q", got.CurrentContext)
	}
	if len(got.Contexts) != 2 {
		t.Fatalf("expected two contexts, got %#v", got.Contexts)
	}
	if got.Contexts["default"].Server != "http://default.local:8080" || got.Contexts["default"].Token != "default-token" {
		t.Fatalf("unexpected default context: %#v", got.Contexts["default"])
	}
	if got.Contexts["staging"].Server != "http://staging.local:8080" || got.Contexts["staging"].Token != "staging-token" {
		t.Fatalf("unexpected staging context: %#v", got.Contexts["staging"])
	}
}

func TestConfigViewSupportsNoCurrentContext(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"contexts":{"default":{"server":"http://default.local:8080","token":"default-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "view"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}

	var got fileConfig
	if err := yaml.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal yaml: %v\noutput=%s", err, stdout.String())
	}
	if got.CurrentContext != "" {
		t.Fatalf("expected empty current context, got %q", got.CurrentContext)
	}
	if len(got.Contexts) != 1 {
		t.Fatalf("expected one context, got %#v", got.Contexts)
	}
}

func TestConfigViewMissingFileReturnsEmptyYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "missing.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "view"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}

	var got fileConfig
	if err := yaml.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal yaml: %v\noutput=%s", err, stdout.String())
	}
	if got.CurrentContext != "" {
		t.Fatalf("expected empty current context, got %q", got.CurrentContext)
	}
	if len(got.Contexts) != 0 {
		t.Fatalf("expected no contexts, got %#v", got.Contexts)
	}
}

func TestConfigGetContextsMarksCurrentContext(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"staging","contexts":{"default":{"server":"http://default.local:8080","token":"default-token"},"staging":{"server":"http://staging.local:8080","token":"staging-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "get-contexts"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}

	var got struct {
		CurrentContext string                         `yaml:"current_context"`
		Contexts       map[string]listedContextConfig `yaml:"contexts"`
	}
	if err := yaml.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal yaml: %v\noutput=%s", err, stdout.String())
	}
	if got.CurrentContext != "staging" {
		t.Fatalf("unexpected current context: %q", got.CurrentContext)
	}
	if len(got.Contexts) != 2 {
		t.Fatalf("expected two contexts, got %#v", got.Contexts)
	}
	if got.Contexts["default"].Current {
		t.Fatalf("expected default to not be current: %#v", got.Contexts["default"])
	}
	if !got.Contexts["staging"].Current {
		t.Fatalf("expected staging to be current: %#v", got.Contexts["staging"])
	}
}

func TestConfigGetContextsMissingFileReturnsEmptyYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "missing.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "get-contexts"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}

	var got struct {
		CurrentContext string                         `yaml:"current_context"`
		Contexts       map[string]listedContextConfig `yaml:"contexts"`
	}
	if err := yaml.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal yaml: %v\noutput=%s", err, stdout.String())
	}
	if got.CurrentContext != "" {
		t.Fatalf("expected empty current context, got %q", got.CurrentContext)
	}
	if len(got.Contexts) != 0 {
		t.Fatalf("expected no contexts, got %#v", got.Contexts)
	}
}

func TestConfigCurrentContextPrintsActiveContext(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"staging","contexts":{"staging":{"server":"http://staging.local:8080","token":"staging-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "current-context"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
	if stdout.String() != "staging\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestConfigCurrentContextWithoutCurrentContextFails(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "missing.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "current-context"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no current context is set") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestConfigUseContextUpdatesCurrentContextOnly(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"default","contexts":{"default":{"server":"http://default.local:8080","token":"default-token"},"staging":{"server":"http://staging.local:8080","token":"staging-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "use-context", "staging"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
	if stdout.String() != "Switched to context \"staging\"\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	cfg, err := readFileConfig(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.CurrentContext != "staging" {
		t.Fatalf("expected current context to be staging, got %q", cfg.CurrentContext)
	}
	if len(cfg.Contexts) != 2 {
		t.Fatalf("expected both contexts preserved, got %#v", cfg.Contexts)
	}
	if cfg.Contexts["default"].Server != "http://default.local:8080" || cfg.Contexts["default"].Token != "default-token" {
		t.Fatalf("default context changed unexpectedly: %#v", cfg.Contexts["default"])
	}
	if cfg.Contexts["staging"].Server != "http://staging.local:8080" || cfg.Contexts["staging"].Token != "staging-token" {
		t.Fatalf("staging context changed unexpectedly: %#v", cfg.Contexts["staging"])
	}
}

func TestConfigUseContextUnknownContextDoesNotModifyFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"default","contexts":{"default":{"server":"http://default.local:8080","token":"default-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	before, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config before: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "use-context", "staging"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `context "staging" not found`) {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}

	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config after: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("expected config file to remain unchanged\nbefore=%s\nafter=%s", before, after)
	}
}

func TestConfigSetContextCreatesConfigAndParentDirectories(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nested", "smith", "config.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--config", cfgPath,
		"config", "set-context", "default",
		"--server", "http://127.0.0.1:8080",
		"--token", "abc",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
	if stdout.String() != "Set context \"default\"\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	cfg, err := readFileConfig(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.CurrentContext != "" {
		t.Fatalf("expected empty current context, got %q", cfg.CurrentContext)
	}
	if len(cfg.Contexts) != 1 {
		t.Fatalf("expected one context, got %#v", cfg.Contexts)
	}
	if cfg.Contexts["default"].Server != "http://127.0.0.1:8080" || cfg.Contexts["default"].Token != "abc" {
		t.Fatalf("unexpected context: %#v", cfg.Contexts["default"])
	}
}

func TestConfigSetContextUpdatesSpecifiedFieldsOnly(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"default","contexts":{"default":{"server":"http://old.local:8080","token":"old-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--config", cfgPath,
		"config", "set-context", "default",
		"--server", "http://new.local:8080",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}

	cfg, err := readFileConfig(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.CurrentContext != "default" {
		t.Fatalf("expected current context preserved, got %q", cfg.CurrentContext)
	}
	if cfg.Contexts["default"].Server != "http://new.local:8080" {
		t.Fatalf("expected server update, got %#v", cfg.Contexts["default"])
	}
	if cfg.Contexts["default"].Token != "old-token" {
		t.Fatalf("expected token preserved, got %#v", cfg.Contexts["default"])
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"--config", cfgPath,
		"config", "set-context", "default",
		"--token", "new-token",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}

	cfg, err = readFileConfig(cfgPath)
	if err != nil {
		t.Fatalf("read config after token update: %v", err)
	}
	if cfg.Contexts["default"].Server != "http://new.local:8080" {
		t.Fatalf("expected server preserved, got %#v", cfg.Contexts["default"])
	}
	if cfg.Contexts["default"].Token != "new-token" {
		t.Fatalf("expected token update, got %#v", cfg.Contexts["default"])
	}
}

func TestConfigSetContextRequiresName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--config", cfgPath,
		"config", "set-context", "   ",
		"--server", "http://127.0.0.1:8080",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: smithctl config set-context <name> [--server URL] [--token TOKEN]") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
	if _, err := os.Stat(cfgPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config file to be absent, stat err=%v", err)
	}
}

func TestConfigSetContextRequiresMutableFieldAndDoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "set-context", "default"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "at least one of --server or --token must be provided") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
	if _, err := os.Stat(cfgPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config file to be absent, stat err=%v", err)
	}
}

func TestConfigSetContextWorkflowAndCurrentContextBehavior(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nested", "smith", "config.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--config", cfgPath,
		"config", "set-context", "default",
		"--server", "http://127.0.0.1:8080",
		"--token", "abc",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("set-context failed code=%d stderr=%s", code, stderr.String())
	}

	cfg, err := readFileConfig(cfgPath)
	if err != nil {
		t.Fatalf("read config after set-context: %v", err)
	}
	if cfg.CurrentContext != "" {
		t.Fatalf("expected current context to remain unset, got %q", cfg.CurrentContext)
	}
	if got := cfg.Contexts["default"]; got.Server != "http://127.0.0.1:8080" || got.Token != "abc" {
		t.Fatalf("unexpected default context after set-context: %#v", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--config", cfgPath, "config", "current-context"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected current-context to fail before use-context")
	}
	if !strings.Contains(stderr.String(), "no current context is set") {
		t.Fatalf("unexpected stderr before use-context: %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--config", cfgPath, "config", "use-context", "default"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("use-context failed code=%d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--config", cfgPath, "config", "current-context"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("current-context failed code=%d stderr=%s", code, stderr.String())
	}
	if stdout.String() != "default\n" {
		t.Fatalf("unexpected current-context output after use-context: %q", stdout.String())
	}
}

func TestConfigUseContextRequiresSingleNameAndDoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "use-context"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: smithctl config use-context <name>") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
	if _, err := os.Stat(cfgPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config file to be absent, stat err=%v", err)
	}
}

func TestConfigDeleteContextRemovesNamedContext(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"default","contexts":{"default":{"server":"http://default.local:8080","token":"default-token"},"staging":{"server":"http://staging.local:8080","token":"staging-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "delete-context", "staging"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
	if stdout.String() != "Deleted context \"staging\"\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	cfg, err := readFileConfig(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.CurrentContext != "default" {
		t.Fatalf("expected current context preserved, got %q", cfg.CurrentContext)
	}
	if len(cfg.Contexts) != 1 {
		t.Fatalf("expected one context, got %#v", cfg.Contexts)
	}
	if _, ok := cfg.Contexts["staging"]; ok {
		t.Fatalf("expected staging context to be removed: %#v", cfg.Contexts)
	}
	if cfg.Contexts["default"].Server != "http://default.local:8080" || cfg.Contexts["default"].Token != "default-token" {
		t.Fatalf("default context changed unexpectedly: %#v", cfg.Contexts["default"])
	}
}

func TestConfigDeleteContextClearsCurrentContextWhenDeleted(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"staging","contexts":{"staging":{"server":"http://staging.local:8080","token":"staging-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "delete-context", "staging"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}

	cfg, err := readFileConfig(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.CurrentContext != "" {
		t.Fatalf("expected current context to be cleared, got %q", cfg.CurrentContext)
	}
	if len(cfg.Contexts) != 0 {
		t.Fatalf("expected all contexts removed, got %#v", cfg.Contexts)
	}
}

func TestConfigDeleteContextUnknownDoesNotModifyFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"default","contexts":{"default":{"server":"http://default.local:8080","token":"default-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	before, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config before: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "delete-context", "staging"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `context "staging" not found`) {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}

	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config after: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("expected config file to remain unchanged\nbefore=%s\nafter=%s", before, after)
	}
}

func TestConfigDeleteContextRequiresSingleNameAndDoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "delete-context"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: smithctl config delete-context <name>") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
	if _, err := os.Stat(cfgPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config file to be absent, stat err=%v", err)
	}
}

func TestConfigRenameContextPreservesFieldsAndUpdatesCurrentContext(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"staging","contexts":{"default":{"server":"http://default.local:8080","token":"default-token"},"staging":{"server":"http://staging.local:8080","token":"staging-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "rename-context", "staging", "preprod"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
	if stdout.String() != "Renamed context \"staging\" to \"preprod\"\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	cfg, err := readFileConfig(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.CurrentContext != "preprod" {
		t.Fatalf("expected current context to be updated, got %q", cfg.CurrentContext)
	}
	if len(cfg.Contexts) != 2 {
		t.Fatalf("expected two contexts, got %#v", cfg.Contexts)
	}
	if _, ok := cfg.Contexts["staging"]; ok {
		t.Fatalf("expected old context key removed: %#v", cfg.Contexts)
	}
	if cfg.Contexts["preprod"].Server != "http://staging.local:8080" || cfg.Contexts["preprod"].Token != "staging-token" {
		t.Fatalf("expected renamed context values preserved, got %#v", cfg.Contexts["preprod"])
	}
	if cfg.Contexts["default"].Server != "http://default.local:8080" || cfg.Contexts["default"].Token != "default-token" {
		t.Fatalf("default context changed unexpectedly: %#v", cfg.Contexts["default"])
	}
}

func TestConfigRenameContextUnknownDoesNotModifyFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"default","contexts":{"default":{"server":"http://default.local:8080","token":"default-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	before, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config before: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "rename-context", "staging", "preprod"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `context "staging" not found`) {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}

	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config after: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("expected config file to remain unchanged\nbefore=%s\nafter=%s", before, after)
	}
}

func TestConfigRenameContextRequiresTwoNamesAndDoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "rename-context", "default"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: smithctl config rename-context <old> <new>") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
	if _, err := os.Stat(cfgPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config file to be absent, stat err=%v", err)
	}
}

func TestConfigRenameContextToExistingNameDoesNotModifyFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"current_context":"staging","contexts":{"default":{"server":"http://default.local:8080","token":"default-token"},"staging":{"server":"http://staging.local:8080","token":"staging-token"}}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	before, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config before: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", cfgPath, "config", "rename-context", "staging", "default"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `context "default" already exists`) {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}

	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config after: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("expected config file to remain unchanged\nbefore=%s\nafter=%s", before, after)
	}
}

func TestLoopCreateBatchArrayFile(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "loops.json")
	content := `[{"title":"A","source_type":"github_issue","source_ref":"org/repo#1"}]`
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "loop", "create", "--batch", filePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if _, ok := received["loops"]; !ok {
		t.Fatalf("expected loops wrapper payload, got %#v", received)
	}
}

func TestLoopCreateWithEnvironmentImageFlags(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--title", "Env", "--description", "Test", "--source-type", "interactive", "--source-ref", "terminal/session-01",
		"--env-image-ref", "ghcr.io/acme/replica:v2", "--env-image-pull-policy", "Always",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	rawEnv, ok := received["environment"].(map[string]any)
	if !ok {
		t.Fatalf("expected environment payload, got %#v", received)
	}
	rawImage, ok := rawEnv["container_image"].(map[string]any)
	if !ok {
		t.Fatalf("expected container_image payload, got %#v", rawEnv)
	}
	if rawImage["ref"] != "ghcr.io/acme/replica:v2" || rawImage["pull_policy"] != "Always" {
		t.Fatalf("unexpected container image payload: %#v", rawImage)
	}
}

func TestLoopCreateWithSkillFlags(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--title", "Skill", "--description", "Test", "--source-type", "interactive", "--source-ref", "terminal/session-01",
		"--skill", "name=commit,source=local://skills/commit",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	skillsRaw, ok := received["skills"].([]any)
	if !ok || len(skillsRaw) != 1 {
		t.Fatalf("expected one skill in payload, got %#v", received["skills"])
	}
	skill, ok := skillsRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected skill map payload, got %#v", skillsRaw[0])
	}
	if skill["name"] != "commit" || skill["source"] != "local://skills/commit" {
		t.Fatalf("unexpected skill payload: %#v", skill)
	}
	if _, exists := skill["mount_path"]; exists {
		t.Fatalf("expected mount_path to be omitted when not provided: %#v", skill)
	}
}

func TestLoopCreateWithWorkspacePRDFile(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	dir := t.TempDir()
	prdPath := filepath.Join(dir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(`{"stories":[{"id":"US-001","status":"open"}]}`), 0o600); err != nil {
		t.Fatalf("write prd file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--workspace-prd-file", prdPath, "--workspace-prompt", "refine PRD then build",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if received["source_type"] != "prompt" {
		t.Fatalf("expected source_type=prompt, got %#v", received["source_type"])
	}
	metadata, ok := received["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata map, got %#v", received["metadata"])
	}
	if metadata["workspace_prd_json"] == "" {
		t.Fatalf("expected workspace_prd_json to be set, got %#v", metadata)
	}
	if metadata["workspace_prd_path"] != ".agents/tasks/prd.json" {
		t.Fatalf("unexpected workspace_prd_path: %#v", metadata["workspace_prd_path"])
	}
}

func TestLoopCreateWithWorkspacePRDValidationErrorPrintsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "prd failed readiness validation",
			"report": map[string]any{
				"valid":     false,
				"readiness": "fail",
				"errors": []map[string]any{{
					"code":       "prd_missing_quality_gates",
					"path":       "$.qualityGates",
					"message":    "at least one quality gate is required",
					"suggestion": "Add the commands required to verify PRD work.",
				}},
			},
		})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--workspace-prd-json", `{"version":1,"project":"Validation","overview":"Canonical PRD validation","qualityGates":[],"stories":[]}`,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code=1, got %d stderr=%s", code, stderr.String())
	}
	var body map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &body); err != nil {
		t.Fatalf("expected json validation output, got %v stdout=%s", err, stdout.String())
	}
	if body["error"] != "prd failed readiness validation" {
		t.Fatalf("unexpected output: %#v", body)
	}
}

func TestLoopCreateWithProviderID(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/loops" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "create",
		"--title", "Provider", "--description", "Test", "--source-type", "prompt", "--source-ref", "prompt:provider-test",
		"--provider-id", "CoDeX",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if got, _ := received["provider_id"].(string); got != "codex" {
		t.Fatalf("expected provider_id codex, got %#v", received["provider_id"])
	}
}

func TestLoopCreateRejectsInvalidWorkspacePRDJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--output", "json", "loop", "create",
		"--workspace-prd-json", `{"stories":`,
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code=2 for invalid json, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "must be valid json") {
		t.Fatalf("expected invalid json error, got stderr=%s", stderr.String())
	}
}

func TestPRDSubmitJSONSendsCanonicalPRD(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/ingress/prd" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer srv.Close()

	dir := t.TempDir()
	prdPath := filepath.Join(dir, "prd.json")
	if err := os.WriteFile(prdPath, []byte(`{"version":1,"project":"Validation","overview":"Canonical PRD validation","qualityGates":["go test ./..."],"stories":[]}`), 0o600); err != nil {
		t.Fatalf("write prd file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "prd", "submit", "--file", prdPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if received["prd"] == nil {
		t.Fatalf("expected canonical prd payload, got %#v", received)
	}
	if received["tasks"] != nil {
		t.Fatalf("did not expect legacy tasks payload, got %#v", received)
	}
}

func TestLoopCreateEnvironmentSourceConflict(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--output", "json", "loop", "create",
		"--title", "Env", "--description", "Test", "--source-type", "interactive", "--source-ref", "terminal/session-01",
		"--env-image-ref", "ghcr.io/acme/replica:v2",
		"--env-docker-context", ".", "--env-dockerfile", "Dockerfile",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code=2 for source conflict, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "environment source conflict") {
		t.Fatalf("expected conflict error, got stderr=%s", stderr.String())
	}
}

func TestLoopGetBatchIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/loops/") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/loops/")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state": map[string]any{
				"loop_id": id,
				"state":   "unresolved",
			},
		})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "loop", "get", "loop-a", "loop-b"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	results, ok := out["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("unexpected results: %#v", out)
	}
}

func TestLoopTraceBatchIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/loops/") || !strings.HasSuffix(r.URL.Path, "/trace") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/loops/"), "/trace")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"loop_id": id,
			"state": map[string]any{
				"loop_id": id,
				"state":   "synced",
			},
			"journal": []any{},
		})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--server", srv.URL, "--output", "json", "loop", "trace", "loop-a", "loop-b"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	results, ok := out["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("unexpected results: %#v", out)
	}
}

func TestLoopCancelBatchPostsOverride(t *testing.T) {
	var calls []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/control/override" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		calls = append(calls, payload)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "cancel",
		"--reason", "test-reason", "--actor", "test-actor", "loop-a", "loop-b",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	for _, payload := range calls {
		if payload["target_state"] != "cancelled" {
			t.Fatalf("expected target_state=cancelled, got %#v", payload)
		}
		if payload["reason"] != "test-reason" {
			t.Fatalf("expected reason, got %#v", payload)
		}
	}
}

func TestLoopDetachPostsControlDetach(t *testing.T) {
	var calls []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/control/detach") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		calls = append(calls, payload)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "detach",
		"--actor", "test-actor", "loop-a",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0]["actor"] != "test-actor" {
		t.Fatalf("expected actor=test-actor, got %#v", calls[0])
	}
}

func TestLoopCommandPostsControlCommand(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/control/command") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "command",
		"loop-a", "--actor", "test-actor", "--command", "pause",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if got["actor"] != "test-actor" || got["command"] != "pause" {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

func TestLoopAttachFallsBackWhenEndpointMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/control/attach"):
			http.Error(w, "endpoint not found", http.StatusNotFound)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/journal"):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"sequence": 1, "message": "entry"},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/loops/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"state": map[string]any{"state": string(model.LoopStateCancelled)},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "loop", "attach",
		"--interval", (10 * time.Millisecond).String(), "--follow=false", "loop-a",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "not_supported") {
		t.Fatalf("expected attach fallback output, got %s", stdout.String())
	}
}

func TestPRDCreateTemplateToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prd.md")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--output", "json", "prd", "create", "Auth Flow", "--template", "feature", "--out", path,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(content), "## Goal") {
		t.Fatalf("expected feature template content, got: %s", string(content))
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out["template"] != "feature" {
		t.Fatalf("expected feature template metadata, got %#v", out)
	}
}

func TestPRDSubmitIncludesLoopIDsAndValidationErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/ingress/prd" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"item_index": 0, "loop_id": "loop-1", "status": "unresolved", "created": true},
				{"item_index": 1, "status": "error", "message": "task title is required", "source_ref": "prd:doc#x"},
			},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	prdPath := filepath.Join(dir, "prd.md")
	if err := os.WriteFile(prdPath, []byte("# Test PRD\n- [ ] One task"), 0o600); err != nil {
		t.Fatalf("write prd file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--server", srv.URL, "--output", "json", "prd", "submit", "--file", prdPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed code=%d stderr=%s", code, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	loopIDs, ok := out["loop_ids"].([]any)
	if !ok || len(loopIDs) != 1 || loopIDs[0] != "loop-1" {
		t.Fatalf("expected loop_ids [loop-1], got %#v", out["loop_ids"])
	}
	errs, ok := out["validation_errors"].([]any)
	if !ok || len(errs) != 1 {
		t.Fatalf("expected one validation error, got %#v", out["validation_errors"])
	}
}
