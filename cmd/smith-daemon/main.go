package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"smith/internal/source/model"
	"smith/internal/source/store"
)

const (
	defaultPort               = 8082
	defaultShutdownTimeout    = 10 * time.Second
	defaultCleanupInterval    = 10 * time.Minute
	defaultCleanupTimeout     = 30 * time.Second
	defaultRetentionFlatline  = 48 * time.Hour
	defaultRetentionCancelled = 48 * time.Hour
	defaultRetentionSynced    = 0 * time.Second
	defaultCleanupMaxDeletes  = 200
	defaultCleanupActor       = "smith-daemon"
)

type config struct {
	port int

	etcdEndpoints   []string
	etcdDialTimeout time.Duration

	cleanupInterval   time.Duration
	cleanupTimeout    time.Duration
	cleanupDryRun     bool
	cleanupMaxDeletes int
	cleanupActor      string
	policyPath        string

	retentionFlatline  time.Duration
	retentionCancelled time.Duration
	retentionSynced    time.Duration

	runtimeNamespace string
}

type cleanupPolicyFile struct {
	Retention struct {
		Flatline  string `yaml:"flatline"`
		Cancelled string `yaml:"cancelled"`
		Synced    string `yaml:"synced"`
	} `yaml:"retention"`
}

type daemon struct {
	cfg   config
	store store.StateStore
	kube  kubernetes.Interface
}

type cleanupStats struct {
	Eligible     int
	Processed    int
	Deleted      int
	DryRun       int
	Errors       int
	RuntimeError int
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("smith-daemon config error: %v", err)
	}

	es, err := store.New(ctx, cfg.etcdEndpoints, cfg.etcdDialTimeout)
	if err != nil {
		log.Fatalf("smith-daemon etcd init failed: %v", err)
	}
	defer func() { _ = es.Close() }()

	kube, err := kubeClient()
	if err != nil {
		log.Printf("smith-daemon kubernetes client unavailable: %v", err)
	}

	d := &daemon{cfg: cfg, store: es, kube: kube}
	go d.runCleanupLoop(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	addr := fmt.Sprintf(":%d", cfg.port)
	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("smith-daemon listening on %s", addr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Printf("smith-daemon shutdown requested")
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("smith-daemon failed: %v", serveErr)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("smith-daemon shutdown failed: %v", err)
	}
}

func (d *daemon) runCleanupLoop(ctx context.Context) {
	d.runCleanupPassWithTimeout(ctx)

	ticker := time.NewTicker(d.cfg.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.runCleanupPassWithTimeout(ctx)
		}
	}
}

func (d *daemon) runCleanupPassWithTimeout(ctx context.Context) {
	if err := applyRetentionPolicyFromFile(&d.cfg); err != nil {
		log.Printf("cleanup policy reload failed: %v", err)
	}

	passCtx, cancel := context.WithTimeout(ctx, d.cfg.cleanupTimeout)
	defer cancel()

	stats, err := d.runCleanupPass(passCtx, time.Now().UTC())
	if err != nil {
		log.Printf("cleanup pass failed: %v", err)
		return
	}
	log.Printf(
		"cleanup pass complete eligible=%d processed=%d deleted=%d dry_run=%d errors=%d runtime_errors=%d",
		stats.Eligible,
		stats.Processed,
		stats.Deleted,
		stats.DryRun,
		stats.Errors,
		stats.RuntimeError,
	)
}

