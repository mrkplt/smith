package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"smith/internal/source/docstore"
	"smith/internal/source/ingress"
	"smith/internal/source/model"
	"smith/internal/source/provider"
	"smith/internal/source/store"
	api "smith/pkg/api/v1"
	pb "smith/proto/v1"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	_ "smith/docs"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	corev1 "k8s.io/api/core/v1"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	kexec "k8s.io/client-go/util/exec"
)

const (
	defaultPort            = 8080
	defaultGRPCPort        = 8081
	defaultShutdownTimeout = 10 * time.Second

	defaultRuntimeReason    = "runtime pod not found"
	terminalCommandMaxSize  = 2048
	terminalCommandRateMax  = 5
	terminalErrUnauthorized = "terminal_unauthorized"
	terminalErrTooLong      = "terminal_command_too_long"
	terminalErrRateLimited  = "terminal_command_rate_limited"
	terminalErrNotAttached  = "terminal_actor_not_attached"
	terminalErrInvalidJSON  = "terminal_invalid_json"
	terminalErrRequiredCmd  = "terminal_command_required"
)

var (
	terminalCommandRateWindow = 10 * time.Second
)

type config struct {
	port          int
	grpcPort      int
	etcdEndpoints []string

	etcdDialTimeout                     time.Duration
	operatorToken                       string
	authStoreBackend                    string
	authStorePath                       string
	authStoreK8sNamespace               string
	authStoreK8sSecret                  string
	authStoreK8sKey                     string
	defaultPreset                       string
	skillPolicy                         model.SkillPolicy
	runtimeNamespace                    string
	runtimeContainerName                string
	providerClaudeEnabled               bool
	providerGeminiEnabled               bool
	documentStoreBackend                string
	documentsPostgresDSN                string
	documentsPostgresMaxConns           int32
	documentsGarageEndpoint             string
	documentsGarageRegion               string
	documentsGarageBucket               string
	documentsGarageAccessKeyID          string
	documentsGarageSecretAccessKey      string
	documentsGarageForcePathStyle       bool
	documentsWatchPollInterval          time.Duration
	documentsMigrationBackfillOnStartup bool
	documentsMigrationReadThroughOnMiss bool
	documentsMigrationMergeListFallback bool
	documentsMigrationDualWriteEtcd     bool
}

type server struct {
	cfg          config
	store        store.StateStore
	documents    docstore.Store
	modelCatalog provider.ModelInventoryService
	projectCred  provider.ProjectCredentialStore
	repoAccess   func(ctx context.Context, repoURL string, githubUser string, credential string) (bool, string, error)
	providers    provider.ProviderProfileStore
	secrets      provider.SecretStore
	projectStore provider.ProjectStore
	presets      *presetCatalog
	skillPolicy  model.SkillPolicy
	term         *terminalSessionStore
	runtimePods  runtimePodReader
	podExec      podExecRunner
	kube         kubernetes.Interface
	restConfig   *rest.Config
}
type overrideRequest = api.OverrideRequest
type costSummary = api.CostSummary
type projectCredentialUpsertRequest = api.ProjectCredentialUpsertRequest
type projectCredentialDeleteRequest = api.ProjectCredentialDeleteRequest
type projectCredentialTestRequest = api.ProjectCredentialTestRequest
type projectCredentialTestResponse = api.ProjectCredentialTestResponse
type onboardingRepositoryRequest = api.OnboardingRepositoryRequest
type onboardingCredentialValidateRequest = api.OnboardingCredentialValidateRequest
type onboardingCredentialStatus = api.OnboardingCredentialStatus
type onboardingRequirement = api.OnboardingRequirement
type onboardingReadinessResponse = api.OnboardingReadinessResponse
type terminalAttachRequest = api.TerminalAttachRequest
type terminalDetachRequest = api.TerminalDetachRequest
type terminalCommandRequest = api.TerminalCommandRequest
type loopRuntimeResponse = api.LoopRuntimeResponse
type loopDeleteRequest = api.LoopDeleteRequest
type loopCleanupRequest = api.LoopCleanupRequest
type loopCleanupResponse = api.LoopCleanupResponse
type documentRequest = api.DocumentRequest
type documentBuildRequest = api.DocumentBuildRequest
type taskContractCreateRequest = api.TaskContractCreateRequest
type taskContractPatchRequest = api.TaskContractPatchRequest
type taskContractApproveRequest = api.TaskContractApproveRequest
type loopLifecycleRequest = api.LoopLifecycleRequest
type loopInterventionRequest = api.LoopInterventionRequest
type loopInterventionResponse = api.LoopInterventionResponse
type loopCreateRequest = api.LoopCreateRequest
type loopBatchRequest = api.LoopBatchRequest
type loopCreateResult = api.LoopCreateResult
type githubIngressRequest = api.GitHubIngressRequest

type githubWebhookPayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		ID      int64  `json:"id"`
	} `json:"issue"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

type prdIngressRequest = api.PRDIngressRequest
type prdValidateRequest = api.PRDValidateRequest
type prdValidateResponse = api.PRDValidateResponse
type ingressResult = api.IngressResult
type ingressSummary = api.IngressSummary
type loopTraceResponse = api.LoopTraceResponse
type loopResponse = api.LoopResponse
type overrideResponse = api.OverrideResponse
type presetCreateRequest = api.PresetCreateRequest

type presetCatalog struct {
	mu            sync.RWMutex
	defaultPreset string
	presets       map[string]struct{}
}

type terminalSessionStore struct {
	mu           sync.Mutex
	sessions     map[string]map[string]terminalSession
	attachCounts map[string]map[string]int
}

type terminalSession struct {
	Actor                string
	Terminal             string
	Status               string
	AttachedAt           time.Time
	LastActivityAt       time.Time
	AttachCount          int
	CommandWindowStarted time.Time
	CommandWindowCount   int
	RuntimeTargetRef     string
	RuntimeNamespace     string
	RuntimePodName       string
	RuntimeContainerName string
	RuntimePodPhase      string
}

type runtimePodReader interface {
	List(ctx context.Context, namespace string, opts metav1.ListOptions) (*corev1.PodList, error)
}

type podExecRequest struct {
	Namespace     string
	PodName       string
	ContainerName string
	Command       string
}

type podExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type podExecRunner interface {
	Execute(ctx context.Context, req podExecRequest) (podExecResult, error)
}

type kubeRuntimePodReader struct {
	kube kubernetes.Interface
}

type kubePodExecRunner struct {
	kube        kubernetes.Interface
	restConfig  *rest.Config
	newExecutor func(*rest.Config, string, *url.URL) (remotecommand.Executor, error)
}

func (k kubeRuntimePodReader) List(ctx context.Context, namespace string, opts metav1.ListOptions) (*corev1.PodList, error) {
	return k.kube.CoreV1().Pods(namespace).List(ctx, opts)
}

func (k kubePodExecRunner) Execute(ctx context.Context, req podExecRequest) (podExecResult, error) {
	out := podExecResult{}
	if k.kube == nil || k.restConfig == nil {
		return out, errors.New("kubernetes pod exec is unavailable")
	}

	namespace := strings.TrimSpace(req.Namespace)
	podName := strings.TrimSpace(req.PodName)
	containerName := strings.TrimSpace(req.ContainerName)
	command := strings.TrimSpace(req.Command)
	if namespace == "" || podName == "" || command == "" {
		return out, errors.New("runtime target and command are required")
	}

	execRequest := k.kube.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"/bin/sh", "-lc", command},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, kubescheme.ParameterCodec)

	newExecutor := k.newExecutor
	if newExecutor == nil {
		newExecutor = remotecommand.NewSPDYExecutor
	}
	executor, err := newExecutor(k.restConfig, http.MethodPost, execRequest.URL())
	if err != nil {
		return out, err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	streamErr := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	out.Stdout = stdout.String()
	out.Stderr = stderr.String()

	if streamErr == nil {
		out.ExitCode = 0
		return out, nil
	}
	var exitErr kexec.ExitError
	if errors.As(streamErr, &exitErr) {
		out.ExitCode = exitErr.ExitStatus()
		return out, nil
	}
	return out, streamErr
}

func newTerminalSessionStore() *terminalSessionStore {
	return &terminalSessionStore{
		sessions:     map[string]map[string]terminalSession{},
		attachCounts: map[string]map[string]int{},
	}
}

func (t *terminalSessionStore) Attach(loopID, actor, terminal string, runtime loopRuntimeResponse) (terminalSession, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sessions[loopID] == nil {
		t.sessions[loopID] = map[string]terminalSession{}
	}
	if t.attachCounts[loopID] == nil {
		t.attachCounts[loopID] = map[string]int{}
	}
	now := time.Now().UTC()
	t.attachCounts[loopID][actor]++
	attachCount := t.attachCounts[loopID][actor]
	session := terminalSession{
		Actor:                actor,
		Terminal:             terminal,
		Status:               "attached",
		AttachedAt:           now,
		LastActivityAt:       now,
		AttachCount:          attachCount,
		CommandWindowStarted: now,
		CommandWindowCount:   0,
		RuntimeTargetRef:     runtimeTargetReference(runtime.Namespace, runtime.PodName, runtime.ContainerName),
		RuntimeNamespace:     runtime.Namespace,
		RuntimePodName:       runtime.PodName,
		RuntimeContainerName: runtime.ContainerName,
		RuntimePodPhase:      runtime.PodPhase,
	}
	t.sessions[loopID][actor] = session
	return session, len(t.sessions[loopID])
}

func (t *terminalSessionStore) Detach(loopID, actor string) (terminalSession, bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	actors, ok := t.sessions[loopID]
	if !ok {
		return terminalSession{}, false, 0
	}
	session, found := actors[actor]
	if !found {
		return terminalSession{}, false, len(actors)
	}
	session.Status = "detached"
	session.LastActivityAt = time.Now().UTC()
	delete(actors, actor)
	if len(actors) == 0 {
		delete(t.sessions, loopID)
		return session, true, 0
	}
	return session, true, len(actors)
}

func (t *terminalSessionStore) IsAttached(loopID, actor string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	actors, ok := t.sessions[loopID]
	if !ok {
		return false
	}
	_, found := actors[actor]
	return found
}

func (t *terminalSessionStore) Session(loopID, actor string) (terminalSession, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	actors, ok := t.sessions[loopID]
	if !ok {
		return terminalSession{}, false
	}
	session, found := actors[actor]
	if !found {
		return terminalSession{}, false
	}
	session.LastActivityAt = time.Now().UTC()
	actors[actor] = session
	return session, true
}

func (t *terminalSessionStore) ConsumeCommandSlot(loopID, actor string, now time.Time) (terminalSession, bool, bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	actors, ok := t.sessions[loopID]
	if !ok {
		return terminalSession{}, false, false, 0
	}
	session, found := actors[actor]
	if !found {
		return terminalSession{}, false, false, 0
	}
	if session.CommandWindowStarted.IsZero() || now.Sub(session.CommandWindowStarted) >= terminalCommandRateWindow {
		session.CommandWindowStarted = now
		session.CommandWindowCount = 0
	}
	elapsed := now.Sub(session.CommandWindowStarted)
	if elapsed < 0 {
		elapsed = 0
	}
	if session.CommandWindowCount >= terminalCommandRateMax {
		retryAfter := terminalCommandRateWindow - elapsed
		if retryAfter < 0 {
			retryAfter = 0
		}
		session.LastActivityAt = now
		actors[actor] = session
		return session, true, false, retryAfter
	}
	session.CommandWindowCount++
	session.LastActivityAt = now
	actors[actor] = session
	return session, true, true, 0
}

func runtimeTargetReference(namespace, podName, containerName string) string {
	namespace = strings.TrimSpace(namespace)
	podName = strings.TrimSpace(podName)
	containerName = strings.TrimSpace(containerName)
	if podName == "" {
		return ""
	}
	ref := podName
	if namespace != "" {
		ref = namespace + "/" + ref
	}
	if containerName != "" {
		ref += ":" + containerName
	}
	return ref
}

func terminalSessionMetadata(actor string, session terminalSession, activeAttachCount int) map[string]string {
	return map[string]string{
		"actor":               actor,
		"terminal":            session.Terminal,
		"attach_count":        strconv.Itoa(session.AttachCount),
		"active_attach_count": strconv.Itoa(activeAttachCount),
		"session_status":      session.Status,
		"runtime_target_ref":  session.RuntimeTargetRef,
		"runtime_namespace":   session.RuntimeNamespace,
		"runtime_pod":         session.RuntimePodName,
		"runtime_container":   session.RuntimeContainerName,
		"runtime_phase":       session.RuntimePodPhase,
	}
}

func terminalAcceptedMetadata(metadata map[string]string) map[string]string {
	out := copyStringMap(metadata)
	out["request_status"] = "accepted"
	return out
}

func terminalRejectedMetadata(metadata map[string]string, reason, errorCode string) map[string]string {
	out := copyStringMap(metadata)
	out["request_status"] = "rejected"
	out["rejection_reason"] = reason
	if strings.TrimSpace(errorCode) != "" {
		out["error_code"] = errorCode
	}
	return out
}

func (s *server) getState(ctx context.Context, loopID string) (store.LoopWithRevision, bool, error) {
	return s.store.GetState(ctx, loopID)
}
func (s *server) appendAudit(ctx context.Context, rec store.AuditRecord) error {
	if s.store == nil {
		return nil
	}
	return s.store.AppendAudit(ctx, rec)
}
func (s *server) appendJournal(ctx context.Context, entry model.JournalEntry) error {
	if s.store == nil {
		return nil
	}
	return s.store.AppendJournal(ctx, entry)
}

func (s *server) documentStore() docstore.Store {
	if s.documents != nil {
		return s.documents
	}
	if s.store != nil {
		return docstore.NewEtcdStore(s.store)
	}
	return nil
}

// @title Smith API
// @version 1.0
// @description Smith API server for managing loops and anomalies.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("smith-api config error: %v", err)
	}

	es, err := store.New(ctx, cfg.etcdEndpoints, cfg.etcdDialTimeout)
	if err != nil {
		log.Fatalf("smith-api etcd init failed: %v", err)
	}
	defer func() { _ = es.Close() }()

	var documentStore docstore.Store = docstore.NewEtcdStore(es)
	if docstore.IsPostgresGarageBackend(cfg.documentStoreBackend) {
		primaryDocumentStore, primaryErr := docstore.NewPostgresGarageStore(ctx, docstore.PostgresGarageConfig{
			PostgresDSN:           cfg.documentsPostgresDSN,
			PostgresMaxConns:      cfg.documentsPostgresMaxConns,
			GarageEndpoint:        cfg.documentsGarageEndpoint,
			GarageRegion:          cfg.documentsGarageRegion,
			GarageBucket:          cfg.documentsGarageBucket,
			GarageAccessKeyID:     cfg.documentsGarageAccessKeyID,
			GarageSecretAccessKey: cfg.documentsGarageSecretAccessKey,
			GarageForcePathStyle:  cfg.documentsGarageForcePathStyle,
			WatchPollInterval:     cfg.documentsWatchPollInterval,
		})
		if primaryErr != nil {
			log.Fatalf("smith-api document store init failed: %v", primaryErr)
		}
		fallbackDocumentStore := docstore.NewEtcdStore(es)

		if cfg.documentsMigrationBackfillOnStartup {
			summary, backfillErr := docstore.BackfillDocuments(ctx, primaryDocumentStore, fallbackDocumentStore)
			if backfillErr != nil {
				log.Fatalf("smith-api document backfill failed: %v", backfillErr)
			}
			if summary.Upserted > 0 {
				log.Printf("smith-api document backfill completed: scanned=%d upserted=%d", summary.Scanned, summary.Upserted)
			}
		}

		documentStore = docstore.NewMigratingStore(primaryDocumentStore, fallbackDocumentStore, docstore.MigrationOptions{
			DualWriteFallback: cfg.documentsMigrationDualWriteEtcd,
			ReadThroughOnMiss: cfg.documentsMigrationReadThroughOnMiss,
			MergeListFallback: cfg.documentsMigrationMergeListFallback,
		})
		defer func() { _ = documentStore.Close() }()
	}

	tokenStore, err := newTokenStore(ctx, cfg)
	if err != nil {
		log.Fatalf("smith-api auth store init failed: %v", err)
	}
	projectCredStore, ok := tokenStore.(provider.ProjectCredentialStore)
	if !ok {
		log.Fatalf("smith-api auth store does not support project credentials")
	}

	projectStore, err := newProjectStore(ctx, cfg)
	if err != nil {
		log.Fatalf("smith-api project store init failed: %v", err)
	}
	providerStore, err := newProviderProfileStore(ctx, cfg)
	if err != nil {
		log.Fatalf("smith-api provider profile store init failed: %v", err)
	}
	secretStore, err := newSecretStore(ctx, cfg)
	if err != nil {
		log.Fatalf("smith-api secret store init failed: %v", err)
	}

	runtimePods, err := newRuntimePodReader()
	if err != nil {
		log.Printf("smith-api runtime pod lookup unavailable: %v", err)
	}
	podExec, err := newPodExecRunner()
	if err != nil {
		log.Printf("smith-api pod exec unavailable: %v", err)
	}

	kube, restConfig, err := kubeClientWithConfig()
	if err != nil {
		log.Printf("smith-api kubernetes client unavailable: %v", err)
	}

	s := &server{
		cfg:          cfg,
		store:        es,
		documents:    documentStore,
		modelCatalog: provider.NewAccountModelInventoryService(),
		projectCred:  projectCredStore,
		providers:    providerStore,
		secrets:      secretStore,
		projectStore: projectStore,
		presets:      newPresetCatalog(cfg.defaultPreset),
		skillPolicy:  cfg.skillPolicy,
		term:         newTerminalSessionStore(),
		runtimePods:  runtimePods,
		podExec:      podExec,
		kube:         kube,
		restConfig:   restConfig,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc("/api/loops", s.handleLoops)
	mux.HandleFunc("/api/loops/cleanup", s.handleLoopCleanup)
	mux.HandleFunc("/api/loops/", s.handleLoopByID)
	mux.HandleFunc("/v1/loops", s.handleLoops)
	mux.HandleFunc("/v1/loops/cleanup", s.handleLoopCleanup)
	mux.HandleFunc("/v1/loops/stream", s.handleLoopStream)
	mux.HandleFunc("/v1/loops/", s.handleLoopByID)
	mux.HandleFunc("/v1/environment/presets", s.handleEnvironmentPresets)
	mux.HandleFunc("/v1/environment/presets/", s.handleEnvironmentPresetByName)
	mux.HandleFunc("/v1/ingress/github/issues", s.handleIngressGitHubIssues)
	mux.HandleFunc("/v1/webhooks/github/issues", s.handleGitHubWebhook)
	mux.HandleFunc("/v1/ingress/prd", s.handleIngressPRD)
	mux.HandleFunc("/v1/prd/validate", s.handlePRDValidate)
	mux.HandleFunc("/v1/control/override", s.handleOverride)
	mux.HandleFunc("/v1/audit", s.handleAudit)
	mux.HandleFunc("/v1/audit/stream", s.handleAuditStream)
	mux.HandleFunc("/v1/reporting/cost", s.handleCost)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTaskByID)
	mux.HandleFunc("/v1/tasks", s.handleTasks)
	mux.HandleFunc("/v1/tasks/", s.handleTaskByID)
	mux.HandleFunc("/v1/documents", s.handleDocuments)
	mux.HandleFunc("/v1/documents/stream", s.handleDocumentStream)
	mux.HandleFunc("/v1/documents/", s.handleDocumentByID)
	mux.HandleFunc("/api/projects/credentials/github", s.handleProjectGitHubCredential)
	mux.HandleFunc("/api/projects/credentials/github/test", s.handleProjectGitHubCredentialTest)
	mux.HandleFunc("/v1/projects/credentials/github", s.handleProjectGitHubCredential)
	mux.HandleFunc("/v1/projects/credentials/github/test", s.handleProjectGitHubCredentialTest)
	mux.HandleFunc("/api/providers/catalog", s.handleProviderCatalog)
	mux.HandleFunc("/api/providers", s.handleProviders)
	mux.HandleFunc("/api/providers/", s.handleProviderByID)
	mux.HandleFunc("/v1/providers/catalog", s.handleProviderCatalog)
	mux.HandleFunc("/v1/providers", s.handleProviders)
	mux.HandleFunc("/v1/providers/", s.handleProviderByID)
	mux.HandleFunc("/v1/secrets", s.handleSecrets)
	mux.HandleFunc("/v1/secrets/", s.handleSecretByID)
	mux.HandleFunc("/api/onboarding/readiness", s.handleOnboardingReadiness)
	mux.HandleFunc("/api/onboarding/repository", s.handleOnboardingRepository)
	mux.HandleFunc("/api/onboarding/credentials/validate", s.handleOnboardingCredentialValidate)
	mux.HandleFunc("/v1/onboarding/readiness", s.handleOnboardingReadiness)
	mux.HandleFunc("/v1/onboarding/repository", s.handleOnboardingRepository)
	mux.HandleFunc("/v1/onboarding/credentials/validate", s.handleOnboardingCredentialValidate)
	mux.HandleFunc("/api/projects", s.handleProjects)
	mux.HandleFunc("/api/projects/", s.handleProjectByID)
	mux.HandleFunc("/v1/projects", s.handleProjects)
	mux.HandleFunc("/v1/projects/", s.handleProjectByID)

	mux.Handle("/swagger/", httpSwagger.WrapHandler)

	// gRPC Server
	gs := &grpcServer{
		store:       es,
		presets:     s.presets,
		skillPolicy: s.skillPolicy,
	}
	grpcAddr := fmt.Sprintf(":%d", cfg.grpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen for gRPC: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterSmithServiceServer(grpcServer, gs)

	go func() {
		log.Printf("smith-api gRPC listening on %s", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server failed: %v", err)
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.port)

	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("smith-api listening on %s", addr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Printf("smith-api shutdown requested")
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("smith-api failed: %v", serveErr)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("smith-api shutdown failed: %v", err)
	}
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *server) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// @Summary List or create loops
// @Description Get all loop states or create a new loop
// @Tags loops
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body loopCreateRequest false "Create Loop Request"
// @Success 200 {array} api.LoopWithRevision "List Loops"
// @Success 201 {object} api.LoopCreateResult "Create Loop"
// @Router /v1/loops [get]
// @Router /v1/loops [post]
func (s *server) handleLoops(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		states, err := s.store.ListStates(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload := make([]api.LoopWithRevision, 0, len(states))
		for _, loop := range states {
			apiState := modelToApiState(loop.Record)
			anomaly, found, getErr := s.store.GetAnomaly(r.Context(), loop.Record.LoopID)
			if getErr != nil {
				writeErr(w, http.StatusInternalServerError, getErr.Error())
				return
			}
			if found {
				enrichLoopStateForPresentation(&apiState, &anomaly)
			} else {
				enrichLoopStateForPresentation(&apiState, nil)
			}
			payload = append(payload, api.LoopWithRevision{Record: apiState, Revision: loop.Revision})
		}
		writeJSON(w, http.StatusOK, payload)
	case http.MethodPost:
		s.handleLoopCreate(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleLoopCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req loopCleanupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	stateFilter, err := parseLoopCleanupStateFilter(req.States)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	requestedIDs := normalizeUniqueLoopIDs(req.LoopIDs)
	if len(requestedIDs) == 0 && len(stateFilter) == 0 {
		writeErr(w, http.StatusBadRequest, "loop_ids or states selector is required")
		return
	}

	allStates, err := s.store.ListStates(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	requestSet := make(map[string]struct{}, len(requestedIDs))
	for _, id := range requestedIDs {
		requestSet[id] = struct{}{}
	}

	candidates := make(map[string]model.StateRecord)
	foundRequested := make(map[string]struct{}, len(requestedIDs))
	for _, loop := range allStates {
		record := loop.Record
		_, requested := requestSet[record.LoopID]
		_, stateMatched := stateFilter[record.State]
		if requested || stateMatched {
			candidates[record.LoopID] = record
		}
		if requested {
			foundRequested[record.LoopID] = struct{}{}
		}
	}

	notFound := make([]string, 0)
	for _, id := range requestedIDs {
		if _, ok := foundRequested[id]; !ok {
			notFound = append(notFound, id)
		}
	}
	sort.Strings(notFound)

	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}

	matched := make([]string, 0, len(candidates))
	for id := range candidates {
		matched = append(matched, id)
	}
	sort.Strings(matched)

	deleted := make([]string, 0, len(matched))
	skippedActive := make([]string, 0)
	for _, id := range matched {
		record := candidates[id]
		if isActiveLoopState(record.State) {
			skippedActive = append(skippedActive, id)
			continue
		}
		if err := s.store.DeleteLoop(r.Context(), id); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		runtimeCleanup := "skipped"
		if err := s.cleanupLoopRuntimeArtifacts(r.Context(), record); err != nil {
			runtimeCleanup = "error"
		}
		_ = s.store.AppendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "delete-loop",
			TargetLoopID:  id,
			CorrelationID: record.CorrelationID,
			Metadata: map[string]string{
				"final_state": string(record.State),
				"cleanup":     "true",
				"runtime":     runtimeCleanup,
			},
		})
		deleted = append(deleted, id)
	}

	writeJSON(w, http.StatusOK, loopCleanupResponse{
		Actor:         actor,
		MatchedCount:  len(matched),
		DeletedCount:  len(deleted),
		Deleted:       deleted,
		SkippedActive: skippedActive,
		NotFound:      notFound,
	})
}

func normalizeUniqueLoopIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func parseLoopCleanupStateFilter(states []api.LoopState) (map[model.LoopState]struct{}, error) {
	if len(states) == 0 {
		return nil, nil
	}
	out := make(map[model.LoopState]struct{}, len(states))
	for _, raw := range states {
		value := model.LoopState(strings.ToLower(strings.TrimSpace(string(raw))))
		switch value {
		case model.LoopStateUnresolved, model.LoopStateRunning, model.LoopStateSynced, model.LoopStateFlatline, model.LoopStateCancelled:
			out[value] = struct{}{}
		default:
			return nil, fmt.Errorf("invalid loop state selector %q", raw)
		}
	}
	return out, nil
}

func (s *server) cleanupLoopRuntimeArtifacts(ctx context.Context, record model.StateRecord) error {
	if s.kube == nil {
		return nil
	}
	jobName := strings.TrimSpace(record.WorkerJobName)
	if jobName == "" {
		return nil
	}
	namespace := runtimeNamespaceForConfig(s.cfg)

	propagation := metav1.DeletePropagationBackground
	if err := s.kube.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	pods, err := s.kube.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "job-name=" + jobName})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		if err := s.kube.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (s *server) handleLoopCreate(w http.ResponseWriter, r *http.Request) {
	raw, err := ioReadAllLimit(r.Body, 1<<20)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var batch loopBatchRequest
	if err := json.Unmarshal(raw, &batch); err == nil && len(batch.Loops) > 0 {
		results := make([]loopCreateResult, 0, len(batch.Loops))
		for _, req := range batch.Loops {
			res := s.createOneLoop(r.Context(), req)
			results = append(results, res)
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results})
		return
	}

	var single loopCreateRequest
	if err := json.Unmarshal(raw, &single); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(single.IdempotencyKey) == "" {
		single.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	}
	result := s.createOneLoop(r.Context(), single)
	code := http.StatusCreated
	if !result.Created {
		code = http.StatusOK
	}
	if result.HTTPCode != 0 {
		code = result.HTTPCode
	}
	writeJSON(w, code, result)
}

func (s *server) handleIngressGitHubIssues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req githubIngressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if len(req.Issues) == 0 {
		writeErr(w, http.StatusBadRequest, "at least one issue is required")
		return
	}
	results := make([]ingressResult, 0, len(req.Issues))
	for i, issue := range req.Issues {
		draft, err := ingress.GitHubIssueToDraft(issue)
		if err != nil {
			results = append(results, ingressResult{
				ItemIndex: i,
				Status:    "error",
				Message:   err.Error(),
			})
			continue
		}
		metadata := copyStringMap(req.Metadata)
		for k, v := range draft.Metadata {
			metadata[k] = v
		}
		res := s.createOneLoop(r.Context(), loopCreateRequest{
			IdempotencyKey: draft.IdempotencyKey,
			Title:          draft.Title,
			Description:    draft.Description,
			SourceType:     draft.SourceType,
			SourceRef:      draft.SourceRef,
			Metadata:       metadata,
		})
		results = append(results, ingressResult{
			ItemIndex: i,
			LoopID:    res.LoopID,
			SourceRef: draft.SourceRef,
			Status:    res.Status,
			Created:   res.Created,
			Message:   res.Message,
		})
	}
	writeJSON(w, http.StatusOK, newIngressSummary(results))
}

func (s *server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	event := r.Header.Get("X-GitHub-Event")
	if event != "issues" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "not-an-issue-event"})
		return
	}
	var payload githubWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if payload.Action != "opened" && payload.Action != "reopened" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "not-an-opened-issue"})
		return
	}

	issue := api.GitHubIssue{
		Repository: payload.Repository.FullName,
		Number:     payload.Issue.Number,
		Title:      payload.Issue.Title,
		Body:       payload.Issue.Body,
		URL:        payload.Issue.HTMLURL,
		ID:         strconv.FormatInt(payload.Issue.ID, 10),
	}

	draft, err := ingress.GitHubIssueToDraft(issue)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	res := s.createOneLoop(r.Context(), loopCreateRequest{
		IdempotencyKey: draft.IdempotencyKey,
		Title:          draft.Title,
		Description:    draft.Description,
		SourceType:     draft.SourceType,
		SourceRef:      draft.SourceRef,
		Metadata:       draft.Metadata,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  res.Status,
		"loop_id": res.LoopID,
		"message": res.Message,
	})
}

func (s *server) handleIngressPRD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req prdIngressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		if strings.TrimSpace(req.Markdown) != "" {
			format = "markdown"
		} else {
			format = "json"
		}
	}
	baseMetadata := copyStringMap(req.Metadata)
	drafts, report, err := buildPRDIngressDrafts(format, req.Markdown, req.PRD, req.SourceRef, baseMetadata)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if report != nil && !report.Valid {
		writePRDValidationFailure(w, *report)
		return
	}

	results := make([]ingressResult, 0, len(drafts))
	for i, draft := range drafts {
		idempotencyKey := strings.TrimSpace(draft.IdempotencyKey)
		if idempotencyKey == "" {
			idempotencyKey = fmt.Sprintf("%s#%d", strings.TrimSpace(draft.SourceRef), i)
		}
		metadata := copyStringMap(draft.Metadata)
		taskContractID := ""
		if strings.EqualFold(strings.TrimSpace(draft.SourceType), "prd_story") {
			var bindErr error
			taskContractID, metadata, bindErr = s.ensurePRDStoryTaskContract(r.Context(), draft, idempotencyKey)
			if bindErr != nil {
				results = append(results, ingressResult{
					ItemIndex: i,
					SourceRef: draft.SourceRef,
					Status:    "error",
					Created:   false,
					Message:   bindErr.Error(),
				})
				continue
			}
		}
		res := s.createOneLoop(r.Context(), loopCreateRequest{
			IdempotencyKey: idempotencyKey,
			TaskContractID: taskContractID,
			Title:          draft.Title,
			Description:    draft.Description,
			SourceType:     draft.SourceType,
			SourceRef:      draft.SourceRef,
			Metadata:       metadata,
		})
		results = append(results, ingressResult{
			ItemIndex: i,
			LoopID:    res.LoopID,
			SourceRef: draft.SourceRef,
			Status:    res.Status,
			Created:   res.Created,
			Message:   res.Message,
		})
	}
	writeJSON(w, http.StatusOK, newIngressSummary(results))
}

func (s *server) handlePRDValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req prdValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		trimmedMarkdown := strings.TrimSpace(req.Markdown)
		trimmedJSON := strings.TrimSpace(req.JSON)
		switch {
		case trimmedJSON != "" || len(req.PRD) > 0:
			format = "json"
		case trimmedMarkdown != "":
			format = "markdown"
		default:
			writeErr(w, http.StatusBadRequest, "markdown or json input is required")
			return
		}
	}

	out := prdValidateResponse{Format: format}
	switch format {
	case "markdown", "md":
		_, out.Report = model.ValidatePRDMarkdown([]byte(req.Markdown))
		out.Format = "markdown"
	case "json", "structured":
		rawJSON := req.PRD
		if strings.TrimSpace(req.JSON) != "" {
			rawJSON = json.RawMessage(req.JSON)
		}
		if len(rawJSON) == 0 {
			writeErr(w, http.StatusBadRequest, "json input is required")
			return
		}
		prd, report := model.ValidatePRDJSON(rawJSON)
		out.Report = report
		out.Format = "json"
		if report.Valid {
			if markdown, markdownReport := prd.RenderMarkdown(); markdownReport.Valid {
				out.CanonicalMarkdown = markdown
			}
		}
	default:
		writeErr(w, http.StatusBadRequest, "format must be markdown or json")
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func buildPRDIngressDrafts(format, markdown string, rawPRD json.RawMessage, sourceRef string, metadata map[string]string) ([]ingress.LoopDraft, *model.PRDValidationReport, error) {
	switch format {
	case "markdown", "md":
		prd, report := model.ValidatePRDMarkdown([]byte(markdown))
		if !report.Valid {
			return nil, &report, nil
		}
		return ingress.CanonicalPRDToDrafts(prd, sourceRef, metadata), nil, nil
	case "json", "structured":
		if len(rawPRD) == 0 {
			return nil, nil, fmt.Errorf("canonical prd payload is required")
		}
		prd, report := model.ValidatePRDJSON(rawPRD)
		if !report.Valid {
			return nil, &report, nil
		}
		return ingress.CanonicalPRDToDrafts(prd, sourceRef, metadata), nil, nil
	default:
		return nil, nil, fmt.Errorf("format must be markdown or json")
	}
}

func writePRDValidationFailure(w http.ResponseWriter, report model.PRDValidationReport) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error":  "prd failed readiness validation",
		"report": report,
	})
}

func validateWorkspacePRDMetadata(metadata map[string]string) (model.PRDValidationReport, bool) {
	if len(metadata) == 0 {
		return model.PRDValidationReport{}, false
	}
	raw := strings.TrimSpace(metadata["workspace_prd_json"])
	if raw == "" {
		return model.PRDValidationReport{}, false
	}
	_, report := model.ValidatePRDJSON([]byte(raw))
	if report.Valid {
		return model.PRDValidationReport{}, false
	}
	return report, true
}

func (s *server) createOneLoop(ctx context.Context, req loopCreateRequest) loopCreateResult {
	req.TaskContractID = strings.TrimSpace(req.TaskContractID)
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.SourceRef = strings.TrimSpace(req.SourceRef)
	req.CorrelationID = strings.TrimSpace(req.CorrelationID)
	req.Metadata = copyStringMap(req.Metadata)
	if req.CorrelationID == "" {
		req.CorrelationID = fmt.Sprintf("corr-%d", time.Now().UTC().UnixNano())
	}

	var (
		boundTask    model.TaskContract
		hasBoundTask bool
	)
	if req.TaskContractID != "" {
		task, found, err := s.store.GetTaskContract(ctx, req.TaskContractID)
		if err != nil {
			return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusInternalServerError}
		}
		if !found {
			return loopCreateResult{Status: "error", Message: "task contract not found", HTTPCode: http.StatusNotFound}
		}
		if task.Status != model.TaskContractStatusApproved {
			return loopCreateResult{Status: "error", Message: "task contract must be approved before loop creation", HTTPCode: http.StatusConflict}
		}
		hasBoundTask = true
		boundTask = task
		if req.Title == "" {
			req.Title = task.Objective
		}
		if req.Description == "" {
			req.Description = task.Objective
		}
		if req.SourceType == "" {
			req.SourceType = "task_contract"
		}
		if req.SourceRef == "" {
			req.SourceRef = strings.TrimSpace(task.SourceDocument)
			if req.SourceRef == "" {
				req.SourceRef = "task:" + task.ID
			}
		}
		req.Metadata["task_contract_id"] = task.ID
		if strings.TrimSpace(task.ProjectID) != "" {
			if _, exists := req.Metadata["project_id"]; !exists {
				req.Metadata["project_id"] = task.ProjectID
			}
		}
		if strings.TrimSpace(task.ProviderProfileID) != "" {
			req.Metadata["provider_profile_id"] = task.ProviderProfileID
		}
		if len(task.Validation) > 0 {
			if raw, err := json.Marshal(task.Validation); err == nil {
				req.Metadata["task_validation_commands_json"] = string(raw)
			}
		}
	}

	if req.Title == "" || req.SourceType == "" || req.SourceRef == "" {
		return loopCreateResult{Status: "error", Message: "title, source_type, and source_ref are required", HTTPCode: http.StatusBadRequest}
	}
	if report, ok := validateWorkspacePRDMetadata(req.Metadata); ok {
		return loopCreateResult{
			Status:           "error",
			Message:          "workspace prd failed readiness validation",
			ValidationReport: &report,
			HTTPCode:         http.StatusUnprocessableEntity,
		}
	}
	if s.projectStore != nil {
		projectID := strings.TrimSpace(req.Metadata["project_id"])
		if projectID != "" {
			project, found, err := s.projectStore.GetProject(ctx, projectID)
			if err != nil {
				return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusInternalServerError}
			}
			if !found {
				return loopCreateResult{Status: "error", Message: "project not found", HTTPCode: http.StatusBadRequest}
			}
			if strings.TrimSpace(req.Metadata["project_name"]) == "" && strings.TrimSpace(project.Name) != "" {
				req.Metadata["project_name"] = strings.TrimSpace(project.Name)
			}
			if strings.TrimSpace(req.Metadata["github_repository"]) == "" && strings.TrimSpace(project.RepoURL) != "" {
				req.Metadata["github_repository"] = strings.TrimSpace(project.RepoURL)
			}
			if strings.TrimSpace(req.Metadata["provider_profile_id"]) == "" && strings.TrimSpace(project.ProviderProfileID) != "" {
				req.Metadata["provider_profile_id"] = strings.TrimSpace(project.ProviderProfileID)
			}
			if strings.TrimSpace(req.Metadata["workspace_seed_image"]) == "" && strings.TrimSpace(project.SkillsImage) != "" {
				req.Metadata["workspace_seed_image"] = strings.TrimSpace(project.SkillsImage)
			}
			if strings.TrimSpace(req.Metadata["workspace_seed_pull_policy"]) == "" && strings.TrimSpace(project.SkillsPullPolicy) != "" {
				req.Metadata["workspace_seed_pull_policy"] = strings.TrimSpace(project.SkillsPullPolicy)
			}
		}
	}

	if providerProfileID := strings.TrimSpace(req.Metadata["provider_profile_id"]); providerProfileID != "" && s.providers != nil {
		profile, found, err := s.providers.GetProviderProfile(ctx, providerProfileID)
		if err != nil {
			return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusInternalServerError}
		}
		if !found {
			return loopCreateResult{Status: "error", Message: "provider profile not found", HTTPCode: http.StatusBadRequest}
		}
		if strings.TrimSpace(req.ProviderID) == "" {
			req.ProviderID = strings.TrimSpace(profile.ProviderType)
		}
		if strings.TrimSpace(req.Model) == "" {
			req.Model = strings.TrimSpace(profile.DefaultModel)
		}
	}

	reg := provider.NewDefaultRegistry()
	selection, err := reg.Resolve(req.ProviderID, req.Model)
	if err != nil {
		return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusBadRequest}
	}
	environment, err := model.NormalizeLoopEnvironmentWithPolicy(apiToModelEnvironment(req.Environment), s.presets.Policy())
	if err != nil {
		return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusBadRequest}
	}
	skills, skillAudit, err := model.NormalizeLoopSkillsWithPolicy(apiToModelSkills(req.Skills), selection.ProviderID, s.skillPolicy)
	if err != nil {
		return loopCreateResult{Status: "error", Message: err.Error(), HTTPCode: http.StatusBadRequest}
	}

	loopID := strings.TrimSpace(req.LoopID)
	if loopID == "" {
		projectID := req.Metadata["project_id"]
		if projectID == "" {
			projectID = req.Metadata["project"]
		}
		loopID = deriveLoopID(projectID, req.IdempotencyKey, req.SourceType, req.SourceRef)
	}
	if existing, found, err := s.store.GetState(ctx, loopID); err == nil && found {
		stored, storedFound, _ := s.store.GetAnomaly(ctx, loopID)
		if !storedFound {
			stored.Environment = environment
		}
		return loopCreateResult{
			LoopID:      loopID,
			Status:      string(existing.Record.State),
			Created:     false,
			Message:     "existing loop returned via idempotency or explicit loop_id",
			Environment: modelToApiEnvironment(stored.Environment),
			Skills:      modelToApiSkills(stored.Skills),
			HTTPCode:    http.StatusOK,
		}
	}

	anomaly := model.Anomaly{
		ID:            loopID,
		Title:         req.Title,
		Description:   req.Description,
		SourceType:    req.SourceType,
		SourceRef:     req.SourceRef,
		ProviderID:    selection.ProviderID,
		Model:         selection.Model,
		Environment:   environment,
		Skills:        skills,
		Metadata:      withIdempotency(req.Metadata, req.IdempotencyKey),
		CorrelationID: req.CorrelationID,
		Policy: model.LoopPolicy{
			MaxAttempts:      3,
			BackoffInitial:   5 * time.Second,
			BackoffMax:       2 * time.Minute,
			Timeout:          30 * time.Minute,
			TerminateOnError: false,
		},
	}
	if err := s.store.PutAnomaly(ctx, anomaly); err != nil {
		return loopCreateResult{LoopID: loopID, Status: "error", Message: err.Error(), HTTPCode: http.StatusInternalServerError}
	}
	state := model.StateRecord{
		LoopID:        loopID,
		State:         model.LoopStateUnresolved,
		Attempt:       0,
		Reason:        "created-via-api",
		CorrelationID: req.CorrelationID,
	}
	if _, err := s.store.PutState(ctx, state, 0); err != nil && !errors.Is(err, store.ErrRevisionMismatch) {
		return loopCreateResult{LoopID: loopID, Status: "error", Message: err.Error(), HTTPCode: http.StatusInternalServerError}
	}
	_ = s.store.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "ingress",
		Level:         "info",
		ActorType:     "api",
		ActorID:       "smith-api",
		Message:       "loop created from ingress",
		CorrelationID: req.CorrelationID,
		Metadata: map[string]string{
			"source_type":                       req.SourceType,
			"source_ref":                        req.SourceRef,
			"environment_mode":                  environment.ResolvedMode,
			"skill_mount_count":                 strconv.Itoa(len(skills)),
			"skill_default_read_only_count":     strconv.Itoa(skillAudit.DefaultReadOnlyCount),
			"skill_writable_mount_count":        strconv.Itoa(skillAudit.WritableCount),
			"skill_writable_override_audit_cnt": strconv.Itoa(skillAudit.WritableOverrideCount),
		},
	})
	if hasBoundTask {
		model.ApplyTaskStatusTransition(&boundTask, model.TaskContractStatusRunning, "", time.Now().UTC())
		if err := s.store.PutTaskContract(ctx, boundTask); err != nil {
			_ = s.store.AppendJournal(ctx, model.JournalEntry{
				LoopID:        loopID,
				Phase:         "ingress",
				Level:         "warn",
				ActorType:     "api",
				ActorID:       "smith-api",
				Message:       "failed to update task contract status to running",
				CorrelationID: req.CorrelationID,
				Metadata: map[string]string{
					"task_contract_id": boundTask.ID,
					"error":            err.Error(),
				},
			})
		}
	}

	return loopCreateResult{
		LoopID:      loopID,
		Status:      string(model.LoopStateUnresolved),
		Created:     true,
		Environment: modelToApiEnvironment(environment),
		Skills:      modelToApiSkills(skills),
		HTTPCode:    http.StatusCreated,
	}
}

func (s *server) handleEnvironmentPresets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"default_preset": s.presets.Default(),
			"presets":        s.presets.List(),
		})
	case http.MethodPost:
		var req presetCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if err := s.presets.Upsert(req.Name); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"name": strings.ToLower(strings.TrimSpace(req.Name))})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleEnvironmentPresetByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/environment/presets/"))
	if name == "" {
		writeErr(w, http.StatusBadRequest, "preset name is required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !s.presets.Has(name) {
			writeErr(w, http.StatusNotFound, "preset not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"name": strings.ToLower(name)})
	case http.MethodPut:
		if err := s.presets.Upsert(name); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"name": strings.ToLower(name)})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// @Summary Get or delete loop
// @Description Get detailed information about a loop or delete it
// @Tags loops
// @Produce json
// @Security BearerAuth
// @Param id path string true "Loop ID"
// @Success 200 {object} api.LoopResponse "Loop Details"
// @Router /v1/loops/{id} [get]
// @Router /v1/loops/{id} [delete]
func (s *server) handleLoopByID(w http.ResponseWriter, r *http.Request) {
	loopID, route := splitLoopRoute(r.URL.Path)
	if strings.TrimSpace(loopID) == "" {
		writeErr(w, http.StatusBadRequest, "loop id is required")
		return
	}
	if route == "" {
		switch r.Method {
		case http.MethodGet:
			state, found, err := s.store.GetState(r.Context(), loopID)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !found {
				writeErr(w, http.StatusNotFound, "loop not found")
				return
			}
			anomaly, anomalyFound, err := s.store.GetAnomaly(r.Context(), loopID)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			apiState := modelToApiState(state.Record)
			if !anomalyFound {
				enrichLoopStateForPresentation(&apiState, nil)
				writeJSON(w, http.StatusOK, api.LoopResponse{
					State: apiState,
				})
				return
			}
			enrichLoopStateForPresentation(&apiState, &anomaly)
			env := modelToApiEnvironment(anomaly.Environment)
			writeJSON(w, http.StatusOK, api.LoopResponse{
				State:       apiState,
				Anomaly:     ptr(modelToApiAnomaly(anomaly)),
				Environment: &env,
			})
			return

		case http.MethodDelete:
			if !s.authorized(r) {
				writeErr(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			state, found, err := s.store.GetState(r.Context(), loopID)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !found {
				writeErr(w, http.StatusNotFound, "loop not found")
				return
			}
			if isActiveLoopState(state.Record.State) {
				writeErr(w, http.StatusConflict, "cannot delete active loop")
				return
			}
			var req loopDeleteRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			actor := strings.TrimSpace(req.Actor)
			if actor == "" {
				actor = "operator"
			}
			if err := s.store.DeleteLoop(r.Context(), loopID); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			_ = s.store.AppendAudit(r.Context(), store.AuditRecord{
				Actor:         actor,
				Action:        "delete-loop",
				TargetLoopID:  loopID,
				CorrelationID: state.Record.CorrelationID,
				Metadata: map[string]string{
					"final_state": string(state.Record.State),
				},
			})
			writeJSON(w, http.StatusOK, map[string]any{
				"loop_id": loopID,
				"status":  "deleted",
				"actor":   actor,
			})
			return
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
	}

	if route == "journal" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 500))
		journal, err := s.store.ListJournal(r.Context(), loopID, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, journal)
		return
	}
	if route == "pause" {
		s.handleLoopLifecycleTransition(w, r, loopID, model.LoopStateUnresolved, "pause-loop", "paused-via-api")
		return
	}
	if route == "resume" {
		s.handleLoopLifecycleTransition(w, r, loopID, model.LoopStateRunning, "resume-loop", "resumed-via-api")
		return
	}
	if route == "cancel" {
		s.handleLoopLifecycleTransition(w, r, loopID, model.LoopStateCancelled, "cancel-loop", "cancelled-via-api")
		return
	}
	if route == "interventions" {
		s.handleLoopIntervention(w, r, loopID)
		return
	}
	if route == "control/attach" {
		s.handleLoopAttach(w, r, loopID)
		return
	}
	if route == "control/detach" {
		s.handleLoopDetach(w, r, loopID)
		return
	}
	if route == "control/command" {
		s.handleLoopControlCommand(w, r, loopID)
		return
	}
	if route == "runtime" {
		s.handleLoopRuntime(w, r, loopID)
		return
	}
	if route == "handoffs" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 100))
		handoffs, err := s.store.ListHandoffs(r.Context(), loopID, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, handoffs)
		return
	}
	if route == "overrides" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 100))
		overrides, err := s.store.ListOverrides(r.Context(), loopID, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, overrides)
		return
	}
	if route == "trace" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleLoopTrace(w, r, loopID)
		return
	}
	if route == "journal/stream" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleJournalStream(w, r, loopID)
		return
	}

	writeErr(w, http.StatusNotFound, "endpoint not found")
}

func splitLoopRoute(path string) (loopID string, route string) {
	remainder := strings.TrimPrefix(path, "/v1/loops/")
	if remainder == path {
		remainder = strings.TrimPrefix(path, "/api/loops/")
	}
	remainder = strings.TrimPrefix(remainder, "/")
	if remainder == "" {
		return "", ""
	}
	for _, suffix := range []string{
		"/pause",
		"/resume",
		"/cancel",
		"/interventions",
		"/journal/stream",
		"/control/attach",
		"/control/detach",
		"/control/command",
		"/runtime",
		"/handoffs",
		"/overrides",
		"/journal",
		"/trace",
	} {
		if strings.HasSuffix(remainder, suffix) {
			loopID = strings.TrimSuffix(remainder, suffix)
			loopID = strings.TrimSuffix(loopID, "/")
			return loopID, strings.TrimPrefix(suffix, "/")
		}
	}
	return remainder, ""
}

func (s *server) handleLoopRuntime(w http.ResponseWriter, r *http.Request, loopID string) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	state, found, err := s.getState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}
	writeJSON(w, http.StatusOK, s.resolveLoopRuntime(r.Context(), loopID, state.Record))
}

func (s *server) resolveLoopRuntime(ctx context.Context, loopID string, state model.StateRecord) loopRuntimeResponse {
	out := loopRuntimeResponse{
		LoopID:        loopID,
		Namespace:     runtimeNamespaceForConfig(s.cfg),
		ContainerName: runtimeContainerForConfig(s.cfg),
		Attachable:    false,
	}
	if !isActiveLoopState(state.State) {
		out.Reason = "loop not active"
		return out
	}
	if strings.TrimSpace(state.WorkerJobName) == "" || s.runtimePods == nil {
		out.Reason = defaultRuntimeReason
		return out
	}
	pod, found, err := s.findRuntimePod(ctx, out.Namespace, state.WorkerJobName)
	if err != nil || !found {
		out.Reason = defaultRuntimeReason
		return out
	}
	out.PodName = pod.Name
	out.PodPhase = string(pod.Status.Phase)

	containerName, ok := resolveRuntimeContainerName(pod, out.ContainerName)
	out.ContainerName = containerName
	if !ok {
		out.Reason = "runtime container not found"
		return out
	}
	if pod.Status.Phase != corev1.PodRunning {
		out.Reason = "runtime pod not running"
		return out
	}
	out.Attachable = true
	out.Reason = ""
	return out
}

func (s *server) findRuntimePod(ctx context.Context, namespace, workerJobName string) (corev1.Pod, bool, error) {
	if s.runtimePods == nil {
		return corev1.Pod{}, false, nil
	}
	list, err := s.runtimePods.List(ctx, namespace, metav1.ListOptions{
		LabelSelector: "job-name=" + strings.TrimSpace(workerJobName),
	})
	if err != nil {
		return corev1.Pod{}, false, err
	}
	if list == nil || len(list.Items) == 0 {
		return corev1.Pod{}, false, nil
	}
	best := list.Items[0]
	for i := 1; i < len(list.Items); i++ {
		if betterRuntimePod(list.Items[i], best) {
			best = list.Items[i]
		}
	}
	return best, true, nil
}

func betterRuntimePod(a, b corev1.Pod) bool {
	aScore := runtimePodScore(a.Status.Phase)
	bScore := runtimePodScore(b.Status.Phase)
	if aScore != bScore {
		return aScore > bScore
	}
	return a.CreationTimestamp.After(b.CreationTimestamp.Time)
}

func runtimePodScore(phase corev1.PodPhase) int {
	switch phase {
	case corev1.PodRunning:
		return 3
	case corev1.PodPending:
		return 2
	case corev1.PodUnknown:
		return 1
	default:
		return 0
	}
}

func resolveRuntimeContainerName(pod corev1.Pod, preferred string) (string, bool) {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		for _, container := range pod.Spec.Containers {
			if container.Name == preferred {
				return preferred, true
			}
		}
	}
	if len(pod.Spec.Containers) == 0 {
		return "", false
	}
	return pod.Spec.Containers[0].Name, true
}

func (s *server) handleLoopAttach(w http.ResponseWriter, r *http.Request, loopID string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:        "unauthenticated",
			Action:       "attach-terminal-rejected",
			TargetLoopID: loopID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor": "unauthenticated",
			}, "unauthorized", terminalErrUnauthorized),
		})
		writeErrCode(w, http.StatusUnauthorized, terminalErrUnauthorized, "unauthorized")
		return
	}
	var req terminalAttachRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	terminal := strings.TrimSpace(req.Terminal)
	if terminal == "" {
		terminal = "unknown"
	}
	state, found, err := s.getState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}
	if !isActiveLoopState(state.Record.State) {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "attach-terminal-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor": actor,
			}, "loop is not active", "terminal_loop_not_active"),
		})
		writeErr(w, http.StatusConflict, "loop is not active")
		return
	}
	runtime := s.resolveLoopRuntime(r.Context(), loopID, state.Record)
	if !runtime.Attachable {
		reason := strings.TrimSpace(runtime.Reason)
		if reason == "" {
			reason = "runtime target not attachable"
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "attach-terminal-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor":    actor,
				"terminal": terminal,
			}, reason, "terminal_runtime_not_attachable"),
		})
		writeErr(w, http.StatusConflict, reason)
		return
	}
	session, activeAttachCount := s.term.Attach(loopID, actor, terminal, runtime)
	metadata := terminalAcceptedMetadata(terminalSessionMetadata(actor, session, activeAttachCount))

	_ = s.appendAudit(r.Context(), store.AuditRecord{
		Actor:         actor,
		Action:        "attach-terminal",
		TargetLoopID:  loopID,
		CorrelationID: state.Record.CorrelationID,
		Metadata:      metadata,
	})
	_ = s.appendJournal(r.Context(), model.JournalEntry{
		LoopID:        loopID,
		Phase:         "operator",
		Level:         "info",
		ActorType:     "operator",
		ActorID:       actor,
		Message:       "terminal attached",
		CorrelationID: state.Record.CorrelationID,
		Metadata:      metadata,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"loop_id":             loopID,
		"status":              "attached",
		"actor":               actor,
		"attach_count":        session.AttachCount,
		"active_attach_count": activeAttachCount,
		"runtime_target_ref":  session.RuntimeTargetRef,
	})
}

func (s *server) handleLoopDetach(w http.ResponseWriter, r *http.Request, loopID string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:        "unauthenticated",
			Action:       "detach-terminal-rejected",
			TargetLoopID: loopID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor": "unauthenticated",
			}, "unauthorized", terminalErrUnauthorized),
		})
		writeErrCode(w, http.StatusUnauthorized, terminalErrUnauthorized, "unauthorized")
		return
	}
	var req terminalDetachRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	state, found, err := s.getState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}

	session, detached, activeAttachCount := s.term.Detach(loopID, actor)
	if !detached {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "detach-terminal-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor": actor,
			}, "actor is not attached", terminalErrNotAttached),
		})
		writeErr(w, http.StatusConflict, "actor is not attached")
		return
	}
	metadata := terminalAcceptedMetadata(terminalSessionMetadata(actor, session, activeAttachCount))

	_ = s.appendAudit(r.Context(), store.AuditRecord{
		Actor:         actor,
		Action:        "detach-terminal",
		TargetLoopID:  loopID,
		CorrelationID: state.Record.CorrelationID,
		Metadata:      metadata,
	})
	_ = s.appendJournal(r.Context(), model.JournalEntry{
		LoopID:        loopID,
		Phase:         "operator",
		Level:         "info",
		ActorType:     "operator",
		ActorID:       actor,
		Message:       "terminal detached",
		CorrelationID: state.Record.CorrelationID,
		Metadata:      metadata,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"loop_id":             loopID,
		"status":              "detached",
		"actor":               actor,
		"attach_count":        session.AttachCount,
		"active_attach_count": activeAttachCount,
		"runtime_target_ref":  session.RuntimeTargetRef,
	})
}

func (s *server) handleLoopControlCommand(w http.ResponseWriter, r *http.Request, loopID string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:        "unauthenticated",
			Action:       "terminal-command-rejected",
			TargetLoopID: loopID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor": "unauthenticated",
			}, "unauthorized", terminalErrUnauthorized),
		})
		writeErrCode(w, http.StatusUnauthorized, terminalErrUnauthorized, "unauthorized")
		return
	}
	state, found, err := s.getState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}
	if !isActiveLoopState(state.Record.State) {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         "operator",
			Action:        "terminal-command-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor": "operator",
			}, "loop is not active", "terminal_loop_not_active"),
		})
		writeErr(w, http.StatusConflict, "loop is not active")
		return
	}
	var req terminalCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         "operator",
			Action:        "terminal-command-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor": "operator",
			}, "invalid json", terminalErrInvalidJSON),
		})
		writeErrCode(w, http.StatusBadRequest, terminalErrInvalidJSON, "invalid json")
		return
	}
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "terminal-command-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor": actor,
			}, "command is required", terminalErrRequiredCmd),
		})
		writeErrCode(w, http.StatusBadRequest, terminalErrRequiredCmd, "command is required")
		return
	}
	if s.term == nil {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "terminal-command-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor":   actor,
				"command": command,
			}, "actor must attach before issuing commands", terminalErrNotAttached),
		})
		writeErr(w, http.StatusConflict, "actor must attach before issuing commands")
		return
	}
	session, attached := s.term.Session(loopID, actor)
	if !attached {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "terminal-command-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata: terminalRejectedMetadata(map[string]string{
				"actor":   actor,
				"command": command,
			}, "actor must attach before issuing commands", terminalErrNotAttached),
		})
		writeErr(w, http.StatusConflict, "actor must attach before issuing commands")
		return
	}
	baseMetadata := map[string]string{
		"actor":                             actor,
		"command":                           command,
		"runtime_target_ref":                session.RuntimeTargetRef,
		"runtime_namespace":                 session.RuntimeNamespace,
		"runtime_pod":                       session.RuntimePodName,
		"runtime_container":                 session.RuntimeContainerName,
		"max_command_length":                strconv.Itoa(terminalCommandMaxSize),
		"command_rate_limit_max":            strconv.Itoa(terminalCommandRateMax),
		"command_rate_limit_window_seconds": strconv.Itoa(int(terminalCommandRateWindow.Seconds())),
	}

	if len(command) > terminalCommandMaxSize {
		rejectedMetadata := terminalRejectedMetadata(baseMetadata, "command too long", terminalErrTooLong)
		rejectedMetadata["result"] = "rejected"
		rejectedMetadata["command_length"] = strconv.Itoa(len(command))
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "terminal-command-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata:      rejectedMetadata,
		})
		writeErrCode(w, http.StatusBadRequest, terminalErrTooLong, fmt.Sprintf("command exceeds max length of %d characters", terminalCommandMaxSize))
		return
	}
	session, slotFound, allowed, retryAfter := s.term.ConsumeCommandSlot(loopID, actor, time.Now().UTC())
	if !slotFound {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "terminal-command-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata:      terminalRejectedMetadata(baseMetadata, "actor must attach before issuing commands", terminalErrNotAttached),
		})
		writeErr(w, http.StatusConflict, "actor must attach before issuing commands")
		return
	}
	if !allowed {
		rejectedMetadata := terminalRejectedMetadata(baseMetadata, "command rate limit exceeded", terminalErrRateLimited)
		rejectedMetadata["result"] = "rejected"
		retryAfterSeconds := int((retryAfter + time.Second - 1) / time.Second)
		if retryAfterSeconds < 1 {
			retryAfterSeconds = 1
		}
		rejectedMetadata["retry_after_seconds"] = strconv.Itoa(retryAfterSeconds)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "terminal-command-rejected",
			TargetLoopID:  loopID,
			CorrelationID: state.Record.CorrelationID,
			Metadata:      rejectedMetadata,
		})
		writeErrCode(w, http.StatusTooManyRequests, terminalErrRateLimited, "command rate limit exceeded")
		return
	}
	if s.podExec == nil {
		writeErr(w, http.StatusServiceUnavailable, "runtime command execution unavailable")
		return
	}

	baseMetadata = terminalAcceptedMetadata(baseMetadata)
	_ = s.appendJournal(r.Context(), model.JournalEntry{
		LoopID:        loopID,
		Phase:         "operator",
		Level:         "info",
		ActorType:     "operator",
		ActorID:       actor,
		Message:       "terminal command started",
		CorrelationID: state.Record.CorrelationID,
		Metadata:      baseMetadata,
	})

	execResult, execErr := s.podExec.Execute(r.Context(), podExecRequest{
		Namespace:     session.RuntimeNamespace,
		PodName:       session.RuntimePodName,
		ContainerName: session.RuntimeContainerName,
		Command:       command,
	})
	delivered := true
	result := "success"
	if execErr != nil {
		result = "error"
		execResult.ExitCode = -1
	} else if execResult.ExitCode != 0 {
		result = "failed"
	}
	for _, line := range journalCommandOutputLines(execResult.Stdout) {
		metadata := copyStringMap(baseMetadata)
		metadata["stream"] = "stdout"
		_ = s.appendJournal(r.Context(), model.JournalEntry{
			LoopID:        loopID,
			Phase:         "operator",
			Level:         "info",
			ActorType:     "operator",
			ActorID:       actor,
			Message:       line,
			CorrelationID: state.Record.CorrelationID,
			Metadata:      metadata,
		})
	}
	for _, line := range journalCommandOutputLines(execResult.Stderr) {
		metadata := copyStringMap(baseMetadata)
		metadata["stream"] = "stderr"
		_ = s.appendJournal(r.Context(), model.JournalEntry{
			LoopID:        loopID,
			Phase:         "operator",
			Level:         "warn",
			ActorType:     "operator",
			ActorID:       actor,
			Message:       line,
			CorrelationID: state.Record.CorrelationID,
			Metadata:      metadata,
		})
	}

	resultMetadata := copyStringMap(baseMetadata)
	resultMetadata["delivered"] = strconv.FormatBool(delivered)
	resultMetadata["result"] = result
	resultMetadata["exit_code"] = strconv.Itoa(execResult.ExitCode)
	resultMetadata["stdout_bytes"] = strconv.Itoa(len(execResult.Stdout))
	resultMetadata["stderr_bytes"] = strconv.Itoa(len(execResult.Stderr))
	if execErr != nil {
		resultMetadata["exec_error"] = execErr.Error()
	}
	_ = s.appendAudit(r.Context(), store.AuditRecord{
		Actor:         actor,
		Action:        "terminal-command",
		TargetLoopID:  loopID,
		CorrelationID: state.Record.CorrelationID,
		Metadata:      resultMetadata,
	})
	_ = s.appendJournal(r.Context(), model.JournalEntry{
		LoopID:        loopID,
		Phase:         "operator",
		Level:         "info",
		ActorType:     "operator",
		ActorID:       actor,
		Message:       "terminal command completed",
		CorrelationID: state.Record.CorrelationID,
		Metadata:      resultMetadata,
	})
	response := map[string]any{
		"loop_id":            loopID,
		"status":             "completed",
		"actor":              actor,
		"command":            command,
		"delivered":          delivered,
		"result":             result,
		"exit_code":          execResult.ExitCode,
		"stdout":             execResult.Stdout,
		"stderr":             execResult.Stderr,
		"runtime_target_ref": session.RuntimeTargetRef,
	}
	if execErr != nil {
		response["error"] = execErr.Error()
	}
	writeJSON(w, http.StatusOK, response)
}

func journalCommandOutputLines(raw string) []string {
	if raw == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	parts := strings.Split(raw, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimRight(part, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func isActiveLoopState(state model.LoopState) bool {
	switch state {
	case model.LoopStateUnresolved, model.LoopStateRunning:
		return true
	default:
		return false
	}
}

func (s *server) handleLoopLifecycleTransition(w http.ResponseWriter, r *http.Request, loopID string, target model.LoopState, action, defaultReason string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req loopLifecycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = defaultReason
	}

	state, found, err := s.store.GetState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}

	current := state.Record.State
	if current == target {
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        action,
			TargetLoopID:  loopID,
			Reason:        reason,
			CorrelationID: state.Record.CorrelationID,
			Metadata: map[string]string{
				"current_state": string(current),
				"target_state":  string(target),
				"idempotent":    "true",
			},
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"loop_id":    loopID,
			"state":      string(current),
			"revision":   state.Revision,
			"idempotent": true,
			"changed":    false,
		})
		return
	}

	if !model.IsValidTransition(current, target) {
		writeErr(w, http.StatusConflict, fmt.Sprintf("invalid transition %s -> %s", current, target))
		return
	}

	next := state.Record
	next.State = target
	next.Reason = reason
	next.LockHolder = "operator-" + action
	rev, err := s.store.PutState(r.Context(), next, state.Revision)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrRevisionMismatch) {
			status = http.StatusConflict
		}
		writeErr(w, status, err.Error())
		return
	}

	_ = s.store.AppendOverride(r.Context(), model.OperatorOverride{
		LoopID:        loopID,
		Actor:         actor,
		Action:        action,
		TargetState:   target,
		Reason:        reason,
		CorrelationID: next.CorrelationID,
	})
	_ = s.appendAudit(r.Context(), store.AuditRecord{
		Actor:         actor,
		Action:        action,
		TargetLoopID:  loopID,
		Reason:        reason,
		CorrelationID: next.CorrelationID,
		Metadata: map[string]string{
			"current_state": string(current),
			"target_state":  string(target),
			"idempotent":    "false",
			"revision":      strconv.FormatInt(rev, 10),
		},
	})
	_ = s.appendJournal(r.Context(), model.JournalEntry{
		LoopID:        loopID,
		Phase:         "operator",
		Level:         "warn",
		ActorType:     "operator",
		ActorID:       actor,
		Message:       action,
		CorrelationID: next.CorrelationID,
		Metadata: map[string]string{
			"current_state": string(current),
			"target_state":  string(target),
			"reason":        reason,
		},
	})
	s.syncTaskContractStatusForLoop(r.Context(), loopID, target, reason, actor, next.CorrelationID)

	writeJSON(w, http.StatusOK, map[string]any{
		"loop_id":    loopID,
		"state":      string(target),
		"revision":   rev,
		"idempotent": false,
		"changed":    true,
	})
}

func (s *server) handleLoopIntervention(w http.ResponseWriter, r *http.Request, loopID string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	state, found, err := s.store.GetState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}

	var req loopInterventionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		writeErr(w, http.StatusBadRequest, "instruction is required")
		return
	}
	interventionType := strings.TrimSpace(req.Type)
	if interventionType == "" {
		interventionType = "operator_intervention"
	}
	eventID := strings.TrimSpace(req.EventID)
	if eventID == "" {
		eventID = fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano())
	}

	if existing, exists, err := s.findInterventionByEventID(r.Context(), loopID, eventID); err == nil && exists {
		writeJSON(w, http.StatusOK, loopInterventionResponse{
			LoopID:      loopID,
			EventID:     eventID,
			Sequence:    existing.Sequence,
			Idempotent:  true,
			Instruction: instruction,
		})
		return
	}

	entry := model.JournalEntry{
		LoopID:        loopID,
		Phase:         "operator",
		Level:         "info",
		ActorType:     "operator",
		ActorID:       actor,
		Message:       "operator intervention recorded",
		CorrelationID: state.Record.CorrelationID,
		Metadata: map[string]string{
			"intervention_event_id": eventID,
			"intervention_type":     interventionType,
			"instruction":           instruction,
		},
	}
	if err := s.store.AppendJournal(r.Context(), entry); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	stored, foundStored, err := s.findInterventionByEventID(r.Context(), loopID, eventID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !foundStored {
		writeErr(w, http.StatusInternalServerError, "failed to resolve intervention sequence")
		return
	}

	_ = s.appendAudit(r.Context(), store.AuditRecord{
		Actor:         actor,
		Action:        "operator-intervention",
		TargetLoopID:  loopID,
		Reason:        instruction,
		CorrelationID: state.Record.CorrelationID,
		Metadata: map[string]string{
			"intervention_event_id": eventID,
			"intervention_type":     interventionType,
			"sequence":              strconv.FormatInt(stored.Sequence, 10),
		},
	})

	writeJSON(w, http.StatusCreated, loopInterventionResponse{
		LoopID:      loopID,
		EventID:     eventID,
		Sequence:    stored.Sequence,
		Idempotent:  false,
		Instruction: instruction,
	})
}

func (s *server) findInterventionByEventID(ctx context.Context, loopID, eventID string) (model.JournalEntry, bool, error) {
	entries, err := s.store.ListJournal(ctx, loopID, 0)
	if err != nil {
		return model.JournalEntry{}, false, err
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return model.JournalEntry{}, false, nil
	}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if strings.TrimSpace(entry.Metadata["intervention_event_id"]) == eventID {
			return entry, true, nil
		}
	}
	return model.JournalEntry{}, false, nil
}

func (s *server) handleLoopTrace(w http.ResponseWriter, r *http.Request, loopID string) {
	limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 500))
	state, found, err := s.store.GetState(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}

	out := loopTraceResponse{
		LoopID:    loopID,
		State:     modelToApiState(state.Record),
		Journal:   []api.JournalEntry{},
		Handoffs:  []api.Handoff{},
		Overrides: []api.OperatorOverride{},
		Audit:     []api.AuditRecord{},
	}

	anomaly, anomalyFound, err := s.store.GetAnomaly(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if anomalyFound {
		apiAnomaly := modelToApiAnomaly(anomaly)
		out.Anomaly = &apiAnomaly
		out.Environment = modelToApiEnvironment(anomaly.Environment)
	}

	journal, err := s.store.ListJournal(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Journal = modelToApiJournalEntries(journal)

	handoffs, err := s.store.ListHandoffs(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Handoffs = modelToApiHandoffs(handoffs)

	overrides, err := s.store.ListOverrides(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Overrides = modelToApiOverrides(overrides)

	audit, err := s.store.ListAudit(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Audit = storeToApiAudits(audit)
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleJournalStream(w http.ResponseWriter, r *http.Request, loopID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	sinceSeq := parseInt64Default(r.URL.Query().Get("since_seq"), 0)
	if sinceSeq < 0 {
		sinceSeq = 0
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event string, payload any) error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	_ = send("ready", map[string]any{"loop_id": loopID, "since_seq": sinceSeq})
	initial, rev, err := s.store.ListJournalSinceWithRevision(r.Context(), loopID, sinceSeq)
	if err != nil {
		_ = send("error", map[string]string{"error": err.Error()})
		return
	}
	for _, entry := range initial {
		if err := send("entry", map[string]any{
			"entry":      entry,
			"emitted_at": time.Now().UTC().Format(time.RFC3339Nano),
		}); err != nil {
			return
		}
		sinceSeq = entry.Sequence
	}

	watchCh := s.store.WatchJournalWithRev(r.Context(), loopID, rev+1)

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			if _, err := fmt.Fprintf(w, ": keepalive %d\n\n", sinceSeq); err != nil {
				return
			}
			flusher.Flush()
		case entry, ok := <-watchCh:
			if !ok {
				return
			}
			if entry.Sequence <= sinceSeq {
				continue
			}
			if err := send("entry", map[string]any{
				"entry":      entry,
				"emitted_at": time.Now().UTC().Format(time.RFC3339Nano),
			}); err != nil {
				return
			}
			sinceSeq = entry.Sequence
		}
	}
}

func (s *server) handleOverride(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req overrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.LoopID = strings.TrimSpace(req.LoopID)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Actor = strings.TrimSpace(req.Actor)
	if req.Actor == "" {
		req.Actor = "operator"
	}
	if req.LoopID == "" || req.Reason == "" {
		writeErr(w, http.StatusBadRequest, "loop_id and reason are required")
		return
	}

	state, found, err := s.store.GetState(r.Context(), req.LoopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "loop not found")
		return
	}
	if !model.IsValidTransition(state.Record.State, model.LoopState(req.TargetState)) {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid transition %s -> %s", state.Record.State, req.TargetState))
		return
	}

	next := state.Record
	next.State = model.LoopState(req.TargetState)
	next.Reason = req.Reason
	next.LockHolder = "operator-override"
	rev, err := s.store.PutState(r.Context(), next, state.Revision)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrRevisionMismatch) {
			status = http.StatusConflict
		}
		writeErr(w, status, err.Error())
		return
	}

	override := model.OperatorOverride{
		LoopID:        req.LoopID,
		Actor:         req.Actor,
		Action:        "override-state",
		TargetState:   model.LoopState(req.TargetState),
		Reason:        req.Reason,
		CorrelationID: next.CorrelationID,
	}
	_ = s.store.AppendOverride(r.Context(), override)

	writeJSON(w, http.StatusOK, overrideResponse{
		LoopID:   req.LoopID,
		Status:   "overridden",
		State:    modelToApiState(next),
		Revision: rev,
	})

	_ = s.store.AppendAudit(r.Context(), store.AuditRecord{
		Actor:         req.Actor,
		Action:        "override-state",
		TargetLoopID:  req.LoopID,
		Reason:        req.Reason,
		CorrelationID: next.CorrelationID,
		Metadata: map[string]string{
			"target_state": string(req.TargetState),
			"revision":     strconv.FormatInt(rev, 10),
		},
	})
	_ = s.store.AppendJournal(r.Context(), model.JournalEntry{
		LoopID:        req.LoopID,
		Phase:         "operator",
		Level:         "warn",
		ActorType:     "operator",
		ActorID:       req.Actor,
		Message:       "manual override applied",
		CorrelationID: next.CorrelationID,
		Metadata: map[string]string{
			"target_state": string(req.TargetState),
			"reason":       req.Reason,
		},
	})
	s.syncTaskContractStatusForLoop(r.Context(), req.LoopID, model.LoopState(req.TargetState), req.Reason, req.Actor, next.CorrelationID)
}

func (s *server) handleCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	loopID := strings.TrimSpace(r.URL.Query().Get("loop_id"))
	if loopID == "" {
		writeErr(w, http.StatusBadRequest, "loop_id is required")
		return
	}

	anomaly, found, err := s.store.GetAnomaly(r.Context(), loopID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	entries, err := s.store.ListJournal(r.Context(), loopID, 0)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := costSummary{LoopID: loopID, EntryCount: len(entries)}
	if found {
		out.ProviderID = anomaly.ProviderID
		out.Model = anomaly.Model
	}
	for _, entry := range entries {

		if entry.Timestamp.After(parseRFC3339(out.LastActivityAt)) {
			out.LastActivityAt = entry.Timestamp.UTC().Format(time.RFC3339)
		}
		out.TotalTokens += parseInt64Default(entry.Metadata["token_total"], 0)
		out.PromptTokens += parseInt64Default(entry.Metadata["token_prompt"], 0)
		out.OutputTokens += parseInt64Default(entry.Metadata["token_output"], 0)
		out.TotalCostUSD += parseFloatDefault(entry.Metadata["cost_usd"], 0)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	loopID := strings.TrimSpace(r.URL.Query().Get("loop_id"))
	limit := int64(parseIntDefault(r.URL.Query().Get("limit"), 500))
	records, err := s.store.ListAudit(r.Context(), loopID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *server) handleProjectGitHubCredential(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.projectCred == nil {
		writeErr(w, http.StatusInternalServerError, "project credential store unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
		if projectID == "" {
			writeErr(w, http.StatusBadRequest, "project_id is required")
			return
		}
		cred, found, err := s.projectCred.GetProjectCredential(r.Context(), projectID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := map[string]any{
			"project_id":     projectID,
			"credential_set": found,
		}
		if found {
			out["github_user"] = strings.TrimSpace(cred.GitHubUser)
			out["credential_masked"] = maskCredentialValue(cred.PAT)
			out["updated_at"] = formatRFC3339OrEmpty(cred.UpdatedAt)
			if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("reveal")), "true") {
				out["credential"] = cred.PAT
			}
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var req projectCredentialUpsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		projectID := strings.TrimSpace(req.ProjectID)
		if projectID == "" {
			writeErr(w, http.StatusBadRequest, "project_id is required")
			return
		}
		credential := strings.TrimSpace(req.Credential)
		if credential == "" {
			writeErr(w, http.StatusBadRequest, "credential is required")
			return
		}
		cred := provider.ProjectCredential{
			GitHubUser: strings.TrimSpace(req.GitHubUser),
			PAT:        credential,
			UpdatedAt:  time.Now().UTC(),
		}
		if err := s.projectCred.PutProjectCredential(r.Context(), projectID, cred); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "update-project-credential",
			Metadata: map[string]string{
				"project_id":     projectID,
				"github_user":    cred.GitHubUser,
				"credential_set": "true",
			},
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"project_id":        projectID,
			"credential_set":    true,
			"github_user":       cred.GitHubUser,
			"credential_masked": maskCredentialValue(cred.PAT),
			"updated_at":        formatRFC3339OrEmpty(cred.UpdatedAt),
		})
	case http.MethodDelete:
		projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
		if projectID == "" {
			var req projectCredentialDeleteRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			projectID = strings.TrimSpace(req.ProjectID)
		}
		if projectID == "" {
			writeErr(w, http.StatusBadRequest, "project_id is required")
			return
		}
		if err := s.projectCred.DeleteProjectCredential(r.Context(), projectID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "delete-project-credential",
			Metadata: map[string]string{
				"project_id":     projectID,
				"credential_set": "false",
			},
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"project_id":     projectID,
			"credential_set": false,
		})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleProjectGitHubCredentialTest(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.projectCred == nil {
		writeErr(w, http.StatusInternalServerError, "project credential store unavailable")
		return
	}
	if s.projectStore == nil {
		writeErr(w, http.StatusInternalServerError, "project store unavailable")
		return
	}

	var req projectCredentialTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" {
		writeErr(w, http.StatusBadRequest, "project_id is required")
		return
	}
	project, found, err := s.projectStore.GetProject(r.Context(), projectID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "project not found")
		return
	}
	cred, found, err := s.projectCred.GetProjectCredential(r.Context(), projectID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found || strings.TrimSpace(cred.PAT) == "" {
		message := "credential missing"
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "test-project-credential",
			Metadata: map[string]string{
				"project_id": projectID,
				"valid":      "false",
				"message":    message,
			},
		})
		writeJSON(w, http.StatusOK, projectCredentialTestResponse{
			ProjectID: projectID,
			RepoURL:   strings.TrimSpace(project.RepoURL),
			Valid:     false,
			Message:   message,
		})
		return
	}

	validator := s.repoAccess
	if validator == nil {
		validator = validateGitHubRepositoryAccess
	}
	valid, message, err := validator(r.Context(), strings.TrimSpace(project.RepoURL), strings.TrimSpace(cred.GitHubUser), strings.TrimSpace(cred.PAT))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if strings.TrimSpace(message) == "" {
		if valid {
			message = "repository access verified"
		} else {
			message = "repository access check failed"
		}
	}

	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	_ = s.appendAudit(r.Context(), store.AuditRecord{
		Actor:  actor,
		Action: "test-project-credential",
		Metadata: map[string]string{
			"project_id": projectID,
			"valid":      strconv.FormatBool(valid),
			"message":    message,
		},
	})

	writeJSON(w, http.StatusOK, projectCredentialTestResponse{
		ProjectID: projectID,
		RepoURL:   strings.TrimSpace(project.RepoURL),
		Valid:     valid,
		Message:   message,
	})
}

func validateGitHubRepositoryAccess(ctx context.Context, repoURL string, githubUser string, credential string) (bool, string, error) {
	repoURL = strings.TrimSpace(repoURL)
	githubUser = strings.TrimSpace(githubUser)
	credential = strings.TrimSpace(credential)
	if repoURL == "" {
		return false, "repo_url is required", nil
	}
	if githubUser == "" {
		return false, "github_user is required", nil
	}
	if credential == "" {
		return false, "credential is required", nil
	}

	repoPath, err := parseGitHubRepositoryPath(repoURL)
	if err != nil {
		return false, err.Error(), nil
	}

	requestCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "https://api.github.com/repos/"+repoPath, nil)
	if err != nil {
		return false, "failed to build repository check request", nil
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "smith-api")

	response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
	if err != nil {
		return false, fmt.Sprintf("failed to reach GitHub API: %v", err), nil
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		return true, "repository access verified", nil
	case http.StatusUnauthorized:
		return false, "credential rejected by GitHub (401 unauthorized)", nil
	case http.StatusForbidden:
		return false, "credential lacks required repository access (403 forbidden)", nil
	case http.StatusNotFound:
		return false, "repository not found or credential has no access (404)", nil
	default:
		return false, fmt.Sprintf("github repository check failed (status %d)", response.StatusCode), nil
	}
}

func parseGitHubRepositoryPath(repoURL string) (string, error) {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return "", errors.New("repo_url is required")
	}
	if strings.HasPrefix(repoURL, "git@github.com:") {
		repoPath := strings.TrimPrefix(repoURL, "git@github.com:")
		repoPath = strings.Trim(strings.TrimSuffix(repoPath, ".git"), "/")
		parts := strings.Split(repoPath, "/")
		if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return "", errors.New("repo_url must include owner and repository name")
		}
		return parts[0] + "/" + parts[1], nil
	}

	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", errors.New("repo_url is invalid")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host != "github.com" {
		return "", errors.New("only github.com repositories are supported for PAT validation")
	}
	repoPath := strings.Trim(strings.TrimSuffix(parsed.Path, ".git"), "/")
	parts := strings.Split(repoPath, "/")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("repo_url must include owner and repository name")
	}
	return parts[0] + "/" + parts[1], nil
}

func (s *server) handleOnboardingReadiness(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	profiles, err := s.providers.ListProviderProfiles(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	projects, err := s.projectStore.ListProjects(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	var selected *provider.Project
	if projectID != "" {
		if project, found, getErr := s.projectStore.GetProject(r.Context(), projectID); getErr != nil {
			writeErr(w, http.StatusInternalServerError, getErr.Error())
			return
		} else if found {
			normalized := normalizeProjectContract(project)
			selected = &normalized
		}
	} else if len(projects) > 0 {
		normalized := normalizeProjectContract(projects[0])
		selected = &normalized
		projectID = normalized.ID
	}

	requirements := make([]onboardingRequirement, 0, 5)
	if len(profiles) == 0 {
		requirements = append(requirements, onboardingRequirement{ID: "provider_catalog", Label: "Provider profiles available", Status: "missing", Details: "no provider profiles configured"})
	} else {
		requirements = append(requirements, onboardingRequirement{ID: "provider_catalog", Label: "Provider profiles available", Status: "complete"})
	}

	if selected == nil {
		requirements = append(requirements,
			onboardingRequirement{ID: "project", Label: "Project configured", Status: "missing", Details: "create a project with repository settings"},
			onboardingRequirement{ID: "repository", Label: "Repository configured", Status: "missing"},
			onboardingRequirement{ID: "provider_binding", Label: "Project provider binding", Status: "missing"},
			onboardingRequirement{ID: "github_credential", Label: "Git credential configured", Status: "missing"},
		)
		writeJSON(w, http.StatusOK, onboardingReadinessResponse{
			Ready:      false,
			ProjectID:  projectID,
			Missing:    missingRequirementIDs(requirements),
			NextStep:   nextMissingRequirement(requirements),
			Requires:   requirements,
			Credential: onboardingCredentialStatus{ProjectID: projectID, CredentialSet: false, Valid: false, Message: "project is not configured"},
		})
		return
	}

	if strings.TrimSpace(selected.ID) == "" {
		requirements = append(requirements, onboardingRequirement{ID: "project", Label: "Project configured", Status: "missing"})
	} else {
		requirements = append(requirements, onboardingRequirement{ID: "project", Label: "Project configured", Status: "complete"})
	}
	if strings.TrimSpace(selected.RepoURL) == "" {
		requirements = append(requirements, onboardingRequirement{ID: "repository", Label: "Repository configured", Status: "missing", Details: "repo_url is required"})
	} else {
		requirements = append(requirements, onboardingRequirement{ID: "repository", Label: "Repository configured", Status: "complete"})
	}

	providerProfileID := strings.TrimSpace(selected.ProviderProfileID)
	if providerProfileID == "" {
		providerProfileID = provider.DefaultProviderProfileID
	}
	if _, found, getErr := s.providers.GetProviderProfile(r.Context(), providerProfileID); getErr != nil {
		writeErr(w, http.StatusInternalServerError, getErr.Error())
		return
	} else if !found {
		requirements = append(requirements, onboardingRequirement{ID: "provider_binding", Label: "Project provider binding", Status: "missing", Details: "provider profile not found"})
	} else {
		requirements = append(requirements, onboardingRequirement{ID: "provider_binding", Label: "Project provider binding", Status: "complete"})
	}

	credential, err := s.resolveOnboardingCredentialStatus(r.Context(), selected.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if credential.Valid {
		requirements = append(requirements, onboardingRequirement{ID: "github_credential", Label: "Git credential configured", Status: "complete"})
	} else {
		requirements = append(requirements, onboardingRequirement{ID: "github_credential", Label: "Git credential configured", Status: "missing", Details: credential.Message})
	}

	missing := missingRequirementIDs(requirements)
	writeJSON(w, http.StatusOK, onboardingReadinessResponse{
		Ready:      len(missing) == 0,
		ProjectID:  selected.ID,
		Missing:    missing,
		NextStep:   nextMissingRequirement(requirements),
		Requires:   requirements,
		Credential: credential,
	})
}

func (s *server) handleOnboardingRepository(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req onboardingRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	projectID := strings.TrimSpace(req.ProjectID)
	repoURL := strings.TrimSpace(req.RepoURL)
	if projectID == "" || repoURL == "" {
		writeErr(w, http.StatusBadRequest, "project_id and repo_url are required")
		return
	}
	project := provider.Project{
		ID:                projectID,
		Name:              strings.TrimSpace(req.Name),
		RepoURL:           repoURL,
		ProviderProfileID: strings.TrimSpace(req.ProviderProfileID),
		GitHubUser:        strings.TrimSpace(req.GitHubUser),
	}
	if existing, found, err := s.projectStore.GetProject(r.Context(), projectID); err == nil && found {
		existing = normalizeProjectContract(existing)
		if strings.TrimSpace(project.Name) == "" {
			project.Name = existing.Name
		}
		if strings.TrimSpace(project.ProviderProfileID) == "" {
			project.ProviderProfileID = existing.ProviderProfileID
		}
		if strings.TrimSpace(project.GitHubUser) == "" {
			project.GitHubUser = existing.GitHubUser
		}
		if strings.TrimSpace(existing.RuntimeImage) != "" {
			project.RuntimeImage = existing.RuntimeImage
		}
		if strings.TrimSpace(existing.RuntimePullPolicy) != "" {
			project.RuntimePullPolicy = existing.RuntimePullPolicy
		}
		if strings.TrimSpace(existing.SkillsImage) != "" {
			project.SkillsImage = existing.SkillsImage
		}
		if strings.TrimSpace(existing.SkillsPullPolicy) != "" {
			project.SkillsPullPolicy = existing.SkillsPullPolicy
		}
	}
	project = normalizeProjectContract(project)
	if err := s.ensureProviderProfileExists(r.Context(), project.ProviderProfileID); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.projectStore.PutProject(r.Context(), project); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	_ = s.appendAudit(r.Context(), store.AuditRecord{
		Actor:  actor,
		Action: "onboarding-save-repository",
		Metadata: map[string]string{
			"project_id":          project.ID,
			"repo_url":            project.RepoURL,
			"provider_profile_id": project.ProviderProfileID,
		},
	})
	writeJSON(w, http.StatusOK, project)
}

func (s *server) handleOnboardingCredentialValidate(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req onboardingCredentialValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" {
		writeErr(w, http.StatusBadRequest, "project_id is required")
		return
	}
	status, err := s.resolveOnboardingCredentialStatus(r.Context(), projectID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "operator"
	}
	_ = s.appendAudit(r.Context(), store.AuditRecord{
		Actor:  actor,
		Action: "onboarding-validate-credential",
		Metadata: map[string]string{
			"project_id":     status.ProjectID,
			"credential_set": strconv.FormatBool(status.CredentialSet),
			"valid":          strconv.FormatBool(status.Valid),
		},
	})
	writeJSON(w, http.StatusOK, status)
}

func (s *server) resolveOnboardingCredentialStatus(ctx context.Context, projectID string) (onboardingCredentialStatus, error) {
	status := onboardingCredentialStatus{ProjectID: projectID, CredentialSet: false, Valid: false}
	if s.projectCred == nil {
		status.Message = "project credential store unavailable"
		return status, nil
	}
	cred, found, err := s.projectCred.GetProjectCredential(ctx, projectID)
	if err != nil {
		return onboardingCredentialStatus{}, err
	}
	if !found || strings.TrimSpace(cred.PAT) == "" {
		status.Message = "credential missing"
		return status, nil
	}
	status.GitHubUser = strings.TrimSpace(cred.GitHubUser)
	status.CredentialSet = true
	status.CredentialMasked = maskCredentialValue(cred.PAT)
	status.UpdatedAt = formatRFC3339OrEmpty(cred.UpdatedAt)
	if strings.TrimSpace(cred.GitHubUser) == "" {
		status.Message = "github_user is missing"
		status.Valid = false
		return status, nil
	}
	status.Valid = true
	status.Message = "credential available"
	return status, nil
}

func missingRequirementIDs(requirements []onboardingRequirement) []string {
	out := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		if strings.TrimSpace(requirement.Status) == "complete" {
			continue
		}
		out = append(out, requirement.ID)
	}
	return out
}

func nextMissingRequirement(requirements []onboardingRequirement) string {
	for _, requirement := range requirements {
		if strings.TrimSpace(requirement.Status) == "complete" {
			continue
		}
		return requirement.ID
	}
	return ""
}

func (s *server) handleProviderCatalog(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.filterProviderCatalogByFlags(provider.SupportedProviderCatalog()))
}

func (s *server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodGet:
		profiles, err := s.providers.ListProviderProfiles(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s.filterProviderProfilesForFlags(profiles))
	case http.MethodPost:
		var profile provider.ProviderProfile
		if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		normalized, err := provider.NormalizeProviderProfile(profile)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if !s.isProviderTypeEnabled(normalized.ProviderType) {
			writeErr(w, http.StatusForbidden, fmt.Sprintf("provider_type %q is disabled by feature flag", normalized.ProviderType))
			return
		}
		if err := s.ensureProviderSecretRefRequired(normalized); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.ensureSecretExists(r.Context(), normalized.SecretRef); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.providers.PutProviderProfile(r.Context(), normalized); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "create-provider-profile",
			Metadata: map[string]string{
				"provider_id":   normalized.ID,
				"provider_type": normalized.ProviderType,
				"secret_ref":    normalized.SecretRef,
			},
		})
		writeJSON(w, http.StatusOK, normalized)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleProviderByID(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, route := splitProviderRoute(r.URL.Path)
	if id == "" {
		writeErr(w, http.StatusBadRequest, "provider id is required")
		return
	}
	if route != "" {
		if route == "models" {
			s.handleProviderModels(w, r, id)
			return
		}
		writeErr(w, http.StatusNotFound, "provider route not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		profile, found, err := s.providers.GetProviderProfile(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusNotFound, "provider profile not found")
			return
		}
		if !s.isProviderTypeEnabled(profile.ProviderType) {
			writeErr(w, http.StatusNotFound, "provider profile not found")
			return
		}
		writeJSON(w, http.StatusOK, profile)
	case http.MethodPut:
		var profile provider.ProviderProfile
		if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if strings.TrimSpace(profile.ID) == "" {
			profile.ID = id
		}
		if strings.TrimSpace(profile.ID) != id {
			writeErr(w, http.StatusBadRequest, "id mismatch")
			return
		}
		normalized, err := provider.NormalizeProviderProfile(profile)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if !s.isProviderTypeEnabled(normalized.ProviderType) {
			writeErr(w, http.StatusForbidden, fmt.Sprintf("provider_type %q is disabled by feature flag", normalized.ProviderType))
			return
		}
		if err := s.ensureProviderSecretRefRequired(normalized); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.ensureSecretExists(r.Context(), normalized.SecretRef); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.providers.PutProviderProfile(r.Context(), normalized); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "update-provider-profile",
			Metadata: map[string]string{
				"provider_id":   normalized.ID,
				"provider_type": normalized.ProviderType,
				"secret_ref":    normalized.SecretRef,
			},
		})
		writeJSON(w, http.StatusOK, normalized)
	case http.MethodDelete:
		projects, err := s.projectStore.ListProjects(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, project := range projects {
			if strings.TrimSpace(project.ProviderProfileID) == id {
				writeErr(w, http.StatusConflict, "provider profile is referenced by a project")
				return
			}
		}
		if err := s.providers.DeleteProviderProfile(r.Context(), id); err != nil {
			if errors.Is(err, provider.ErrProtectedProviderProfile) {
				writeErr(w, http.StatusConflict, err.Error())
				return
			}
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "delete-provider-profile",
			Metadata: map[string]string{
				"provider_id": id,
			},
		})
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleProviderModels(w http.ResponseWriter, r *http.Request, providerID string) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	profile, found, err := s.providers.GetProviderProfile(r.Context(), providerID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "provider profile not found")
		return
	}
	if !s.isProviderTypeEnabled(profile.ProviderType) {
		writeErr(w, http.StatusNotFound, "provider profile not found")
		return
	}

	credential, err := s.resolveProviderCredentialForModelInventory(r.Context(), profile)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	includeAll := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include")), "all")
	models, err := s.modelInventory().ListModels(r.Context(), provider.ModelInventoryRequest{
		Profile:    profile,
		Credential: credential,
		IncludeAll: includeAll,
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	_ = s.appendAudit(r.Context(), store.AuditRecord{
		Actor:  "operator",
		Action: "list-provider-models",
		Metadata: map[string]string{
			"provider_id":   profile.ID,
			"provider_type": profile.ProviderType,
			"model_count":   strconv.Itoa(len(models)),
		},
	})

	source := "account_scoped_chat"
	if includeAll {
		source = "account_scoped_all"
	}

	apiModels := make([]api.ProviderModel, 0, len(models))
	for _, item := range models {
		apiModels = append(apiModels, api.ProviderModel{
			ID:      item.ID,
			OwnedBy: item.OwnedBy,
			Created: item.Created,
		})
	}

	writeJSON(w, http.StatusOK, api.ProviderModelsResponse{
		ProviderID:   profile.ID,
		ProviderType: profile.ProviderType,
		DefaultModel: profile.DefaultModel,
		Source:       source,
		FetchedAt:    time.Now().UTC().Format(time.RFC3339),
		Models:       apiModels,
	})
}

func (s *server) modelInventory() provider.ModelInventoryService {
	if s.modelCatalog != nil {
		return s.modelCatalog
	}
	return provider.NewAccountModelInventoryService()
}

func (s *server) resolveProviderCredentialForModelInventory(ctx context.Context, profile provider.ProviderProfile) (string, error) {
	if secretRef := strings.TrimSpace(profile.SecretRef); secretRef != "" {
		if s.secrets == nil {
			return "", errors.New("secret store unavailable")
		}
		secret, found, err := s.secrets.GetSecret(ctx, secretRef)
		if err != nil {
			return "", err
		}
		if !found {
			return "", fmt.Errorf("secret %q not found for provider profile %q", secretRef, profile.ID)
		}
		value := strings.TrimSpace(secret.Value)
		if value == "" {
			return "", fmt.Errorf("secret %q has no value", secretRef)
		}
		return value, nil
	}
	return "", fmt.Errorf("provider profile %q requires secret_ref for model inventory", profile.ID)
}

func (s *server) handleSecrets(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.secrets == nil {
		writeErr(w, http.StatusInternalServerError, "secret store unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		secrets, err := s.secrets.ListSecrets(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := make([]map[string]any, 0, len(secrets))
		for _, secret := range secrets {
			out = append(out, secretResponse(secret))
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var secret provider.SettingsSecret
		if err := json.NewDecoder(r.Body).Decode(&secret); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		normalized, err := provider.NormalizeSettingsSecret(secret)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		secretAction := "create-secret"
		if existing, found, err := s.secrets.GetSecret(r.Context(), normalized.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		} else if found && normalized.Value == "" {
			normalized.Value = existing.Value
			secretAction = "update-secret"
		} else if found {
			secretAction = "update-secret"
		}
		if strings.TrimSpace(normalized.Value) == "" {
			writeErr(w, http.StatusBadRequest, "secret value is required")
			return
		}
		if err := s.secrets.PutSecret(r.Context(), normalized); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: secretAction,
			Metadata: map[string]string{
				"secret_id": normalized.ID,
			},
		})
		writeJSON(w, http.StatusOK, secretResponse(normalized))
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleSecretByID(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if s.secrets == nil {
		writeErr(w, http.StatusInternalServerError, "secret store unavailable")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/secrets/")
	id = strings.TrimSpace(id)
	if id == "" {
		writeErr(w, http.StatusBadRequest, "secret id is required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		secret, found, err := s.secrets.GetSecret(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusNotFound, "secret not found")
			return
		}
		writeJSON(w, http.StatusOK, secretResponse(secret))
	case http.MethodPut:
		var secret provider.SettingsSecret
		if err := json.NewDecoder(r.Body).Decode(&secret); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if strings.TrimSpace(secret.ID) == "" {
			secret.ID = id
		}
		if strings.TrimSpace(secret.ID) != id {
			writeErr(w, http.StatusBadRequest, "id mismatch")
			return
		}
		normalized, err := provider.NormalizeSettingsSecret(secret)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		existing, found, err := s.secrets.GetSecret(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusNotFound, "secret not found")
			return
		}
		if strings.TrimSpace(normalized.Value) == "" {
			normalized.Value = existing.Value
		}
		if strings.TrimSpace(normalized.Value) == "" {
			writeErr(w, http.StatusBadRequest, "secret value is required")
			return
		}
		if err := s.secrets.PutSecret(r.Context(), normalized); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "update-secret",
			Metadata: map[string]string{
				"secret_id": normalized.ID,
			},
		})
		writeJSON(w, http.StatusOK, secretResponse(normalized))
	case http.MethodDelete:
		profiles, err := s.providers.ListProviderProfiles(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, profile := range profiles {
			if strings.TrimSpace(profile.SecretRef) == id {
				writeErr(w, http.StatusConflict, "secret is referenced by a provider profile")
				return
			}
		}
		if err := s.secrets.DeleteSecret(r.Context(), id); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "delete-secret",
			Metadata: map[string]string{
				"secret_id": id,
			},
		})
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func secretResponse(secret provider.SettingsSecret) map[string]any {
	value := strings.TrimSpace(secret.Value)
	out := map[string]any{
		"id":          secret.ID,
		"name":        secret.Name,
		"description": secret.Description,
		"updated_at":  secret.UpdatedAt,
		"has_value":   value != "",
	}
	if value != "" {
		out["value_masked"] = maskCredentialValue(value)
	}
	return out
}

func (s *server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodGet:
		projects, err := s.projectStore.ListProjects(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		for i := range projects {
			projects[i] = normalizeProjectContract(projects[i])
		}
		writeJSON(w, http.StatusOK, projects)
	case http.MethodPost:
		var p provider.Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		p.ID = strings.TrimSpace(p.ID)
		if p.ID == "" {
			writeErr(w, http.StatusBadRequest, "project id is required")
			return
		}
		p = normalizeProjectContract(p)
		if err := s.ensureProviderProfileExists(r.Context(), p.ProviderProfileID); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.projectStore.PutProject(r.Context(), p); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "create-project",
			Metadata: map[string]string{
				"project_id":          p.ID,
				"provider_profile_id": p.ProviderProfileID,
				"repo_url":            p.RepoURL,
			},
		})
		writeJSON(w, http.StatusOK, p)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleProjectByID(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := projectIDFromPath(r.URL.Path)
	if id == "" {
		writeErr(w, http.StatusBadRequest, "project id is required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, found, err := s.projectStore.GetProject(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusNotFound, "project not found")
			return
		}
		p = normalizeProjectContract(p)
		writeJSON(w, http.StatusOK, p)
	case http.MethodPut:
		var p provider.Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if p.ID == "" {
			p.ID = id
		}
		if p.ID != id {
			writeErr(w, http.StatusBadRequest, "id mismatch")
			return
		}
		p = normalizeProjectContract(p)
		if err := s.ensureProviderProfileExists(r.Context(), p.ProviderProfileID); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.projectStore.PutProject(r.Context(), p); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "update-project",
			Metadata: map[string]string{
				"project_id":          p.ID,
				"provider_profile_id": p.ProviderProfileID,
				"repo_url":            p.RepoURL,
			},
		})
		writeJSON(w, http.StatusOK, p)
	case http.MethodDelete:
		if err := s.projectStore.DeleteProject(r.Context(), id); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  "operator",
			Action: "delete-project",
			Metadata: map[string]string{
				"project_id": id,
			},
		})
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) ensureProviderProfileExists(ctx context.Context, providerProfileID string) error {
	providerProfileID = strings.TrimSpace(providerProfileID)
	if providerProfileID == "" {
		providerProfileID = provider.DefaultProviderProfileID
	}
	profile, found, err := s.providers.GetProviderProfile(ctx, providerProfileID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("provider profile %q not found", providerProfileID)
	}
	if !s.isProviderTypeEnabled(profile.ProviderType) {
		return fmt.Errorf("provider profile %q is disabled by feature flag", providerProfileID)
	}
	return nil
}

func (s *server) isProviderTypeEnabled(providerType string) bool {
	switch canonicalProviderType(providerType) {
	case provider.ProviderCodex:
		return true
	case provider.ProviderClaude:
		return s.cfg.providerClaudeEnabled
	case provider.ProviderGemini:
		return s.cfg.providerGeminiEnabled
	default:
		return false
	}
}

func (s *server) filterProviderProfilesForFlags(profiles []provider.ProviderProfile) []provider.ProviderProfile {
	if len(profiles) == 0 {
		return []provider.ProviderProfile{}
	}
	filtered := make([]provider.ProviderProfile, 0, len(profiles))
	for _, profile := range profiles {
		if !s.isProviderTypeEnabled(profile.ProviderType) {
			continue
		}
		filtered = append(filtered, profile)
	}
	return filtered
}

func (s *server) filterProviderCatalogByFlags(entries []provider.CatalogEntry) []provider.CatalogEntry {
	if len(entries) == 0 {
		return []provider.CatalogEntry{}
	}
	filtered := make([]provider.CatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if !s.isProviderTypeEnabled(entry.ID) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func canonicalProviderType(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case provider.ProviderCodex, "openai":
		return provider.ProviderCodex
	case provider.ProviderClaude, "anthropic":
		return provider.ProviderClaude
	case provider.ProviderGemini, "google":
		return provider.ProviderGemini
	default:
		return ""
	}
}

func (s *server) ensureSecretExists(ctx context.Context, secretRef string) error {
	secretRef = strings.TrimSpace(secretRef)
	if secretRef == "" {
		return nil
	}
	if s.secrets == nil {
		return errors.New("secret store unavailable")
	}
	_, found, err := s.secrets.GetSecret(ctx, secretRef)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("secret %q not found", secretRef)
	}
	return nil
}

func (s *server) ensureProviderSecretRefRequired(profile provider.ProviderProfile) error {
	providerType := canonicalProviderType(profile.ProviderType)
	if providerType == "" {
		return fmt.Errorf("unsupported provider_type %q", profile.ProviderType)
	}
	if strings.TrimSpace(profile.SecretRef) == "" {
		return fmt.Errorf("provider profile %q requires secret_ref", profile.ID)
	}
	return nil
}

func normalizeProjectContract(in provider.Project) provider.Project {
	in.ProviderProfileID = strings.TrimSpace(in.ProviderProfileID)
	if in.ProviderProfileID == "" {
		in.ProviderProfileID = provider.DefaultProviderProfileID
	}
	in.RuntimePullPolicy = strings.TrimSpace(in.RuntimePullPolicy)
	if in.RuntimePullPolicy == "" {
		in.RuntimePullPolicy = "IfNotPresent"
	}
	in.SkillsPullPolicy = strings.TrimSpace(in.SkillsPullPolicy)
	if in.SkillsPullPolicy == "" {
		in.SkillsPullPolicy = "IfNotPresent"
	}
	return in
}

func (s *server) authorized(r *http.Request) bool {
	token := strings.TrimSpace(s.cfg.operatorToken)
	if token == "" {
		return true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, prefix)) == token
}

func newProjectStore(_ context.Context, cfg config) (provider.ProjectStore, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.authStoreBackend))
	switch backend {
	case "", "file":
		return provider.NewFileProjectStore(), nil
	case "kubernetes", "k8s":
		clientset, err := kubeClient()
		if err != nil {
			return nil, fmt.Errorf("kubernetes clientset: %w", err)
		}
		return provider.NewConfigMapProjectStore(
			clientset,
			cfg.authStoreK8sNamespace,
			"smith-projects",
		)
	default:
		return nil, fmt.Errorf("unsupported project store backend %q", cfg.authStoreBackend)
	}
}

func newProviderProfileStore(_ context.Context, cfg config) (provider.ProviderProfileStore, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.authStoreBackend))
	switch backend {
	case "", "file":
		return provider.NewFileProviderProfileStore(), nil
	case "kubernetes", "k8s":
		clientset, err := kubeClient()
		if err != nil {
			return nil, fmt.Errorf("kubernetes clientset: %w", err)
		}
		return provider.NewConfigMapProviderProfileStore(
			clientset,
			cfg.authStoreK8sNamespace,
			"smith-provider-profiles",
		)
	default:
		return nil, fmt.Errorf("unsupported provider profile store backend %q", cfg.authStoreBackend)
	}
}

func newSecretStore(_ context.Context, cfg config) (provider.SecretStore, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.authStoreBackend))
	switch backend {
	case "", "file":
		return provider.NewFileSecretStore(), nil
	case "kubernetes", "k8s":
		clientset, err := kubeClient()
		if err != nil {
			return nil, fmt.Errorf("kubernetes clientset: %w", err)
		}
		return provider.NewSecretSecretStore(
			clientset,
			cfg.authStoreK8sNamespace,
			"smith-settings-secrets",
		)
	default:
		return nil, fmt.Errorf("unsupported secret store backend %q", cfg.authStoreBackend)
	}
}

func newTokenStore(_ context.Context, cfg config) (provider.TokenStore, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.authStoreBackend))
	switch backend {
	case "", "file":
		return provider.NewFileTokenStore(cfg.authStorePath), nil
	case "kubernetes", "k8s":
		restConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("kubernetes in-cluster config: %w", err)
		}
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, fmt.Errorf("kubernetes clientset: %w", err)
		}
		return provider.NewSecretTokenStore(
			clientset,
			cfg.authStoreK8sNamespace,
			cfg.authStoreK8sSecret,
			cfg.authStoreK8sKey,
		)
	default:
		return nil, fmt.Errorf("unsupported auth store backend %q", cfg.authStoreBackend)
	}
}

func newRuntimePodReader() (runtimePodReader, error) {
	client, err := kubeClient()
	if err != nil {
		return nil, err
	}
	return kubeRuntimePodReader{kube: client}, nil
}

func newPodExecRunner() (podExecRunner, error) {
	client, restConfig, err := kubeClientWithConfig()
	if err != nil {
		return nil, err
	}
	return kubePodExecRunner{
		kube:       client,
		restConfig: restConfig,
	}, nil
}

func kubeClient() (*kubernetes.Clientset, error) {
	client, _, err := kubeClientWithConfig()
	return client, err
}

func kubeClientWithConfig() (*kubernetes.Clientset, *rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		client, clientErr := kubernetes.NewForConfig(cfg)
		return client, cfg, clientErr
	}
	kubeconfig := strings.TrimSpace(os.Getenv("KUBECONFIG"))
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			kubeconfig = home + "/.kube/config"
		}
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return client, cfg, nil
}

func loadConfig() (config, error) {
	endpoints := splitCSV(os.Getenv("SMITH_ETCD_ENDPOINTS"))
	if len(endpoints) == 0 {
		endpoints = []string{"http://127.0.0.1:2379"}
	}
	skillPolicy := model.DefaultSkillPolicy()
	if raw := splitCSV(os.Getenv("SMITH_SKILL_ALLOWED_SOURCES")); len(raw) > 0 {
		skillPolicy.AllowedSourcePrefixes = raw
	}
	skillPolicy.AllowWritable = envBool("SMITH_SKILL_ALLOW_WRITABLE", skillPolicy.AllowWritable)
	authStoreBackend := strings.ToLower(strings.TrimSpace(envString("SMITH_AUTH_STORE_BACKEND", "file")))
	authStoreK8sNamespace := strings.TrimSpace(envString("SMITH_AUTH_STORE_K8S_NAMESPACE", envString("POD_NAMESPACE", "default")))
	authStoreK8sSecret := strings.TrimSpace(envString("SMITH_AUTH_STORE_K8S_SECRET", "smith-auth-store"))
	authStoreK8sKey := strings.TrimSpace(envString("SMITH_AUTH_STORE_K8S_KEY", "tokens.json"))
	documentStoreBackend := docstore.NormalizeBackend(envString("SMITH_DOCUMENT_STORE_BACKEND", "etcd"))
	if !docstore.IsSupportedBackend(documentStoreBackend) {
		return config{}, fmt.Errorf("unsupported document store backend %q", documentStoreBackend)
	}
	return config{
		port:          envInt("SMITH_API_PORT", defaultPort),
		grpcPort:      envInt("SMITH_GRPC_PORT", defaultGRPCPort),
		etcdEndpoints: endpoints,

		etcdDialTimeout:                     envDuration("SMITH_ETCD_DIAL_TIMEOUT", 5*time.Second),
		operatorToken:                       strings.TrimSpace(os.Getenv("SMITH_OPERATOR_TOKEN")),
		authStoreBackend:                    authStoreBackend,
		authStorePath:                       envString("SMITH_AUTH_STORE_PATH", "/tmp/smith-auth/tokens.json"),
		authStoreK8sNamespace:               authStoreK8sNamespace,
		authStoreK8sSecret:                  authStoreK8sSecret,
		authStoreK8sKey:                     authStoreK8sKey,
		defaultPreset:                       strings.TrimSpace(os.Getenv("SMITH_DEFAULT_ENV_PRESET")),
		skillPolicy:                         skillPolicy,
		runtimeNamespace:                    strings.TrimSpace(envString("SMITH_RUNTIME_NAMESPACE", envString("SMITH_NAMESPACE", authStoreK8sNamespace))),
		runtimeContainerName:                strings.TrimSpace(envString("SMITH_RUNTIME_CONTAINER_NAME", "replica")),
		providerClaudeEnabled:               envBool("SMITH_PROVIDER_CLAUDE_ENABLED", true),
		providerGeminiEnabled:               envBool("SMITH_PROVIDER_GEMINI_ENABLED", false),
		documentStoreBackend:                documentStoreBackend,
		documentsPostgresDSN:                strings.TrimSpace(envString("SMITH_DOCUMENTS_POSTGRES_DSN", "")),
		documentsPostgresMaxConns:           envInt32("SMITH_DOCUMENTS_POSTGRES_MAX_CONNS", 10),
		documentsGarageEndpoint:             strings.TrimSpace(envString("SMITH_DOCUMENTS_GARAGE_ENDPOINT", "")),
		documentsGarageRegion:               strings.TrimSpace(envString("SMITH_DOCUMENTS_GARAGE_REGION", "us-east-1")),
		documentsGarageBucket:               strings.TrimSpace(envString("SMITH_DOCUMENTS_GARAGE_BUCKET", "")),
		documentsGarageAccessKeyID:          strings.TrimSpace(envString("SMITH_DOCUMENTS_GARAGE_ACCESS_KEY_ID", "")),
		documentsGarageSecretAccessKey:      strings.TrimSpace(envString("SMITH_DOCUMENTS_GARAGE_SECRET_ACCESS_KEY", "")),
		documentsGarageForcePathStyle:       envBool("SMITH_DOCUMENTS_GARAGE_FORCE_PATH_STYLE", true),
		documentsWatchPollInterval:          envDuration("SMITH_DOCUMENTS_WATCH_POLL_INTERVAL", 2*time.Second),
		documentsMigrationBackfillOnStartup: envBool("SMITH_DOCUMENTS_MIGRATION_BACKFILL_ON_STARTUP", true),
		documentsMigrationReadThroughOnMiss: envBool("SMITH_DOCUMENTS_MIGRATION_READ_THROUGH_ON_MISS", true),
		documentsMigrationMergeListFallback: envBool("SMITH_DOCUMENTS_MIGRATION_MERGE_LIST_FALLBACK", true),
		documentsMigrationDualWriteEtcd:     envBool("SMITH_DOCUMENTS_MIGRATION_DUAL_WRITE_ETCD", false),
	}, nil
}

func deriveLoopID(projectID, idempotencyKey, sourceType, sourceRef string) string {
	prefix := "smi"
	if len(projectID) >= 3 {
		prefix = strings.ToLower(projectID[:3])
	} else if len(projectID) > 0 {
		prefix = strings.ToLower(projectID)
	}

	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		key = sourceType + ":" + sourceRef
	}
	key = strings.ToLower(strings.TrimSpace(key))
	hashInput := key
	if scopedProject := strings.ToLower(strings.TrimSpace(projectID)); scopedProject != "" {
		hashInput = scopedProject + "|" + hashInput
	}
	replacer := strings.NewReplacer("/", "-", "_", "-", ".", "-", " ", "-", ":", "-")
	key = replacer.Replace(key)
	key = strings.Trim(key, "-")

	// Generate a stable short hash for the "xxxxx" part
	h := sha256.New()
	h.Write([]byte(hashInput))
	fullHash := hex.EncodeToString(h.Sum(nil))
	hashPart := fullHash[:5]

	if key == "" {
		// Use a bit more of the hash if key is empty to avoid collisions
		return fmt.Sprintf("%s-%s-%s", prefix, hashPart, fullHash[5:10])
	}

	// Clean key for use in ID
	key = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, key)
	key = collapseRedundantIDSegments(key)
	key = strings.Trim(key, "-")

	if len(key) > 32 {
		key = key[:32]
	}
	key = strings.Trim(key, "-")
	if key == "" {
		return fmt.Sprintf("%s-%s-%s", prefix, hashPart, fullHash[5:10])
	}

	return fmt.Sprintf("%s-%s-%s", prefix, hashPart, key)
}

func collapseRedundantIDSegments(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) == 0 {
		return ""
	}
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(filtered) > 0 && filtered[len(filtered)-1] == part {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, "-")
}

func newIngressSummary(results []ingressResult) ingressSummary {
	created := 0
	existing := 0
	errorsCount := 0
	for _, res := range results {
		switch {
		case res.Status == "error":
			errorsCount++
		case res.Created:
			created++
		default:
			existing++
		}
	}
	var out ingressSummary
	out.Results = results
	out.Summary.Created = created
	out.Summary.Existing = existing
	out.Summary.Errors = errorsCount
	return out
}
func withIdempotency(metadata map[string]string, key string) map[string]string {
	out := map[string]string{}
	for k, v := range metadata {
		out[k] = v
	}
	if strings.TrimSpace(key) != "" {
		out["idempotency_key"] = strings.TrimSpace(key)
	}
	return out
}

func newPresetCatalog(defaultPreset string) *presetCatalog {
	policy := model.DefaultEnvironmentPolicy()
	presets := map[string]struct{}{}
	for name := range policy.AllowedPresets {
		presets[name] = struct{}{}
	}
	resolvedDefault := strings.ToLower(strings.TrimSpace(defaultPreset))
	if resolvedDefault == "" {
		resolvedDefault = policy.DefaultPreset
	}
	if _, ok := presets[resolvedDefault]; !ok {
		presets[resolvedDefault] = struct{}{}
	}
	return &presetCatalog{
		defaultPreset: resolvedDefault,
		presets:       presets,
	}
}

func (c *presetCatalog) Upsert(name string) error {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return errors.New("preset name is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.presets[normalized] = struct{}{}
	return nil
}

func (c *presetCatalog) Has(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.presets[normalized]
	return ok
}

func (c *presetCatalog) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.presets))
	for name := range c.presets {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (c *presetCatalog) Default() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.defaultPreset
}

func (c *presetCatalog) Policy() model.EnvironmentPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	allowed := map[string]struct{}{}
	for name := range c.presets {
		allowed[name] = struct{}{}
	}
	return model.EnvironmentPolicy{
		DefaultPreset:  c.defaultPreset,
		AllowedPresets: allowed,
	}
}

func copyStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func runtimeNamespaceForConfig(cfg config) string {
	namespace := strings.TrimSpace(cfg.runtimeNamespace)
	if namespace == "" {
		return "default"
	}
	return namespace
}

func runtimeContainerForConfig(cfg config) string {
	container := strings.TrimSpace(cfg.runtimeContainerName)
	if container == "" {
		return "replica"
	}
	return container
}

func envString(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
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
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeErrCode(w http.ResponseWriter, status int, errorCode, msg string) {
	writeJSON(w, status, map[string]string{
		"code":  errorCode,
		"error": msg,
	})
}

func parseIntDefault(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

func parseInt64Default(raw string, fallback int64) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func parseFloatDefault(raw string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fallback
	}
	return v
}

func parseRFC3339(raw string) time.Time {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

func formatRFC3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func maskCredentialValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return strings.Repeat("*", len(trimmed))
	}
	return trimmed[:4] + strings.Repeat("*", len(trimmed)-8) + trimmed[len(trimmed)-4:]
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func envInt32(name string, fallback int32) int32 {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || v <= 0 {
		return fallback
	}
	return int32(v)
}

func ioReadAllLimit(body io.Reader, max int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(body, max))
}

type auditBridge struct {
	store *store.Store
}

func (a *auditBridge) RecordAuthEvent(ctx context.Context, event provider.AuthEvent) error {
	if a == nil || a.store == nil {
		return nil
	}
	// Auth lifecycle calls should not block on audit availability.
	auditCtx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	return a.store.AppendAudit(auditCtx, store.AuditRecord{
		Actor:        event.Actor,
		Action:       "auth-" + event.Action,
		TargetLoopID: "",
		Metadata: map[string]string{
			"provider_id": event.ProviderID,
		},
	})
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (s *server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tasks, err := s.store.ListTaskContracts(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := make([]api.TaskContract, 0, len(tasks))
		for _, task := range tasks {
			out = append(out, modelTaskToAPI(task))
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var req taskContractCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		projectID := strings.TrimSpace(req.ProjectID)
		providerProfileID := strings.TrimSpace(req.ProviderProfileID)
		objective := strings.TrimSpace(req.Objective)
		if projectID == "" || providerProfileID == "" || objective == "" {
			writeErr(w, http.StatusBadRequest, "project_id, provider_profile_id, and objective are required")
			return
		}
		validation := normalizeTaskList(req.Validation)
		if len(validation) == 0 {
			writeErr(w, http.StatusBadRequest, "at least one validation command is required")
			return
		}

		status := model.TaskContractStatusDraft
		if strings.TrimSpace(string(req.Status)) != "" {
			status = apiTaskStatusToModel(req.Status)
		}
		if !model.IsTaskContractStatus(status) {
			writeErr(w, http.StatusBadRequest, "invalid task status")
			return
		}
		if status != model.TaskContractStatusDraft && status != model.TaskContractStatusValidated {
			writeErr(w, http.StatusBadRequest, "task must start in draft or validated status")
			return
		}

		taskID := strings.TrimSpace(req.ID)
		if taskID == "" {
			taskID = fmt.Sprintf("task-%d", time.Now().UTC().UnixNano())
		}
		correlationID := strings.TrimSpace(req.CorrelationID)
		if correlationID == "" {
			correlationID = fmt.Sprintf("task-corr-%d", time.Now().UTC().UnixNano())
		}
		task := model.TaskContract{
			Kind:               "smith.task",
			ID:                 taskID,
			ProjectID:          projectID,
			ProviderProfileID:  providerProfileID,
			SourceDocument:     strings.TrimSpace(req.SourceDocument),
			Objective:          objective,
			Constraints:        normalizeTaskList(req.Constraints),
			AcceptanceCriteria: normalizeTaskList(req.AcceptanceCriteria),
			Validation:         validation,
			Status:             status,
			Metadata:           copyStringMap(req.Metadata),
			CorrelationID:      correlationID,
		}
		if err := s.store.PutTaskContract(r.Context(), task); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		actor := strings.TrimSpace(req.Actor)
		if actor == "" {
			actor = "operator"
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  actor,
			Action: "create-task",
			Metadata: map[string]string{
				"task_id": taskID,
				"status":  string(task.Status),
			},
			CorrelationID: correlationID,
		})

		stored, found, err := s.store.GetTaskContract(r.Context(), taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusInternalServerError, "task not found after create")
			return
		}
		writeJSON(w, http.StatusCreated, modelTaskToAPI(stored))
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	taskID, route := splitTaskRoute(r.URL.Path)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		writeErr(w, http.StatusBadRequest, "task id is required")
		return
	}

	if route == "approve" {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req taskContractApproveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		task, found, err := s.store.GetTaskContract(r.Context(), taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}
		if task.Status != model.TaskContractStatusValidated {
			writeErr(w, http.StatusConflict, "task must be validated before approval")
			return
		}
		before := task
		task.Status = model.TaskContractStatusApproved
		if err := s.store.PutTaskContract(r.Context(), task); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		actor := strings.TrimSpace(req.Actor)
		if actor == "" {
			actor = "operator"
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:  actor,
			Action: "approve-task",
			Metadata: map[string]string{
				"task_id":     task.ID,
				"status_from": string(before.Status),
				"status_to":   string(task.Status),
			},
			CorrelationID: task.CorrelationID,
		})
		stored, found, err := s.store.GetTaskContract(r.Context(), taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusInternalServerError, "task not found after approval")
			return
		}
		writeJSON(w, http.StatusOK, modelTaskToAPI(stored))
		return
	}
	if route != "" {
		writeErr(w, http.StatusNotFound, "endpoint not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, found, err := s.store.GetTaskContract(r.Context(), taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}
		writeJSON(w, http.StatusOK, modelTaskToAPI(task))
	case http.MethodPatch:
		var req taskContractPatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		task, found, err := s.store.GetTaskContract(r.Context(), taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusNotFound, "task not found")
			return
		}
		if task.Status != model.TaskContractStatusDraft && task.Status != model.TaskContractStatusValidated {
			writeErr(w, http.StatusConflict, "task can only be patched in draft or validated status")
			return
		}

		before := task
		if req.ProjectID != nil {
			task.ProjectID = strings.TrimSpace(*req.ProjectID)
		}
		if req.ProviderProfileID != nil {
			task.ProviderProfileID = strings.TrimSpace(*req.ProviderProfileID)
		}
		if req.SourceDocument != nil {
			task.SourceDocument = strings.TrimSpace(*req.SourceDocument)
		}
		if req.Objective != nil {
			task.Objective = strings.TrimSpace(*req.Objective)
		}
		if req.Constraints != nil {
			task.Constraints = normalizeTaskList(*req.Constraints)
		}
		if req.AcceptanceCriteria != nil {
			task.AcceptanceCriteria = normalizeTaskList(*req.AcceptanceCriteria)
		}
		if req.Validation != nil {
			task.Validation = normalizeTaskList(*req.Validation)
		}
		if req.Status != nil {
			nextStatus := apiTaskStatusToModel(*req.Status)
			if !model.IsTaskContractStatus(nextStatus) {
				writeErr(w, http.StatusBadRequest, "invalid task status")
				return
			}
			if nextStatus == model.TaskContractStatusApproved {
				writeErr(w, http.StatusConflict, "use approve endpoint for validated->approved transition")
				return
			}
			if !model.IsTaskContractPatchTransitionAllowed(task.Status, nextStatus) {
				writeErr(w, http.StatusConflict, "invalid task status transition")
				return
			}
			task.Status = nextStatus
		}
		if req.Metadata != nil {
			task.Metadata = copyStringMap(req.Metadata)
		}

		if task.ProjectID == "" || task.ProviderProfileID == "" || task.Objective == "" {
			writeErr(w, http.StatusBadRequest, "project_id, provider_profile_id, and objective are required")
			return
		}
		if len(task.Validation) == 0 {
			writeErr(w, http.StatusBadRequest, "at least one validation command is required")
			return
		}
		if err := s.store.PutTaskContract(r.Context(), task); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		changedFields := taskChangedFields(before, task)
		actor := strings.TrimSpace(req.Actor)
		if actor == "" {
			actor = "operator"
		}
		metadata := map[string]string{
			"task_id":        task.ID,
			"status_from":    string(before.Status),
			"status_to":      string(task.Status),
			"changed_fields": strings.Join(changedFields, ","),
		}
		_ = s.appendAudit(r.Context(), store.AuditRecord{
			Actor:         actor,
			Action:        "patch-task",
			CorrelationID: task.CorrelationID,
			Metadata:      metadata,
		})

		stored, found, err := s.store.GetTaskContract(r.Context(), taskID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeErr(w, http.StatusInternalServerError, "task not found after update")
			return
		}
		writeJSON(w, http.StatusOK, modelTaskToAPI(stored))
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func splitTaskRoute(path string) (taskID string, route string) {
	remainder := strings.TrimPrefix(path, "/api/tasks/")
	if remainder == path {
		remainder = strings.TrimPrefix(path, "/v1/tasks/")
	}
	remainder = strings.TrimPrefix(remainder, "/")
	if remainder == "" {
		return "", ""
	}
	parts := strings.Split(remainder, "/")
	taskID = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		route = strings.TrimSpace(strings.Join(parts[1:], "/"))
	}
	return taskID, route
}

func (s *server) ensurePRDStoryTaskContract(ctx context.Context, draft ingress.LoopDraft, idempotencyKey string) (string, map[string]string, error) {
	metadata := copyStringMap(draft.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}

	projectID := strings.TrimSpace(metadata["project_id"])
	providerProfileID := strings.TrimSpace(metadata["provider_profile_id"])
	if projectID != "" && s.projectStore != nil {
		project, found, err := s.projectStore.GetProject(ctx, projectID)
		if err != nil {
			return "", metadata, err
		}
		if !found {
			return "", metadata, fmt.Errorf("project not found")
		}
		if providerProfileID == "" {
			providerProfileID = strings.TrimSpace(project.ProviderProfileID)
		}
	}
	if providerProfileID != "" && s.providers != nil {
		_, found, err := s.providers.GetProviderProfile(ctx, providerProfileID)
		if err != nil {
			return "", metadata, err
		}
		if !found {
			return "", metadata, fmt.Errorf("provider profile not found")
		}
	}
	if providerProfileID != "" {
		metadata["provider_profile_id"] = providerProfileID
	}

	taskID := strings.TrimSpace(metadata["task_contract_id"])
	if taskID == "" {
		seed := strings.TrimSpace(idempotencyKey)
		if seed == "" {
			seed = strings.TrimSpace(draft.SourceRef)
		}
		if seed == "" {
			seed = strings.TrimSpace(draft.Title)
		}
		if seed == "" {
			seed = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
		}
		if projectID != "" {
			seed = projectID + "|" + seed
		}
		taskID = deriveAutoTaskContractID(seed)
	}

	objective := strings.TrimSpace(draft.Title)
	if objective == "" {
		objective = strings.TrimSpace(draft.Description)
	}
	if objective == "" {
		objective = taskID
	}
	sourceDocument := strings.TrimSpace(metadata["document_id"])
	if sourceDocument == "" {
		sourceDocument = strings.TrimSpace(metadata["prd_source_ref"])
	}
	validation := taskValidationCommandsFromMetadata(metadata)
	acceptanceCriteria := prdAcceptanceCriteriaFromMetadata(metadata)

	task, found, err := s.store.GetTaskContract(ctx, taskID)
	if err != nil {
		return "", metadata, err
	}
	if !found {
		task = model.TaskContract{
			Kind:               "smith.task",
			ID:                 taskID,
			ProjectID:          projectID,
			ProviderProfileID:  providerProfileID,
			SourceDocument:     sourceDocument,
			Objective:          objective,
			AcceptanceCriteria: acceptanceCriteria,
			Validation:         validation,
			Metadata:           map[string]string{},
			CorrelationID:      fmt.Sprintf("task-corr-%d", time.Now().UTC().UnixNano()),
		}
	} else {
		if task.ProjectID == "" {
			task.ProjectID = projectID
		}
		if task.ProviderProfileID == "" {
			task.ProviderProfileID = providerProfileID
		}
		if task.Objective == "" {
			task.Objective = objective
		}
		if task.SourceDocument == "" {
			task.SourceDocument = sourceDocument
		}
		if len(task.Validation) == 0 {
			task.Validation = validation
		}
		if len(task.AcceptanceCriteria) == 0 {
			task.AcceptanceCriteria = acceptanceCriteria
		}
		if strings.TrimSpace(task.CorrelationID) == "" {
			task.CorrelationID = fmt.Sprintf("task-corr-%d", time.Now().UTC().UnixNano())
		}
	}

	if task.Metadata == nil {
		task.Metadata = map[string]string{}
	}
	for key, value := range metadata {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		task.Metadata[key] = trimmed
	}
	task.Metadata["task_origin"] = "prd_story"
	task.Metadata["task_auto_generated"] = "true"
	if strings.TrimSpace(draft.SourceRef) != "" {
		task.Metadata["source_ref"] = strings.TrimSpace(draft.SourceRef)
	}
	if strings.TrimSpace(draft.ID) != "" {
		task.Metadata["prd_story_id"] = strings.TrimSpace(draft.ID)
	}

	model.ApplyTaskStatusTransition(&task, model.TaskContractStatusApproved, "", time.Now().UTC())
	if err := s.store.PutTaskContract(ctx, task); err != nil {
		return "", metadata, err
	}

	metadata["task_contract_id"] = task.ID
	if task.ProjectID != "" {
		metadata["project_id"] = task.ProjectID
	}
	if task.ProviderProfileID != "" {
		metadata["provider_profile_id"] = task.ProviderProfileID
	}

	return task.ID, metadata, nil
}

func deriveAutoTaskContractID(seed string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(seed)))
	return "task-" + hex.EncodeToString(hash[:8])
}

func taskValidationCommandsFromMetadata(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}
	if raw := strings.TrimSpace(metadata["task_validation_commands_json"]); raw != "" {
		var parsed []string
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			return normalizeTaskList(parsed)
		}
	}
	if raw := strings.TrimSpace(metadata["task_validation_commands"]); raw != "" {
		return normalizeTaskList(strings.Split(raw, "\n"))
	}
	return nil
}

func prdAcceptanceCriteriaFromMetadata(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}
	raw := strings.TrimSpace(metadata["prd_story_acceptance_criteria_json"])
	if raw == "" {
		return nil
	}
	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	return normalizeTaskList(parsed)
}

func splitProviderRoute(path string) (providerID string, route string) {
	remainder := strings.TrimPrefix(path, "/v1/providers/")
	if remainder == path {
		remainder = strings.TrimPrefix(path, "/api/providers/")
	}
	remainder = strings.TrimPrefix(remainder, "/")
	if remainder == "" {
		return "", ""
	}
	parts := strings.Split(remainder, "/")
	providerID = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		route = strings.TrimSpace(strings.Join(parts[1:], "/"))
	}
	return providerID, route
}

func providerIDFromPath(path string) string {
	id, _ := splitProviderRoute(path)
	return id
}

func projectIDFromPath(path string) string {
	remainder := strings.TrimPrefix(path, "/v1/projects/")
	if remainder == path {
		remainder = strings.TrimPrefix(path, "/api/projects/")
	}
	return strings.TrimSpace(strings.TrimPrefix(remainder, "/"))
}

func normalizeTaskList(in []string) []string {
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

func taskChangedFields(before, after model.TaskContract) []string {
	out := make([]string, 0, 12)
	if before.ProjectID != after.ProjectID {
		out = append(out, "project_id")
	}
	if before.ProviderProfileID != after.ProviderProfileID {
		out = append(out, "provider_profile_id")
	}
	if before.SourceDocument != after.SourceDocument {
		out = append(out, "source_document")
	}
	if before.Objective != after.Objective {
		out = append(out, "objective")
	}
	if !equalStringSlices(before.Constraints, after.Constraints) {
		out = append(out, "constraints")
	}
	if !equalStringSlices(before.AcceptanceCriteria, after.AcceptanceCriteria) {
		out = append(out, "acceptance_criteria")
	}
	if !equalStringSlices(before.Validation, after.Validation) {
		out = append(out, "validation")
	}
	if before.Status != after.Status {
		out = append(out, "status")
	}
	if before.TerminalOutcome != after.TerminalOutcome {
		out = append(out, "terminal_outcome")
	}
	if before.TerminalReason != after.TerminalReason {
		out = append(out, "terminal_reason")
	}
	if (before.TerminalAt == nil) != (after.TerminalAt == nil) || (before.TerminalAt != nil && after.TerminalAt != nil && !before.TerminalAt.Equal(*after.TerminalAt)) {
		out = append(out, "terminal_at")
	}
	if !equalStringMap(before.Metadata, after.Metadata) {
		out = append(out, "metadata")
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func (s *server) syncTaskContractStatusForLoop(ctx context.Context, loopID string, loopState model.LoopState, reason, actor, correlationID string) {
	if s == nil || s.store == nil {
		return
	}
	anomaly, found, err := s.store.GetAnomaly(ctx, loopID)
	if err != nil || !found {
		return
	}
	taskID := strings.TrimSpace(anomaly.Metadata["task_contract_id"])
	if taskID == "" {
		return
	}
	task, found, err := s.store.GetTaskContract(ctx, taskID)
	if err != nil || !found {
		return
	}
	status, shouldUpdate := taskStatusForLoopState(loopState)
	if !shouldUpdate || task.Status == status {
		return
	}
	from := task.Status
	model.ApplyTaskStatusTransition(&task, status, reason, time.Now().UTC())
	if strings.TrimSpace(correlationID) != "" {
		task.CorrelationID = correlationID
	}
	if err := s.store.PutTaskContract(ctx, task); err != nil {
		_ = s.appendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "operator",
			Level:         "warn",
			ActorType:     "api",
			ActorID:       "smith-api",
			Message:       "failed to synchronize task contract status",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"task_contract_id": taskID,
				"loop_state":       string(loopState),
				"target_status":    string(status),
				"error":            err.Error(),
			},
		})
		return
	}
	if strings.TrimSpace(actor) == "" {
		actor = "operator"
	}
	metadata := map[string]string{
		"task_id":                 taskID,
		"status_from":             string(from),
		"status_to":               string(status),
		"loop_state":              string(loopState),
		"sync_trigger":            "loop_state_transition",
		"terminal_outcome":        string(task.TerminalOutcome),
		"terminal_reason_present": strconv.FormatBool(strings.TrimSpace(task.TerminalReason) != ""),
	}
	if task.TerminalAt != nil {
		metadata["terminal_at"] = task.TerminalAt.UTC().Format(time.RFC3339Nano)
	}
	_ = s.appendAudit(ctx, store.AuditRecord{
		Actor:         actor,
		Action:        "sync-task-status",
		TargetLoopID:  loopID,
		Reason:        reason,
		CorrelationID: correlationID,
		Metadata:      metadata,
	})
}

func taskStatusForLoopState(loopState model.LoopState) (model.TaskContractStatus, bool) {
	switch loopState {
	case model.LoopStateRunning:
		return model.TaskContractStatusRunning, true
	case model.LoopStateSynced:
		return model.TaskContractStatusCompleted, true
	case model.LoopStateFlatline, model.LoopStateCancelled:
		return model.TaskContractStatusBlocked, true
	default:
		return "", false
	}
}

func modelTaskToAPI(in model.TaskContract) api.TaskContract {
	return api.TaskContract{
		Kind:               in.Kind,
		ID:                 in.ID,
		ProjectID:          in.ProjectID,
		ProviderProfileID:  in.ProviderProfileID,
		SourceDocument:     in.SourceDocument,
		Objective:          in.Objective,
		Constraints:        in.Constraints,
		AcceptanceCriteria: in.AcceptanceCriteria,
		Validation:         in.Validation,
		Status:             modelTaskStatusToAPI(in.Status),
		TerminalOutcome:    string(in.TerminalOutcome),
		TerminalReason:     in.TerminalReason,
		TerminalAt:         in.TerminalAt,
		Metadata:           copyStringMap(in.Metadata),
		CreatedAt:          in.CreatedAt,
		UpdatedAt:          in.UpdatedAt,
		CorrelationID:      in.CorrelationID,
		SchemaVersion:      in.SchemaVersion,
	}
}

func modelTaskStatusToAPI(in model.TaskContractStatus) api.TaskContractStatus {
	return api.TaskContractStatus(in)
}

func apiTaskStatusToModel(in api.TaskContractStatus) model.TaskContractStatus {
	return model.TaskContractStatus(in)
}

func (s *server) handleDocuments(w http.ResponseWriter, r *http.Request) {
	docStore := s.documentStore()
	if docStore == nil {
		writeErr(w, http.StatusInternalServerError, "document store unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		docs, err := docStore.ListDocuments(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, docs)
	case http.MethodPost:
		var req documentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if req.ProjectID == "" || req.Title == "" || req.Content == "" {
			writeErr(w, http.StatusBadRequest, "project_id, title, and content are required")
			return
		}
		docID := req.ID
		if docID == "" {
			docID = fmt.Sprintf("doc-%d", time.Now().UTC().UnixNano())
		}
		status := req.Status
		if status == "" {
			status = "active"
		}
		doc := model.Document{
			ID:            docID,
			ProjectID:     req.ProjectID,
			Title:         req.Title,
			Content:       req.Content,
			Format:        req.Format,
			SourceType:    req.SourceType,
			SourceRef:     req.SourceRef,
			Status:        status,
			Metadata:      req.Metadata,
			CorrelationID: fmt.Sprintf("doc-corr-%d", time.Now().UTC().UnixNano()),
		}
		if err := docStore.PutDocument(r.Context(), doc); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, doc)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleDocumentByID(w http.ResponseWriter, r *http.Request) {
	docStore := s.documentStore()
	if docStore == nil {
		writeErr(w, http.StatusInternalServerError, "document store unavailable")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/documents/")
	parts := strings.Split(id, "/")
	docID := parts[0]
	if docID == "" {
		writeErr(w, http.StatusBadRequest, "document id is required")
		return
	}
	route := ""
	if len(parts) > 1 {
		route = parts[1]
	}

	doc, found, err := docStore.GetDocument(r.Context(), docID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "document not found")
		return
	}

	if route == "build" {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		// Build instantiates a smith loop from the document content
		// Content is expected to be PRD JSON or Markdown
		format := strings.ToLower(doc.Format)
		if format == "" {
			if strings.HasPrefix(strings.TrimSpace(doc.Content), "{") {
				format = "json"
			} else {
				format = "markdown"
			}
		}

		var drafts []ingress.LoopDraft
		baseMetadata := copyStringMap(doc.Metadata)
		if baseMetadata == nil {
			baseMetadata = make(map[string]string)
		}
		baseMetadata["document_id"] = doc.ID
		baseMetadata["project_id"] = doc.ProjectID

		sourceRef := doc.SourceRef
		if sourceRef == "" {
			sourceRef = fmt.Sprintf("doc:%s", doc.ID)
		}

		var (
			report *model.PRDValidationReport
			err    error
		)
		switch format {
		case "markdown", "md":
			drafts, report, err = buildPRDIngressDrafts("markdown", doc.Content, nil, sourceRef, baseMetadata)
		case "json":
			drafts, report, err = buildPRDIngressDrafts("json", "", json.RawMessage(doc.Content), sourceRef, baseMetadata)
		default:
			err = fmt.Errorf("unsupported document format for build")
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if report != nil && !report.Valid {
			writePRDValidationFailure(w, *report)
			return
		}

		results := make([]ingressResult, 0, len(drafts))
		buildRunID := fmt.Sprintf("build-%d", time.Now().UTC().UnixNano())
		for i, draft := range drafts {
			title := draft.Title
			if doc.Title != "" {
				title = fmt.Sprintf("[%s] %s", doc.Title, draft.Title)
			}
			idempotencyKey := strings.TrimSpace(draft.IdempotencyKey)
			if idempotencyKey == "" {
				idempotencyKey = fmt.Sprintf("%s#%d", strings.TrimSpace(draft.SourceRef), i)
			}
			idempotencyKey = fmt.Sprintf("%s|%s", idempotencyKey, buildRunID)
			metadata := copyStringMap(draft.Metadata)
			taskContractID := ""
			if strings.EqualFold(strings.TrimSpace(draft.SourceType), "prd_story") {
				var bindErr error
				taskContractID, metadata, bindErr = s.ensurePRDStoryTaskContract(r.Context(), draft, idempotencyKey)
				if bindErr != nil {
					results = append(results, ingressResult{
						ItemIndex: i,
						SourceRef: draft.SourceRef,
						Status:    "error",
						Created:   false,
						Message:   bindErr.Error(),
					})
					continue
				}
			}
			res := s.createOneLoop(r.Context(), loopCreateRequest{
				IdempotencyKey: idempotencyKey,
				TaskContractID: taskContractID,
				Title:          title,
				Description:    draft.Description,
				SourceType:     draft.SourceType,
				SourceRef:      draft.SourceRef,
				Metadata:       metadata,
			})
			results = append(results, ingressResult{
				ItemIndex: i,
				LoopID:    res.LoopID,
				SourceRef: draft.SourceRef,
				Status:    res.Status,
				Created:   res.Created,
				Message:   res.Message,
			})
		}
		writeJSON(w, http.StatusOK, newIngressSummary(results))
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, doc)
	case http.MethodPut:
		var req documentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if strings.TrimSpace(req.ProjectID) != "" {
			doc.ProjectID = strings.TrimSpace(req.ProjectID)
		}
		if req.Title != "" {
			doc.Title = req.Title
		}
		if req.Content != "" {
			doc.Content = req.Content
		}
		if req.Format != "" {
			doc.Format = req.Format
		}
		if req.Status != "" {
			doc.Status = req.Status
		}
		if req.Metadata != nil {
			doc.Metadata = req.Metadata
		}
		if err := docStore.PutDocument(r.Context(), doc); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, doc)
	case http.MethodDelete:
		if err := docStore.DeleteDocument(r.Context(), docID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleLoopStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event string, payload any) error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	_ = send("ready", map[string]string{"status": "connected"})

	// Emit initial states
	states, err := s.store.ListStates(r.Context())
	if err == nil {
		for _, loop := range states {
			apiState := modelToApiState(loop.Record)
			anomaly, found, getErr := s.store.GetAnomaly(r.Context(), loop.Record.LoopID)
			if getErr == nil {
				if found {
					enrichLoopStateForPresentation(&apiState, &anomaly)
				} else {
					enrichLoopStateForPresentation(&apiState, nil)
				}
			}
			_ = send("update", api.LoopWithRevision{Record: apiState, Revision: loop.Revision})
		}
	}

	// Watch for updates
	events := s.store.WatchState(r.Context())
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if ev.HasState {
				apiState := modelToApiState(ev.State)
				anomaly, found, err := s.store.GetAnomaly(r.Context(), ev.State.LoopID)
				if err == nil {
					if found {
						enrichLoopStateForPresentation(&apiState, &anomaly)
					} else {
						enrichLoopStateForPresentation(&apiState, nil)
					}
				}
				_ = send("update", api.LoopWithRevision{Record: apiState, Revision: ev.Revision})
			}
		}
	}
}

func (s *server) handleDocumentStream(w http.ResponseWriter, r *http.Request) {
	docStore := s.documentStore()
	if docStore == nil {
		writeErr(w, http.StatusInternalServerError, "document store unavailable")
		return
	}
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event string, payload any) error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	_ = send("ready", map[string]string{"status": "connected"})

	docs, err := docStore.ListDocuments(r.Context())
	if err == nil {
		for _, doc := range docs {
			_ = send("update", doc)
		}
	}

	events := docStore.WatchDocuments(r.Context())
	for {
		select {
		case <-r.Context().Done():
			return
		case doc, ok := <-events:
			if !ok {
				return
			}
			_ = send("update", doc)
		}
	}
}

func (s *server) handleAuditStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event string, payload any) error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	_ = send("ready", map[string]string{"status": "connected"})

	events := s.store.WatchAudit(r.Context())
	for {
		select {
		case <-r.Context().Done():
			return
		case rec, ok := <-events:
			if !ok {
				return
			}
			_ = send("update", rec)
		}
	}
}

func apiToModelEnvironment(in *api.LoopEnvironment) *model.LoopEnvironment {
	if in == nil {
		return nil
	}
	out := &model.LoopEnvironment{
		Preset:       in.Preset,
		Env:          in.Env,
		ResolvedMode: in.ResolvedMode,
	}
	if in.Mise != nil {
		out.Mise = &model.MiseEnvironment{
			ToolVersionsFile: in.Mise.ToolVersionsFile,
			Tools:            in.Mise.Tools,
		}
	}
	if in.ContainerImage != nil {
		out.ContainerImage = &model.ContainerImageProfile{
			Ref:        in.ContainerImage.Ref,
			PullPolicy: in.ContainerImage.PullPolicy,
		}
	} else if in.ImageRef != "" {
		out.ContainerImage = &model.ContainerImageProfile{
			Ref:        in.ImageRef,
			PullPolicy: in.ImagePullPolicy,
		}
	}
	if in.Dockerfile != nil {
		out.Dockerfile = &model.DockerfileProfile{
			ContextDir:     in.Dockerfile.ContextDir,
			DockerfilePath: in.Dockerfile.DockerfilePath,
			Target:         in.Dockerfile.Target,
			BuildArgs:      in.Dockerfile.BuildArgs,
		}
	}
	return out
}

func modelToApiEnvironment(in model.LoopEnvironment) api.LoopEnvironment {
	out := api.LoopEnvironment{
		Preset:       in.Preset,
		Env:          in.Env,
		ResolvedMode: in.ResolvedMode,
	}
	if in.Mise != nil {
		out.Mise = &api.MiseEnvironment{
			ToolVersionsFile: in.Mise.ToolVersionsFile,
			Tools:            in.Mise.Tools,
		}
	}
	if in.ContainerImage != nil {
		out.ContainerImage = &api.ContainerImageProfile{
			Ref:        in.ContainerImage.Ref,
			PullPolicy: in.ContainerImage.PullPolicy,
		}
	}
	if in.Dockerfile != nil {
		out.Dockerfile = &api.DockerfileProfile{
			ContextDir:     in.Dockerfile.ContextDir,
			DockerfilePath: in.Dockerfile.DockerfilePath,
			Target:         in.Dockerfile.Target,
			BuildArgs:      in.Dockerfile.BuildArgs,
		}
	}
	return out
}

func apiToModelSkills(in []api.LoopSkillMount) []model.LoopSkillMount {
	if in == nil {
		return nil
	}
	out := make([]model.LoopSkillMount, 0, len(in))
	for _, s := range in {
		out = append(out, model.LoopSkillMount{
			Name:      s.Name,
			Source:    s.Source,
			Version:   s.Version,
			MountPath: s.MountPath,
			ReadOnly:  s.ReadOnly,
		})
	}
	return out
}

func modelToApiSkills(in []model.LoopSkillMount) []api.LoopSkillMount {
	if in == nil {
		return nil
	}
	out := make([]api.LoopSkillMount, 0, len(in))
	for _, s := range in {
		out = append(out, api.LoopSkillMount{
			Name:      s.Name,
			Source:    s.Source,
			Version:   s.Version,
			MountPath: s.MountPath,
			ReadOnly:  s.ReadOnly,
		})
	}
	return out
}

func modelToApiState(in model.State) api.State {
	return api.State{
		LoopID:           in.LoopID,
		State:            api.LoopState(in.State),
		Attempt:          in.Attempt,
		Reason:           in.Reason,
		WorkerJobName:    in.WorkerJobName,
		LockHolder:       in.LockHolder,
		ObservedRevision: in.ObservedRevision,
		UpdatedAt:        in.UpdatedAt,
		LastHeartbeatAt:  in.LastHeartbeatAt,
		CorrelationID:    in.CorrelationID,
		SchemaVersion:    in.SchemaVersion,
	}
}

func enrichLoopStateForPresentation(state *api.State, anomaly *model.Anomaly) {
	if state == nil {
		return
	}
	maxAttempts := 0
	if anomaly != nil {
		maxAttempts = anomaly.Policy.MaxAttempts
	}
	state.CurrentCount, state.TargetCount = deriveLoopProgressCounts(state.Attempt, maxAttempts)
	state.DisplayTitle = deriveLoopDisplayTitle(state.LoopID, anomaly)
}

func deriveLoopProgressCounts(attempt, maxAttempts int) (int, int) {
	current := attempt
	if current < 0 {
		current = 0
	}
	target := maxAttempts
	if target < 0 {
		target = 0
	}
	if target == 0 {
		if current == 0 {
			target = 1
		} else {
			target = current
		}
	}
	if current > target {
		target = current
	}
	return current, target
}

func deriveLoopDisplayTitle(loopID string, anomaly *model.Anomaly) string {
	if anomaly == nil {
		return strings.TrimSpace(loopID)
	}
	if title := strings.TrimSpace(anomaly.Metadata["display_title"]); title != "" {
		return title
	}
	storyID := strings.TrimSpace(anomaly.Metadata["prd_story_id"])
	title := strings.TrimSpace(anomaly.Title)
	if storyID != "" && title != "" {
		return storyID + ": " + title
	}
	if title != "" {
		return title
	}
	if sourceRef := strings.TrimSpace(anomaly.SourceRef); sourceRef != "" {
		return sourceRef
	}
	return strings.TrimSpace(loopID)
}

func modelToApiAnomaly(in model.Anomaly) api.Anomaly {
	return api.Anomaly{
		ID:            in.ID,
		Title:         in.Title,
		Description:   in.Description,
		SourceType:    in.SourceType,
		SourceRef:     in.SourceRef,
		ProviderID:    in.ProviderID,
		Model:         in.Model,
		Environment:   modelToApiEnvironment(in.Environment),
		Skills:        modelToApiSkills(in.Skills),
		Policy:        api.LoopPolicy(in.Policy),
		Metadata:      in.Metadata,
		CreatedAt:     in.CreatedAt,
		UpdatedAt:     in.UpdatedAt,
		CorrelationID: in.CorrelationID,
		SchemaVersion: in.SchemaVersion,
	}
}
func ptr[T any](v T) *T { return &v }

func modelToApiJournalEntry(in model.JournalEntry) api.JournalEntry {
	return api.JournalEntry{
		LoopID:        in.LoopID,
		Sequence:      in.Sequence,
		Timestamp:     in.Timestamp,
		Phase:         in.Phase,
		Level:         in.Level,
		ActorType:     in.ActorType,
		ActorID:       in.ActorID,
		Message:       in.Message,
		Command:       in.Command,
		ExitCode:      in.ExitCode,
		Metadata:      in.Metadata,
		CorrelationID: in.CorrelationID,
		SchemaVersion: in.SchemaVersion,
	}
}

func modelToApiHandoff(in model.Handoff) api.Handoff {
	return api.Handoff{
		LoopID:            in.LoopID,
		Sequence:          in.Sequence,
		Timestamp:         in.Timestamp,
		FinalDiffSummary:  in.FinalDiffSummary,
		ValidationState:   in.ValidationState,
		ValidationDetails: in.ValidationDetails,
		NextSteps:         in.NextSteps,
		ArtifactRefs:      in.ArtifactRefs,
		Metadata:          in.Metadata,
		CorrelationID:     in.CorrelationID,
		SchemaVersion:     in.SchemaVersion,
	}
}

func modelToApiOverride(in model.OperatorOverride) api.OperatorOverride {
	return api.OperatorOverride{
		LoopID:        in.LoopID,
		Sequence:      in.Sequence,
		Timestamp:     in.Timestamp,
		Actor:         in.Actor,
		Action:        in.Action,
		TargetState:   api.LoopState(in.TargetState),
		Reason:        in.Reason,
		CorrelationID: in.CorrelationID,
		SchemaVersion: in.SchemaVersion,
	}
}

func storeToApiAudit(in store.AuditRecord) api.AuditRecord {
	return api.AuditRecord{
		EventID:       in.EventID,
		Timestamp:     in.Timestamp,
		Actor:         in.Actor,
		Action:        in.Action,
		TargetLoopID:  in.TargetLoopID,
		Reason:        in.Reason,
		CorrelationID: in.CorrelationID,
		Metadata:      in.Metadata,
		SchemaVersion: in.SchemaVersion,
	}
}

func modelToApiJournalEntries(in []model.JournalEntry) []api.JournalEntry {
	if in == nil {
		return nil
	}
	out := make([]api.JournalEntry, 0, len(in))
	for _, e := range in {
		out = append(out, modelToApiJournalEntry(e))
	}
	return out
}

func modelToApiHandoffs(in []model.Handoff) []api.Handoff {
	if in == nil {
		return nil
	}
	out := make([]api.Handoff, 0, len(in))
	for _, h := range in {
		out = append(out, modelToApiHandoff(h))
	}
	return out
}

func modelToApiOverrides(in []model.OperatorOverride) []api.OperatorOverride {
	if in == nil {
		return nil
	}
	out := make([]api.OperatorOverride, 0, len(in))
	for _, o := range in {
		out = append(out, modelToApiOverride(o))
	}
	return out
}

func storeToApiAudits(in []store.AuditRecord) []api.AuditRecord {
	if in == nil {
		return nil
	}
	out := make([]api.AuditRecord, 0, len(in))
	for _, a := range in {
		out = append(out, storeToApiAudit(a))
	}
	return out
}

type grpcServer struct {
	pb.UnimplementedSmithServiceServer
	store       store.StateStore
	presets     *presetCatalog
	skillPolicy model.SkillPolicy
}

func (s *grpcServer) ListLoops(ctx context.Context, req *pb.ListLoopsRequest) (*pb.ListLoopsResponse, error) {
	states, err := s.store.ListStates(ctx)
	if err != nil {
		return nil, err
	}

	var loops []*pb.LoopWithRevision
	for _, l := range states {
		loops = append(loops, &pb.LoopWithRevision{
			Record:   modelToPbState(l.Record),
			Revision: l.Revision,
		})
	}

	return &pb.ListLoopsResponse{Loops: loops}, nil
}

func (s *grpcServer) CreateLoop(ctx context.Context, req *pb.LoopCreateRequest) (*pb.LoopCreateResult, error) {
	reg := provider.NewDefaultRegistry()
	selection, err := reg.Resolve(req.ProviderId, req.Model)
	if err != nil {
		return nil, err
	}

	environment, err := model.NormalizeLoopEnvironmentWithPolicy(pbToModelEnvironment(req.Environment), s.presets.Policy())
	if err != nil {
		return nil, err
	}

	skills, _, err := model.NormalizeLoopSkillsWithPolicy(pbToModelSkills(req.Skills), selection.ProviderID, s.skillPolicy)
	if err != nil {
		return nil, err
	}

	loopID := strings.TrimSpace(req.LoopId)
	if loopID == "" {
		loopID = deriveLoopID("", req.IdempotencyKey, req.SourceType, req.SourceRef)
	}

	existing, found, err := s.store.GetState(ctx, loopID)
	if err == nil && found {
		stored, _, _ := s.store.GetAnomaly(ctx, loopID)
		return &pb.LoopCreateResult{
			LoopId:      loopID,
			Status:      string(existing.Record.State),
			Created:     false,
			Message:     "existing loop returned via idempotency or explicit loop_id",
			Environment: modelToPbEnvironment(stored.Environment),
			Skills:      modelToPbSkills(stored.Skills),
		}, nil
	}

	anomaly := model.Anomaly{
		ID:            loopID,
		Title:         req.Title,
		Description:   req.Description,
		SourceType:    req.SourceType,
		SourceRef:     req.SourceRef,
		ProviderID:    selection.ProviderID,
		Model:         selection.Model,
		Environment:   environment,
		Skills:        skills,
		Metadata:      req.Metadata,
		CorrelationID: req.CorrelationId,
	}

	err = s.store.PutAnomaly(ctx, anomaly)
	if err != nil {
		return nil, err
	}

	state := model.StateRecord{
		LoopID:        loopID,
		State:         model.LoopStateUnresolved,
		Reason:        "created-via-grpc",
		CorrelationID: req.CorrelationId,
	}

	_, err = s.store.PutState(ctx, state, 0)
	if err != nil {
		return nil, err
	}

	return &pb.LoopCreateResult{
		LoopId:      loopID,
		Status:      string(model.LoopStateUnresolved),
		Created:     true,
		Environment: modelToPbEnvironment(environment),
		Skills:      modelToPbSkills(skills),
	}, nil
}

func (s *grpcServer) GetLoop(ctx context.Context, req *pb.GetLoopRequest) (*pb.LoopResponse, error) {
	state, found, err := s.store.GetState(ctx, req.LoopId)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("loop not found")
	}

	res := &pb.LoopResponse{
		State: modelToPbState(state.Record),
	}

	anomaly, found, err := s.store.GetAnomaly(ctx, req.LoopId)
	if err == nil && found {
		res.Anomaly = modelToPbAnomaly(anomaly)
		res.Environment = modelToPbEnvironment(anomaly.Environment)
	}

	return res, nil
}

func (s *grpcServer) DeleteLoop(ctx context.Context, req *pb.LoopDeleteRequest) (*pb.DeleteLoopResponse, error) {
	state, found, err := s.store.GetState(ctx, req.LoopId)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("loop not found")
	}

	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "grpc-client"
	}

	next := state.Record
	next.State = model.LoopStateCancelled
	next.Reason = fmt.Sprintf("cancelled by %s", actor)
	next.LockHolder = ""

	_, err = s.store.PutState(ctx, next, state.Revision)
	if err != nil {
		return nil, err
	}

	_ = s.store.AppendAudit(ctx, store.AuditRecord{
		Actor:        actor,
		Action:       "delete-loop",
		TargetLoopID: req.LoopId,
		Reason:       "request via grpc",
	})

	return &pb.DeleteLoopResponse{
		LoopId: req.LoopId,
		Status: "deleted",
		Actor:  actor,
	}, nil
}

func (s *grpcServer) TraceLoop(ctx context.Context, req *pb.TraceLoopRequest) (*pb.LoopTraceResponse, error) {
	state, found, err := s.store.GetState(ctx, req.LoopId)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("loop not found")
	}

	out := &pb.LoopTraceResponse{
		LoopId: req.LoopId,
		State:  modelToPbState(state.Record),
	}

	anomaly, anomalyFound, err := s.store.GetAnomaly(ctx, req.LoopId)
	if err == nil && anomalyFound {
		out.Anomaly = modelToPbAnomaly(anomaly)
		out.Environment = modelToPbEnvironment(anomaly.Environment)
	}

	journal, _ := s.store.ListJournal(ctx, req.LoopId, 500)
	for _, j := range journal {
		out.Journal = append(out.Journal, modelToPbJournalEntry(j))
	}

	handoffs, _ := s.store.ListHandoffs(ctx, req.LoopId, 500)
	for _, h := range handoffs {
		out.Handoffs = append(out.Handoffs, modelToPbHandoff(h))
	}

	overrides, _ := s.store.ListOverrides(ctx, req.LoopId, 500)
	for _, o := range overrides {
		out.Overrides = append(out.Overrides, modelToPbOverride(o))
	}

	audit, _ := s.store.ListAudit(ctx, req.LoopId, 500)
	for _, a := range audit {
		out.Audit = append(out.Audit, storeToPbAudit(a))
	}

	return out, nil
}

func (s *grpcServer) GetJournal(ctx context.Context, req *pb.GetJournalRequest) (*pb.GetJournalResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 500
	}

	journal, err := s.store.ListJournal(ctx, req.LoopId, limit)
	if err != nil {
		return nil, err
	}

	var entries []*pb.JournalEntry
	for _, j := range journal {
		entries = append(entries, modelToPbJournalEntry(j))
	}

	return &pb.GetJournalResponse{Entries: entries}, nil
}

func (s *grpcServer) OverrideLoop(ctx context.Context, req *pb.OverrideRequest) (*pb.OverrideResponse, error) {
	state, found, err := s.store.GetState(ctx, req.LoopId)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("loop not found")
	}

	targetState := pbToModelLoopState(req.TargetState)
	if !model.IsValidTransition(state.Record.State, targetState) {
		return nil, fmt.Errorf("invalid transition %s -> %s", state.Record.State, targetState)
	}

	next := state.Record
	next.State = targetState
	next.Reason = req.Reason
	next.LockHolder = "operator-override"

	rev, err := s.store.PutState(ctx, next, state.Revision)
	if err != nil {
		return nil, err
	}

	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "grpc-client"
	}

	_ = s.store.AppendOverride(ctx, model.OperatorOverride{
		LoopID:        req.LoopId,
		Actor:         actor,
		Action:        "override-state",
		TargetState:   targetState,
		Reason:        req.Reason,
		CorrelationID: next.CorrelationID,
	})

	return &pb.OverrideResponse{
		LoopId:   req.LoopId,
		Status:   "overridden",
		State:    modelToPbState(next),
		Revision: rev,
	}, nil
}

// Helper conversions

func modelToPbState(in model.StateRecord) *pb.State {
	return &pb.State{
		LoopId:           in.LoopID,
		State:            modelToPbLoopState(in.State),
		Attempt:          int32(in.Attempt),
		Reason:           in.Reason,
		WorkerJobName:    in.WorkerJobName,
		LockHolder:       in.LockHolder,
		ObservedRevision: in.ObservedRevision,
		UpdatedAt:        timestamppb.New(in.UpdatedAt),
		LastHeartbeatAt:  timestamppb.New(safeTime(in.LastHeartbeatAt)),
		CorrelationId:    in.CorrelationID,
		SchemaVersion:    in.SchemaVersion,
	}
}

func modelToPbLoopState(in model.LoopState) pb.LoopState {
	switch in {
	case model.LoopStateUnresolved:
		return pb.LoopState_LOOP_STATE_UNRESOLVED
	case model.LoopStateRunning:
		return pb.LoopState_LOOP_STATE_RUNNING
	case model.LoopStateSynced:
		return pb.LoopState_LOOP_STATE_SYNCED
	case model.LoopStateFlatline:
		return pb.LoopState_LOOP_STATE_FLATLINE
	case model.LoopStateCancelled:
		return pb.LoopState_LOOP_STATE_CANCELLED
	default:
		return pb.LoopState_LOOP_STATE_UNSPECIFIED
	}
}

func pbToModelLoopState(in pb.LoopState) model.LoopState {
	switch in {
	case pb.LoopState_LOOP_STATE_UNRESOLVED:
		return model.LoopStateUnresolved
	case pb.LoopState_LOOP_STATE_RUNNING:
		return model.LoopStateRunning
	case pb.LoopState_LOOP_STATE_SYNCED:
		return model.LoopStateSynced
	case pb.LoopState_LOOP_STATE_FLATLINE:
		return model.LoopStateFlatline
	case pb.LoopState_LOOP_STATE_CANCELLED:
		return model.LoopStateCancelled
	default:
		return model.LoopStateUnresolved // Default
	}
}

func modelToPbEnvironment(in model.LoopEnvironment) *pb.LoopEnvironment {
	out := &pb.LoopEnvironment{
		Preset:       in.Preset,
		Env:          in.Env,
		ResolvedMode: in.ResolvedMode,
	}
	if in.Mise != nil {
		out.Mise = &pb.MiseEnvironment{
			ToolVersionsFile: in.Mise.ToolVersionsFile,
			Tools:            in.Mise.Tools,
		}
	}
	if in.ContainerImage != nil {
		out.ContainerImage = &pb.ContainerImageProfile{
			Ref:        in.ContainerImage.Ref,
			PullPolicy: in.ContainerImage.PullPolicy,
		}
	}
	if in.Dockerfile != nil {
		out.Dockerfile = &pb.DockerfileProfile{
			ContextDir:     in.Dockerfile.ContextDir,
			DockerfilePath: in.Dockerfile.DockerfilePath,
			Target:         in.Dockerfile.Target,
			BuildArgs:      in.Dockerfile.BuildArgs,
		}
	}
	return out
}

func pbToModelEnvironment(in *pb.LoopEnvironment) *model.LoopEnvironment {
	if in == nil {
		return nil
	}
	out := &model.LoopEnvironment{
		Preset:       in.Preset,
		Env:          in.Env,
		ResolvedMode: in.ResolvedMode,
	}
	if in.Mise != nil {
		out.Mise = &model.MiseEnvironment{
			ToolVersionsFile: in.Mise.ToolVersionsFile,
			Tools:            in.Mise.Tools,
		}
	}
	if in.ContainerImage != nil {
		out.ContainerImage = &model.ContainerImageProfile{
			Ref:        in.ContainerImage.Ref,
			PullPolicy: in.ContainerImage.PullPolicy,
		}
	}
	if in.Dockerfile != nil {
		out.Dockerfile = &model.DockerfileProfile{
			ContextDir:     in.Dockerfile.ContextDir,
			DockerfilePath: in.Dockerfile.DockerfilePath,
			Target:         in.Dockerfile.Target,
			BuildArgs:      in.Dockerfile.BuildArgs,
		}
	}
	return out
}

func modelToPbSkills(in []model.LoopSkillMount) []*pb.LoopSkillMount {
	var out []*pb.LoopSkillMount
	for _, s := range in {
		out = append(out, &pb.LoopSkillMount{
			Name:      s.Name,
			Source:    s.Source,
			Version:   s.Version,
			MountPath: s.MountPath,
			ReadOnly:  safeBool(s.ReadOnly),
			Config:    nil, // Model doesn't have config yet
		})
	}
	return out
}

func pbToModelSkills(in []*pb.LoopSkillMount) []model.LoopSkillMount {
	var out []model.LoopSkillMount
	for _, s := range in {
		out = append(out, model.LoopSkillMount{
			Name:      s.Name,
			Source:    s.Source,
			Version:   s.Version,
			MountPath: s.MountPath,
			ReadOnly:  ptr(s.ReadOnly),
		})
	}
	return out
}

func modelToPbAnomaly(in model.Anomaly) *pb.Anomaly {
	return &pb.Anomaly{
		Id:            in.ID,
		Title:         in.Title,
		Description:   in.Description,
		SourceType:    in.SourceType,
		SourceRef:     in.SourceRef,
		ProviderId:    in.ProviderID,
		Model:         in.Model,
		Environment:   modelToPbEnvironment(in.Environment),
		Skills:        modelToPbSkills(in.Skills),
		Policy:        modelToPbPolicy(in.Policy),
		Metadata:      in.Metadata,
		CreatedAt:     timestamppb.New(in.CreatedAt),
		UpdatedAt:     timestamppb.New(in.UpdatedAt),
		CorrelationId: in.CorrelationID,
		SchemaVersion: in.SchemaVersion,
	}
}

func modelToPbPolicy(in model.LoopPolicy) *pb.LoopPolicy {
	return &pb.LoopPolicy{
		MaxAttempts:         int32(in.MaxAttempts),
		BackoffInitialNanos: in.BackoffInitial.Nanoseconds(),
		BackoffMaxNanos:     in.BackoffMax.Nanoseconds(),
		TimeoutNanos:        in.Timeout.Nanoseconds(),
		TerminateOnError:    in.TerminateOnError,
	}
}

func modelToPbJournalEntry(in model.JournalEntry) *pb.JournalEntry {
	return &pb.JournalEntry{
		LoopId:        in.LoopID,
		Sequence:      in.Sequence,
		Timestamp:     timestamppb.New(in.Timestamp),
		Phase:         in.Phase,
		Level:         in.Level,
		ActorType:     in.ActorType,
		ActorId:       in.ActorID,
		Message:       in.Message,
		Command:       in.Command,
		ExitCode:      int32(safeIntPtr(in.ExitCode)),
		Metadata:      in.Metadata,
		CorrelationId: in.CorrelationID,
		SchemaVersion: in.SchemaVersion,
	}
}

func modelToPbHandoff(in model.Handoff) *pb.Handoff {
	return &pb.Handoff{
		LoopId:            in.LoopID,
		Sequence:          in.Sequence,
		Timestamp:         timestamppb.New(in.Timestamp),
		FinalDiffSummary:  in.FinalDiffSummary,
		ValidationState:   in.ValidationState,
		ValidationDetails: in.ValidationDetails,
		NextSteps:         in.NextSteps,
		ArtifactRefs:      in.ArtifactRefs,
		Metadata:          in.Metadata,
		CorrelationId:     in.CorrelationID,
		SchemaVersion:     in.SchemaVersion,
	}
}

func modelToPbOverride(in model.OperatorOverride) *pb.OperatorOverride {
	return &pb.OperatorOverride{
		LoopId:        in.LoopID,
		Sequence:      in.Sequence,
		Timestamp:     timestamppb.New(in.Timestamp),
		Actor:         in.Actor,
		Action:        in.Action,
		TargetState:   modelToPbLoopState(in.TargetState),
		Reason:        in.Reason,
		CorrelationId: in.CorrelationID,
		SchemaVersion: in.SchemaVersion,
	}
}

func storeToPbAudit(in store.AuditRecord) *pb.AuditRecord {
	return &pb.AuditRecord{
		EventId:       in.EventID,
		Timestamp:     timestamppb.New(in.Timestamp),
		Actor:         in.Actor,
		Action:        in.Action,
		TargetLoopId:  in.TargetLoopID,
		Reason:        in.Reason,
		CorrelationId: in.CorrelationID,
		Metadata:      in.Metadata,
		SchemaVersion: in.SchemaVersion,
	}
}

func safeTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func safeBool(b *bool) bool {
	if b == nil {
		return true
	}
	return *b
}

func safeIntPtr(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
