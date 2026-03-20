package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"smith/internal/chat/goosed"
	"smith/internal/chat/httpapi"
	"smith/internal/chat/prompts"
	"smith/internal/chat/sessions"
	"smith/internal/chat/smithbridge"
	"smith/internal/source/store"
	api "smith/pkg/api/v1"
)

type config struct {
	port          int
	etcdEndpoints []string
	apiURL        string
}

func main() {
	var (
		port          = flag.Int("port", 8081, "Listen port")
		etcdEndpoints = flag.String("etcd-endpoints", "http://127.0.0.1:2379", "etcd endpoints")
		apiURL        = flag.String("api-url", "http://localhost:8080", "smith-api base URL for commit actions")
	)
	flag.Parse()

	cfg := config{
		port:          *port,
		etcdEndpoints: strings.Split(*etcdEndpoints, ","),
		apiURL:        *apiURL,
	}

	etcdStore, err := store.New(context.Background(), cfg.etcdEndpoints, 5*time.Second)
	if err != nil {
		log.Fatalf("failed to connect to etcd: %v", err)
	}
	defer func() { _ = etcdStore.Close() }()

	documentFetcher := smithbridge.NewAPIDocumentFetcher(cfg.apiURL, nil)
	bridge := smithbridge.NewEtcdBridgeWithDocumentFetcher(etcdStore, documentFetcher)
	chatHandler := httpapi.NewServerWithCommit(
		goosed.NewEngine(),
		sessions.NewManager(),
		prompts.NewManager(bridge),
		newCommitHandler(cfg.apiURL),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.Handle("/v1/chat/", chatHandler)
	mux.HandleFunc("/v1/chat/ui/resolve", handleUIResolve)

	handler := corsMiddleware(mux)
	addr := fmt.Sprintf(":%d", cfg.port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("smith-chat listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("smith-chat failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("smith-chat shutdown requested")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("smith-chat shutdown failed: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newCommitHandler(apiURL string) func(*http.Request, api.ChatCommitActionRequest) (api.ChatCommitActionResponse, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	client := &http.Client{Timeout: 10 * time.Second}

	return func(r *http.Request, req api.ChatCommitActionRequest) (api.ChatCommitActionResponse, error) {
		switch strings.TrimSpace(req.Action) {
		case "create-loop":
			payload, err := json.Marshal(req.Payload)
			if err != nil {
				return api.ChatCommitActionResponse{}, &httpapi.HTTPError{Code: http.StatusBadRequest, Message: fmt.Sprintf("marshal create-loop payload: %v", err)}
			}

			httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, baseURL+"/v1/loops", bytes.NewReader(payload))
			if err != nil {
				return api.ChatCommitActionResponse{}, &httpapi.HTTPError{Code: http.StatusBadRequest, Message: fmt.Sprintf("create loop request: %v", err)}
			}
			httpReq.Header.Set("Content-Type", "application/json")
			if auth := r.Header.Get("Authorization"); auth != "" {
				httpReq.Header.Set("Authorization", auth)
			}

			resp, err := client.Do(httpReq)
			if err != nil {
				status := http.StatusBadGateway
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					status = http.StatusGatewayTimeout
				}
				return api.ChatCommitActionResponse{}, &httpapi.HTTPError{Code: status, Message: fmt.Sprintf("execute loop create request: %v", err)}
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				message := strings.TrimSpace(string(raw))
				if message == "" {
					message = http.StatusText(resp.StatusCode)
				}
				return api.ChatCommitActionResponse{}, &httpapi.HTTPError{Code: resp.StatusCode, Message: message}
			}

			return api.ChatCommitActionResponse{Status: "accepted"}, nil
		default:
			return api.ChatCommitActionResponse{}, &httpapi.HTTPError{Code: http.StatusBadRequest, Message: fmt.Sprintf("unsupported action: %s", req.Action)}
		}
	}
}

func handleUIResolve(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