func (d *daemon) runCleanupPass(ctx context.Context, now time.Time) (cleanupStats, error) {
	var stats cleanupStats

	states, err := d.store.ListStates(ctx)
	if err != nil {
		return stats, err
	}

	candidates := selectRetentionCandidates(states, now, d.cfg)
	stats.Eligible = len(candidates)

	if d.cfg.cleanupMaxDeletes > 0 && len(candidates) > d.cfg.cleanupMaxDeletes {
		candidates = candidates[:d.cfg.cleanupMaxDeletes]
	}
	stats.Processed = len(candidates)

	for _, candidate := range candidates {
		if d.cfg.cleanupDryRun {
			stats.DryRun++
			log.Printf("cleanup dry-run loop=%s state=%s updated_at=%s", candidate.Record.LoopID, candidate.Record.State, candidate.Record.UpdatedAt.UTC().Format(time.RFC3339))
			continue
		}

		if err := d.store.DeleteLoop(ctx, candidate.Record.LoopID); err != nil {
			stats.Errors++
			log.Printf("cleanup delete failed loop=%s: %v", candidate.Record.LoopID, err)
			continue
		}

		runtimeCleanup := "skipped"
		if err := d.cleanupLoopRuntimeArtifacts(ctx, candidate.Record); err != nil {
			stats.RuntimeError++
			runtimeCleanup = "error"
			log.Printf("cleanup runtime cleanup failed loop=%s: %v", candidate.Record.LoopID, err)
		} else if d.kube != nil && strings.TrimSpace(candidate.Record.WorkerJobName) != "" {
			runtimeCleanup = "deleted"
		}

		retention := d.retentionForState(candidate.Record.State)
		if err := d.store.AppendAudit(ctx, store.AuditRecord{
			Actor:         d.cfg.cleanupActor,
			Action:        "delete-loop",
			TargetLoopID:  candidate.Record.LoopID,
			CorrelationID: candidate.Record.CorrelationID,
			Reason:        "retention-expired",
			Metadata: map[string]string{
				"cleanup":           "retention",
				"final_state":       string(candidate.Record.State),
				"retention_seconds": strconv.FormatInt(int64(retention.Seconds()), 10),
				"updated_at":        candidate.Record.UpdatedAt.UTC().Format(time.RFC3339),
				"runtime":           runtimeCleanup,
			},
		}); err != nil {
			log.Printf("cleanup audit append failed loop=%s: %v", candidate.Record.LoopID, err)
		}

		stats.Deleted++
	}

	return stats, nil
}

func (d *daemon) retentionForState(state model.LoopState) time.Duration {
	return retentionForState(state, d.cfg)
}

func retentionForState(state model.LoopState, cfg config) time.Duration {
	switch state {
	case model.LoopStateFlatline:
		return cfg.retentionFlatline
	case model.LoopStateCancelled:
		return cfg.retentionCancelled
	case model.LoopStateSynced:
		return cfg.retentionSynced
	default:
		return 0
	}
}

func selectRetentionCandidates(states []store.LoopWithRevision, now time.Time, cfg config) []store.LoopWithRevision {
	out := make([]store.LoopWithRevision, 0)
	for _, loop := range states {
		retention := retentionForState(loop.Record.State, cfg)
		if retention <= 0 {
			continue
		}
		if loop.Record.UpdatedAt.IsZero() {
			continue
		}
		if now.Sub(loop.Record.UpdatedAt) < retention {
			continue
		}
		out = append(out, loop)
	}

	sort.Slice(out, func(i, j int) bool {
		left := out[i].Record.UpdatedAt
		right := out[j].Record.UpdatedAt
		if left.Equal(right) {
			return out[i].Record.LoopID < out[j].Record.LoopID
		}
		return left.Before(right)
	})

	return out
}

func (d *daemon) cleanupLoopRuntimeArtifacts(ctx context.Context, record model.StateRecord) error {
	if d.kube == nil {
		return nil
	}
	jobName := strings.TrimSpace(record.WorkerJobName)
	if jobName == "" {
		return nil
	}

	namespace := runtimeNamespaceForConfig(d.cfg)
	propagation := metav1.DeletePropagationBackground
	if err := d.kube.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	pods, err := d.kube.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "job-name=" + jobName})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		if err := d.kube.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func runtimeNamespaceForConfig(cfg config) string {
	namespace := strings.TrimSpace(cfg.runtimeNamespace)
	if namespace == "" {
		return "default"
	}
	return namespace
}

func loadConfig() (config, error) {
	endpoints := splitCSV(os.Getenv("SMITH_ETCD_ENDPOINTS"))
	if len(endpoints) == 0 {
		endpoints = []string{"http://127.0.0.1:2379"}
	}

	etcdDialTimeout, err := envDuration("SMITH_ETCD_DIAL_TIMEOUT", 5*time.Second)
	if err != nil {
		return config{}, err
	}
	cleanupInterval, err := envDuration("SMITH_DAEMON_CLEANUP_INTERVAL", defaultCleanupInterval)
	if err != nil {
		return config{}, err
	}
	cleanupTimeout, err := envDuration("SMITH_DAEMON_CLEANUP_TIMEOUT", defaultCleanupTimeout)
	if err != nil {
		return config{}, err
	}
	retentionFlatline, err := envDuration("SMITH_DAEMON_RETENTION_FLATLINE", defaultRetentionFlatline)
	if err != nil {
		return config{}, err
	}
	retentionCancelled, err := envDuration("SMITH_DAEMON_RETENTION_CANCELLED", defaultRetentionCancelled)
	if err != nil {
		return config{}, err
	}
	retentionSynced, err := envDuration("SMITH_DAEMON_RETENTION_SYNCED", defaultRetentionSynced)
	if err != nil {
		return config{}, err
	}
	port, err := envInt("SMITH_DAEMON_PORT", defaultPort)
	if err != nil {
		return config{}, err
	}
	cleanupMaxDeletes, err := envInt("SMITH_DAEMON_CLEANUP_MAX_DELETES", defaultCleanupMaxDeletes)
	if err != nil {
		return config{}, err
	}
	cleanupDryRun, err := envBool("SMITH_DAEMON_CLEANUP_DRY_RUN", false)
	if err != nil {
		return config{}, err
	}
	policyPath := strings.TrimSpace(os.Getenv("SMITH_DAEMON_POLICY_PATH"))

	cfg := config{
		port:               port,
		etcdEndpoints:      endpoints,
		etcdDialTimeout:    etcdDialTimeout,
		cleanupInterval:    cleanupInterval,
		cleanupTimeout:     cleanupTimeout,
		cleanupDryRun:      cleanupDryRun,
		cleanupMaxDeletes:  cleanupMaxDeletes,
		cleanupActor:       strings.TrimSpace(envString("SMITH_DAEMON_CLEANUP_ACTOR", defaultCleanupActor)),
		policyPath:         policyPath,
		retentionFlatline:  retentionFlatline,
		retentionCancelled: retentionCancelled,
		retentionSynced:    retentionSynced,
		runtimeNamespace: strings.TrimSpace(envString(
			"SMITH_RUNTIME_NAMESPACE",
			envString("SMITH_NAMESPACE", envString("POD_NAMESPACE", "default")),
		)),
	}

	if cfg.cleanupActor == "" {
		cfg.cleanupActor = defaultCleanupActor
	}
	if cfg.cleanupInterval <= 0 {
		return config{}, errors.New("SMITH_DAEMON_CLEANUP_INTERVAL must be > 0")
	}
	if cfg.cleanupTimeout <= 0 {
		return config{}, errors.New("SMITH_DAEMON_CLEANUP_TIMEOUT must be > 0")
	}
	if cfg.cleanupMaxDeletes < 0 {
		return config{}, errors.New("SMITH_DAEMON_CLEANUP_MAX_DELETES must be >= 0")
	}
	if cfg.retentionFlatline < 0 {
		return config{}, errors.New("SMITH_DAEMON_RETENTION_FLATLINE must be >= 0")
	}
	if cfg.retentionCancelled < 0 {
		return config{}, errors.New("SMITH_DAEMON_RETENTION_CANCELLED must be >= 0")
	}
	if cfg.retentionSynced < 0 {
		return config{}, errors.New("SMITH_DAEMON_RETENTION_SYNCED must be >= 0")
	}
	if err := applyRetentionPolicyFromFile(&cfg); err != nil {
		return config{}, err
	}

	return cfg, nil
}

func applyRetentionPolicyFromFile(cfg *config) error {
	path := strings.TrimSpace(cfg.policyPath)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read daemon policy file %q: %w", path, err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil
	}

	var policy cleanupPolicyFile
	if err := yaml.Unmarshal(raw, &policy); err != nil {
		return fmt.Errorf("parse daemon policy file %q: %w", path, err)
	}

	if strings.TrimSpace(policy.Retention.Flatline) != "" {
		value, err := time.ParseDuration(strings.TrimSpace(policy.Retention.Flatline))
		if err != nil {
			return fmt.Errorf("parse retention.flatline duration: %w", err)
		}
		if value < 0 {
			return errors.New("retention.flatline must be >= 0")
		}
		cfg.retentionFlatline = value
	}
	if strings.TrimSpace(policy.Retention.Cancelled) != "" {
		value, err := time.ParseDuration(strings.TrimSpace(policy.Retention.Cancelled))
		if err != nil {
			return fmt.Errorf("parse retention.cancelled duration: %w", err)
		}
		if value < 0 {
			return errors.New("retention.cancelled must be >= 0")
		}
		cfg.retentionCancelled = value
	}
	if strings.TrimSpace(policy.Retention.Synced) != "" {
		value, err := time.ParseDuration(strings.TrimSpace(policy.Retention.Synced))
		if err != nil {
			return fmt.Errorf("parse retention.synced duration: %w", err)
		}
		if value < 0 {
			return errors.New("retention.synced must be >= 0")
		}
		cfg.retentionSynced = value
	}

	return nil
}

func kubeClient() (*kubernetes.Clientset, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return kubernetes.NewForConfig(cfg)
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
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func envString(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return parsed, nil
}

func envInt(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return parsed, nil
}

func envBool(name string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s: %w", name, err)
	}
	return parsed, nil
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
