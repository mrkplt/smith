SHELL := /bin/bash
.DEFAULT_GOAL := help
SMITH_NAMESPACE ?= smith-system
SMITH_RELEASE ?= smith
SMITH_VALUES ?= helm/smith/values/local.yaml
SMITH_LOCAL_VALUES ?= helm/smith/values/local.yaml
SMITH_STAGING_VALUES ?= helm/smith/values/staging.yaml
SMITH_PROD_VALUES ?= helm/smith/values/prod.yaml
SMITH_K3D_CLUSTER_NAME ?= smith-int
SMITH_CORE_IMAGE ?= ghcr.io/smith/core:v0.4.3
SMITH_API_IMAGE ?= ghcr.io/smith/api:v0.4.3
SMITH_REPLICA_IMAGE ?= ghcr.io/smith/replica:v0.4.3
SMITH_CONSOLE_IMAGE ?= ghcr.io/smith/console:v0.4.3
SMITH_DAEMON_IMAGE ?= ghcr.io/smith/daemon:v0.4.3
SMITH_TEST_ARTIFACTS_DIR ?= /tmp/smith-test-artifacts
ACT ?= act
MISE ?= mise
GO ?= $(MISE) exec --no-prepare -- go
NPM ?= $(MISE) exec --no-prepare -- npm
GO_TEST_PACKAGES = $$($(GO) list ./... | grep -Ev '^smith/(frontend/node_modules|site)(/|$$)')
SMITH_FIXTURE_DIR ?= /tmp/smith-test-repo
SMITH_K3D_CLUSTER_NAME ?= smith-int
SMITH_LOCAL_CORE_IMAGE ?= smith-core:local
SMITH_LOCAL_API_IMAGE ?= smith-api:local
SMITH_LOCAL_REPLICA_IMAGE ?= smith-replica:local
SMITH_LOCAL_CONSOLE_IMAGE ?= smith-console:local
SMITH_LOCAL_CHAT_IMAGE ?= smith-chat:local
SMITH_LOCAL_DAEMON_IMAGE ?= smith-daemon:local
SMITH_LOCAL_SKILLS_IMAGE ?= smith-skills:local
SMITH_LOCAL_GIT_PAT ?=
SMITH_LOCAL_RUNTIME_CREDENTIALS ?=
SMITH_LOCAL_RUNTIME_CREDENTIALS_CLAUDE ?=
SMITH_LOCAL_OVERLAY ?=
SMITH_BOOTSTRAP_DOCUMENT_STORAGE ?= false
SMITH_BOOTSTRAP_ENV_FILE ?= $(HOME)/.smith/.env
SMITH_BOOTSTRAP_SCRIPT ?= ./scripts/bootstrap-document-storage.sh
SMITH_FORCE_HELM_ROLLOUT_ID ?= true
SMITH_FORCE_ROLLOUT ?= true
SMITH_MIN_GO_VERSION ?= 1.22.0
SMITH_MIN_KUBECTL_VERSION ?= 1.29.0
SMITH_MIN_HELM_VERSION ?= 3.13.0
SMITH_VCLUSTER_VERSION ?= 0.32.1
BIN_DIR ?= bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

.PHONY: help \
	doctor bootstrap \
	cluster cluster-up cluster-up-local cluster-up-k3d cluster-up-vcluster cluster-down cluster-down-local cluster-down-k3d cluster-down-vcluster cluster-reset cluster-health \
	build build-local image-build-local image-load-local images-local deploy deploy-local deploy-local-document-storage deploy-staging deploy-prod rollout-local undeploy undeploy-local \
	console-build-local console-load-local console-rollout-local console-deploy-local \
	chat-build-local chat-load-local chat-deploy-local \
	daemon-build-local daemon-load-local daemon-rollout-local daemon-deploy-local \
	skills-build-local skills-load-local skills-deploy-local \
	test test-unit test-frontend hook-fast-pre-commit hook-fast-pre-push \
	\
	teardown \
	build docs-check ci-local hooks-install hooks-run-pre-commit hooks-run-pre-push
help: ## Show available make targets
	@awk 'BEGIN {FS = ":.*##"; printf "Smith local developer workflow\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
doctor: ## Validate local prerequisites for make-first workflow
	@set -euo pipefail; \
	missing=0; \
	for cmd in $(MISE) kubectl helm docker; do \
	  if ! command -v $$cmd >/dev/null 2>&1; then \
	    echo "missing required command: $$cmd"; \
	    missing=1; \
	  fi; \
	done; \
	if [[ $$missing -ne 0 ]]; then \
	  echo "doctor failed: install missing prerequisites before continuing"; \
	  exit 1; \
	fi; \
	ver_ge() { \
	  local current="$$1"; local required="$$2"; \
	  [[ "$$(printf '%s\n%s\n' "$$required" "$$current" | sort -V | head -n1)" == "$$required" ]]; \
	}; \
	if ! $(MISE) trust --quiet mise.toml >/dev/null 2>&1; then \
	  echo "doctor failed: mise could not trust repo config"; \
	  echo "remediation: run 'mise trust mise.toml' from the repo root"; \
	  exit 1; \
	fi; \
	if ! $(GO) version >/dev/null 2>&1; then \
	  echo "doctor failed: go is not available through mise"; \
	  echo "remediation: run 'mise install' from the repo root"; \
	  exit 1; \
	fi; \
	go_v="$$( $(GO) version | awk '{print $$3}' | sed 's/^go//')"; \
	if ! ver_ge "$$go_v" "$(SMITH_MIN_GO_VERSION)"; then \
	  echo "doctor failed: go $$go_v is below required $(SMITH_MIN_GO_VERSION)"; \
	  echo "remediation: update the pinned Go version in mise.toml or repair the local mise runtime"; \
	  exit 1; \
	fi; \
	if ! $(NPM) --version >/dev/null 2>&1; then \
	  echo "doctor failed: node/npm is not available through mise"; \
	  echo "remediation: run 'mise install' from the repo root"; \
	  exit 1; \
	fi; \
	kubectl_v="$$(kubectl version --client=true -o yaml 2>/dev/null | awk '/gitVersion:/{print $$2; exit}' | sed 's/^v//')"; \
	if [[ -n "$$kubectl_v" ]] && ! ver_ge "$$kubectl_v" "$(SMITH_MIN_KUBECTL_VERSION)"; then \
	  echo "doctor failed: kubectl $$kubectl_v is below required $(SMITH_MIN_KUBECTL_VERSION)"; \
	  echo "remediation: upgrade kubectl via package manager or Kubernetes release binaries"; \
	  exit 1; \
	fi; \
	helm_v="$$(helm version --short 2>/dev/null | sed -E 's/^v([0-9]+\.[0-9]+\.[0-9]+).*/\\1/')"; \
	if [[ -n "$$helm_v" ]] && ! ver_ge "$$helm_v" "$(SMITH_MIN_HELM_VERSION)"; then \
	  echo "doctor failed: helm $$helm_v is below required $(SMITH_MIN_HELM_VERSION)"; \
	  echo "remediation: upgrade helm via package manager or https://helm.sh/docs/intro/install/"; \
	  exit 1; \
	fi; \
	echo "doctor passed: required local tools found"
bootstrap: ## Install required mise runtimes and optional k3d/vcluster prerequisites
	@set -euo pipefail; \
	$(MISE) trust mise.toml; \
	$(MISE) install; \
	SMITH_VCLUSTER_VERSION="$(SMITH_VCLUSTER_VERSION)" ./scripts/integration/prereqs.sh; \
	mkdir -p "$$HOME/.smith"; \
	if [[ ! -f "$$HOME/.smith/config.json" ]]; then \
	  printf '%s\n' '{"current_context":"default","contexts":{"default":{"server":"http://127.0.0.1:8080","token":""}}}' > "$$HOME/.smith/config.json"; \
	  echo "bootstrap: created $$HOME/.smith/config.json"; \
	else \
	  echo "bootstrap: preserved existing $$HOME/.smith/config.json"; \
	fi

build: build-smithctl build-services ## Build all binaries

build-smithctl: ## Build smithctl binary for current platform
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT)" -o $(BIN_DIR)/smithctl ./cmd/smithctl

build-services: ## Build all service binaries
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/smith-api ./cmd/smith-api
	$(GO) build -o $(BIN_DIR)/smith-core ./cmd/smith-core
	$(GO) build -o $(BIN_DIR)/smith-daemon ./cmd/smith-daemon
	$(GO) build -o $(BIN_DIR)/smith-replica ./cmd/smith-replica
	$(GO) build -o $(BIN_DIR)/smith ./cmd/smith
	$(GO) build -o $(BIN_DIR)/task ./cmd/task

dist: ## Build cross-platform smithctl binaries
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 $(GO) build -o dist/smithctl-linux-amd64 ./cmd/smithctl
	GOOS=linux GOARCH=arm64 $(GO) build -o dist/smithctl-linux-arm64 ./cmd/smithctl
	GOOS=darwin GOARCH=amd64 $(GO) build -o dist/smithctl-darwin-amd64 ./cmd/smithctl
	GOOS=darwin GOARCH=arm64 $(GO) build -o dist/smithctl-darwin-arm64 ./cmd/smithctl
	GOOS=windows GOARCH=amd64 $(GO) build -o dist/smithctl-windows-amd64.exe ./cmd/smithctl

cluster: cluster-up ## Alias for cluster-up

cluster-up: cluster-up-local ## Alias for the default local cluster provider
cluster-up-local: ## Provision etcd on the current kubectl context
	SMITH_CLUSTER_PROVIDER=current SMITH_USE_VCLUSTER=false ./scripts/integration/env-up.sh
cluster-up-k3d: ## Provision local k3d + etcd environment
	SMITH_CLUSTER_PROVIDER=k3d SMITH_USE_VCLUSTER=false ./scripts/integration/env-up.sh
cluster-up-vcluster: ## Provision local k3d + vcluster + etcd environment
	SMITH_CLUSTER_PROVIDER=k3d SMITH_USE_VCLUSTER=true ./scripts/integration/env-up.sh
cluster-down: cluster-down-local ## Alias for the default local cluster provider
cluster-down-local: ## Remove etcd from the current kubectl context
	SMITH_CLUSTER_PROVIDER=current SMITH_USE_VCLUSTER=false ./scripts/integration/env-down.sh
cluster-down-k3d: ## Delete local k3d + etcd environment
	SMITH_CLUSTER_PROVIDER=k3d SMITH_USE_VCLUSTER=false ./scripts/integration/env-down.sh
cluster-down-vcluster: ## Delete local k3d + vcluster + etcd environment
	SMITH_CLUSTER_PROVIDER=k3d SMITH_USE_VCLUSTER=true ./scripts/integration/env-down.sh
cluster-reset: ## Reset the default local environment (down then up)
	@set -euo pipefail; \
	echo "[cluster-reset] tearing down existing local environment"; \
	$(MAKE) --no-print-directory cluster-down; \
	echo "[cluster-reset] bringing local environment back up"; \
	$(MAKE) --no-print-directory cluster-up; \
	echo "[cluster-reset] completed"
cluster-health: ## Verify current cluster API, node readiness, and etcd readiness
	@set -euo pipefail; \
	echo "[cluster-health] checking Kubernetes API reachability"; \
	if ! kubectl cluster-info >/dev/null 2>&1; then \
	  echo "[cluster-health] ERROR: kubectl cannot reach a cluster context"; \
	  echo "[cluster-health] HINT: run 'make cluster-up' or fix KUBECONFIG"; \
	  exit 1; \
	fi; \
	echo "[cluster-health] checking node readiness"; \
	if ! kubectl wait --for=condition=Ready nodes --all --timeout=120s >/dev/null 2>&1; then \
	  echo "[cluster-health] ERROR: one or more cluster nodes are not Ready"; \
	  echo "[cluster-health] HINT: run 'kubectl get nodes -o wide' and inspect node events"; \
	  exit 1; \
	fi; \
	ETCD_NS="$${SMITH_ETCD_NAMESPACE:-smith-system}"; \
	ETCD_RELEASE="$${SMITH_ETCD_RELEASE_NAME:-smith-etcd}"; \
	ETCD_MODE="$${SMITH_ETCD_MODE:-simple}"; \
	echo "[cluster-health] checking etcd service in namespace $${ETCD_NS}"; \
	if ! kubectl -n "$${ETCD_NS}" get svc "$${ETCD_RELEASE}" >/dev/null 2>&1; then \
	  echo "[cluster-health] ERROR: etcd service '$${ETCD_RELEASE}' not found in namespace '$${ETCD_NS}'"; \
	  echo "[cluster-health] HINT: run 'make cluster-up' and review env-up output"; \
	  exit 1; \
	fi; \
	if [[ "$${ETCD_MODE}" == "helm" ]]; then \
	  echo "[cluster-health] checking etcd statefulset readiness"; \
	  if ! kubectl -n "$${ETCD_NS}" rollout status statefulset/"$${ETCD_RELEASE}" --timeout=120s >/dev/null 2>&1; then \
	    echo "[cluster-health] ERROR: etcd statefulset '$${ETCD_RELEASE}' is not ready"; \
	    echo "[cluster-health] HINT: run 'kubectl -n $${ETCD_NS} get pods -o wide'"; \
	    exit 1; \
	  fi; \
	else \
	  echo "[cluster-health] checking etcd deployment readiness"; \
	  if ! kubectl -n "$${ETCD_NS}" rollout status deployment/"$${ETCD_RELEASE}" --timeout=120s >/dev/null 2>&1; then \
	    echo "[cluster-health] ERROR: etcd deployment '$${ETCD_RELEASE}' is not ready"; \
	    echo "[cluster-health] HINT: run 'kubectl -n $${ETCD_NS} get pods -o wide'"; \
	    exit 1; \
	  fi; \
	fi; \
	echo "[cluster-health] ready"
deploy: ## Deploy Smith with Helm using SMITH_VALUES profile
	helm upgrade --install "$(SMITH_RELEASE)" ./helm/smith \
	  --namespace "$(SMITH_NAMESPACE)" \
	  --create-namespace \
	  -f "$(SMITH_VALUES)"
deploy-local: ## Deploy Smith via Helm using local values profile
	@if [[ -z "$(SMITH_LOCAL_GIT_PAT)" ]]; then \
	  echo "deploy-local: SMITH_LOCAL_GIT_PAT is required"; \
	  exit 1; \
	fi
	@if [[ -z "$(SMITH_LOCAL_RUNTIME_CREDENTIALS)" ]]; then \
	  echo "deploy-local: SMITH_LOCAL_RUNTIME_CREDENTIALS is required"; \
	  exit 1; \
	fi
	@if [[ "$(SMITH_BOOTSTRAP_DOCUMENT_STORAGE)" == "true" ]]; then \
	  if [[ ! -f "$(SMITH_BOOTSTRAP_ENV_FILE)" ]]; then \
	    echo "deploy-local: expected env file not found for document storage bootstrap: $(SMITH_BOOTSTRAP_ENV_FILE)"; \
	    exit 1; \
	  fi; \
	  if [[ ! -x "$(SMITH_BOOTSTRAP_SCRIPT)" ]]; then \
	    echo "deploy-local: expected bootstrap script is not executable: $(SMITH_BOOTSTRAP_SCRIPT)"; \
	    exit 1; \
	  fi; \
	  echo "deploy-local: pre-deploy document secret bootstrap"; \
	  "$(SMITH_BOOTSTRAP_SCRIPT)" --env-file "$(SMITH_BOOTSTRAP_ENV_FILE)" --namespace "$(SMITH_NAMESPACE)" --release "$(SMITH_RELEASE)" --skip-garage; \
	fi
	$(MAKE) --no-print-directory images-local
	@helm upgrade --install "$(SMITH_RELEASE)" ./helm/smith \
	  --namespace "$(SMITH_NAMESPACE)" \
	  --create-namespace \
	  --set-string secrets.managed.gitPat="$(SMITH_LOCAL_GIT_PAT)" \
	  --set-string secrets.managed.runtimeCredentials="$(SMITH_LOCAL_RUNTIME_CREDENTIALS)" \
	  --set-string secrets.managed.runtimeCredentialsClaude="$(SMITH_LOCAL_RUNTIME_CREDENTIALS_CLAUDE)" \
	  $(if $(filter true,$(SMITH_FORCE_HELM_ROLLOUT_ID)),--set global.rolloutId="$(shell date +%s)",) \
	  -f "$(SMITH_LOCAL_VALUES)" \
	  $(if $(strip $(SMITH_LOCAL_OVERLAY)),-f "$(SMITH_LOCAL_OVERLAY)",)
	@if [[ "$(SMITH_FORCE_ROLLOUT)" == "true" ]]; then \
	  $(MAKE) --no-print-directory rollout-local; \
	else \
	  echo "deploy-local: skipping forced rollout (SMITH_FORCE_ROLLOUT=false)"; \
	fi
	@if [[ "$(SMITH_BOOTSTRAP_DOCUMENT_STORAGE)" == "true" ]]; then \
	  echo "deploy-local: post-deploy garage bootstrap"; \
	  "$(SMITH_BOOTSTRAP_SCRIPT)" --env-file "$(SMITH_BOOTSTRAP_ENV_FILE)" --namespace "$(SMITH_NAMESPACE)" --release "$(SMITH_RELEASE)" --skip-secrets; \
	fi
deploy-local-document-storage: ## Deploy local with in-cluster Postgres+Garage bootstrap order
	SMITH_BOOTSTRAP_DOCUMENT_STORAGE=true SMITH_FORCE_HELM_ROLLOUT_ID=false SMITH_FORCE_ROLLOUT=false SMITH_LOCAL_OVERLAY=helm/smith/values/local-document-storage-1password.yaml $(MAKE) --no-print-directory deploy-local
console-build-local: ## Build only the console local image
	docker build -f docker/console.Dockerfile -t "$(SMITH_LOCAL_CONSOLE_IMAGE)" .
console-load-local: ## Load only the console image when using k3d
	@if [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "k3d" ]] || { [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "auto" ]] && [[ "$$(kubectl config current-context 2>/dev/null)" == k3d-* ]]; }; then \
	  k3d image import "$(SMITH_LOCAL_CONSOLE_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"; \
	else \
	  echo "console-load-local: skipping image import for current-cluster provider"; \
	fi
console-rollout-local: ## Restart only the console deployment
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-console -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-console -n $(SMITH_NAMESPACE)
console-deploy-local: console-build-local console-load-local console-rollout-local ## Build, load, and restart only the console
api-build-local: ## Build only the smith-api local image
	docker build -f docker/api.Dockerfile -t "$(SMITH_LOCAL_API_IMAGE)" .
api-load-local: ## Load only the smith-api image when using k3d
	@if [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "k3d" ]] || { [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "auto" ]] && [[ "$$(kubectl config current-context 2>/dev/null)" == k3d-* ]]; }; then \
	  k3d image import "$(SMITH_LOCAL_API_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"; \
	else \
	  echo "api-load-local: skipping image import for current-cluster provider"; \
	fi
api-rollout-local: ## Restart only the smith-api deployment
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-api -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-api -n $(SMITH_NAMESPACE)
api-deploy-local: api-build-local api-load-local api-rollout-local ## Build, load, and restart only the smith-api
replica-build-local: ## Build only the smith-replica local image
	docker build -f docker/replica.Dockerfile -t "$(SMITH_LOCAL_REPLICA_IMAGE)" .
replica-load-local: ## Load only the smith-replica image when using k3d
	@if [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "k3d" ]] || { [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "auto" ]] && [[ "$$(kubectl config current-context 2>/dev/null)" == k3d-* ]]; }; then \
	  k3d image import "$(SMITH_LOCAL_REPLICA_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"; \
	else \
	  echo "replica-load-local: skipping image import for current-cluster provider"; \
	fi
replica-deploy-local: replica-build-local replica-load-local ## Build and load only the smith-replica (no rollout needed as it runs as Jobs)
chat-build-local: ## Build only the smith-chat local image
	docker build -f docker/chat.Dockerfile -t "$(SMITH_LOCAL_CHAT_IMAGE)" .
chat-load-local: ## Load only the smith-chat image when using k3d
	@if [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "k3d" ]] || { [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "auto" ]] && [[ "$$(kubectl config current-context 2>/dev/null)" == k3d-* ]]; }; then \
	  k3d image import "$(SMITH_LOCAL_CHAT_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"; \
	else \
	  echo "chat-load-local: skipping image import for current-cluster provider"; \
	fi
chat-deploy-local: chat-build-local chat-load-local ## Build and load only the smith-chat
daemon-build-local: ## Build only the smith-daemon local image
	docker build -f docker/daemon.Dockerfile -t "$(SMITH_LOCAL_DAEMON_IMAGE)" .
daemon-load-local: ## Load only the smith-daemon image when using k3d
	@if [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "k3d" ]] || { [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "auto" ]] && [[ "$$(kubectl config current-context 2>/dev/null)" == k3d-* ]]; }; then \
	  k3d image import "$(SMITH_LOCAL_DAEMON_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"; \
	else \
	  echo "daemon-load-local: skipping image import for current-cluster provider"; \
	fi
daemon-rollout-local: ## Restart only the smith-daemon deployment
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-daemon -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-daemon -n $(SMITH_NAMESPACE)
daemon-deploy-local: daemon-build-local daemon-load-local daemon-rollout-local ## Build, load, and restart only the smith-daemon
skills-build-local: ## Build only the smith-skills local image
	docker build -f docker/skills.Dockerfile -t "$(SMITH_LOCAL_SKILLS_IMAGE)" .
skills-load-local: ## Load only the smith-skills image when using k3d
	@if [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "k3d" ]] || { [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "auto" ]] && [[ "$$(kubectl config current-context 2>/dev/null)" == k3d-* ]]; }; then \
	  k3d image import "$(SMITH_LOCAL_SKILLS_IMAGE)" -c "$(SMITH_K3D_CLUSTER_NAME)"; \
	else \
	  echo "skills-load-local: skipping image import for current-cluster provider"; \
	fi
skills-deploy-local: skills-build-local skills-load-local ## Build and load only the smith-skills image
console-api-deploy-local: console-build-local console-load-local api-build-local api-load-local rollout-local ## Build, load, and restart console + api
rollout-local: ## Force restart local deployments to pick up new images
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-api -n $(SMITH_NAMESPACE)
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-console -n $(SMITH_NAMESPACE)
	kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-core -n $(SMITH_NAMESPACE)
	-kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-daemon -n $(SMITH_NAMESPACE)
	-kubectl rollout restart deployment/$(SMITH_RELEASE)-smith-chat -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-api -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-console -n $(SMITH_NAMESPACE)
	kubectl rollout status deployment/$(SMITH_RELEASE)-smith-core -n $(SMITH_NAMESPACE)
	-kubectl rollout status deployment/$(SMITH_RELEASE)-smith-daemon -n $(SMITH_NAMESPACE)
deploy-staging: ## Deploy Smith via Helm using staging values profile
	helm upgrade --install "$(SMITH_RELEASE)" ./helm/smith \
	  --namespace "$(SMITH_NAMESPACE)" \
	  --create-namespace \
	  -f "$(SMITH_STAGING_VALUES)"
deploy-prod: ## Deploy Smith via Helm using production values profile
	helm upgrade --install "$(SMITH_RELEASE)" ./helm/smith \
	  --namespace "$(SMITH_NAMESPACE)" \
	  --create-namespace \
	  -f "$(SMITH_PROD_VALUES)"
undeploy: ## Remove Helm release from cluster
	-helm uninstall "$(SMITH_RELEASE)" -n "$(SMITH_NAMESPACE)"
undeploy-local: ## Remove local Helm deployment
	-helm uninstall "$(SMITH_RELEASE)" -n "$(SMITH_NAMESPACE)"

test: test-unit test-frontend test-acceptance ## Run default local test workflow (non-cluster matrix)

test-unit: ## Run full Go test suite
	$(GO) test $(GO_TEST_PACKAGES)

test-bdd: ## Run godog-based BDD acceptance suite
	$(GO) test ./test/acceptance -run TestFeatures -count=1

test-acceptance-smoke: ## Run acceptance smoke suite with JSON artifact output
	@set -euo pipefail; \
	mkdir -p "$(SMITH_TEST_ARTIFACTS_DIR)"; \
	$(GO) test ./test/acceptance -run TestHarnessSmoke -count=1 -json | tee "$(SMITH_TEST_ARTIFACTS_DIR)/acceptance-smoke.jsonl"

test-acceptance-bdd: ## Run acceptance BDD suite with JSON artifact output
	@set -euo pipefail; \
	mkdir -p "$(SMITH_TEST_ARTIFACTS_DIR)"; \
	$(GO) test ./test/acceptance -run TestFeatures -count=1 -json | tee "$(SMITH_TEST_ARTIFACTS_DIR)/acceptance-bdd.jsonl"

test-acceptance: test-acceptance-smoke test-acceptance-bdd ## Run all Go-native acceptance harness suites

test-observability-latency: ## Measure journal-to-console propagation latency (requires running API and loop)
	./scripts/integration/measure-observability-latency.sh

test-frontend: ## Run Playwright frontend/component tests for console
	@if [ ! -d test/playwright/node_modules ]; then $(NPM) --prefix test/playwright install; fi
	@if [ ! -d frontend/node_modules ]; then $(NPM) --prefix frontend install; fi
	@if [ ! -d frontend/.svelte-kit ]; then $(NPM) --prefix frontend exec svelte-kit sync; fi
	$(NPM) --prefix frontend run build
	$(NPM) --prefix test/playwright run test:frontend

trivy-scan-local: ## Run local vulnerability scans on all Smith images
	@echo "[trivy] scanning images for critical vulnerabilities..."
	@vulnerabilities=0; \
	for img in core api replica console chat daemon; do \
	  echo "Scanning smith-$$img:local..."; \
	  if ! trivy image --severity CRITICAL --exit-code 1 "smith-$$img:local" > /tmp/trivy-$$img.log 2>&1; then \
	    cat /tmp/trivy-$$img.log; \
	    vulnerabilities=$$((vulnerabilities + 1)); \
	  else \
	    cat /tmp/trivy-$$img.log; \
	  fi; \
	done; \
	if [ $$vulnerabilities -gt 0 ]; then \
	  echo "[trivy] Found CRITICAL vulnerabilities in $$vulnerabilities images. Fails build."; \
	  exit 1; \
	fi

image-build-local: ## Build local Smith container images with deploy-local tags
	docker build -f docker/core.Dockerfile -t "$(SMITH_LOCAL_CORE_IMAGE)" .
	docker build -f docker/api.Dockerfile -t "$(SMITH_LOCAL_API_IMAGE)" .
	docker build -f docker/replica.Dockerfile -t "$(SMITH_LOCAL_REPLICA_IMAGE)" .
	docker build -f docker/console.Dockerfile -t "$(SMITH_LOCAL_CONSOLE_IMAGE)" .
	docker build -f docker/chat.Dockerfile -t "$(SMITH_LOCAL_CHAT_IMAGE)" .
	docker build -f docker/daemon.Dockerfile -t "$(SMITH_LOCAL_DAEMON_IMAGE)" .
	docker build -f docker/skills.Dockerfile -t "$(SMITH_LOCAL_SKILLS_IMAGE)" .

build-local: image-build-local ## Backward-compatible alias for local image builds

image-load-local: ## Import local Smith container images when using k3d
	@if [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "k3d" ]] || { [[ "$${SMITH_CLUSTER_PROVIDER:-auto}" == "auto" ]] && [[ "$$(kubectl config current-context 2>/dev/null)" == k3d-* ]]; }; then \
	  k3d image import -c "$(SMITH_K3D_CLUSTER_NAME)" \
	    "$(SMITH_LOCAL_CORE_IMAGE)" \
	    "$(SMITH_LOCAL_API_IMAGE)" \
	    "$(SMITH_LOCAL_REPLICA_IMAGE)" \
	    "$(SMITH_LOCAL_CONSOLE_IMAGE)" \
	    "$(SMITH_LOCAL_CHAT_IMAGE)" \
	    "$(SMITH_LOCAL_DAEMON_IMAGE)" \
	    "$(SMITH_LOCAL_SKILLS_IMAGE)"; \
	else \
	  echo "image-load-local: skipping image import for current-cluster provider"; \
	fi

images-local: image-build-local image-load-local ## Build local Smith images and import them when using k3d
docs-image-build: ## Build the containerized docs tool image
	docker build -f docker/docs.Dockerfile -t "smith-docs:local" .
docs-check: ## Run docs quality checks
	./scripts/docs/quality-check.sh

ci-local: ## Run local CI-equivalent checks
	$(MAKE) build
	$(MAKE) test-unit
	$(MAKE) test-acceptance
	$(MAKE) docs-check

ci-local-act: ## Run core CI jobs locally using 'act' (requires 'act' and Docker)
	@if ! command -v "$(ACT)" >/dev/null 2>&1; then \
		echo "ERROR: 'act' is not installed."; \
		echo "HINT: install it with 'brew install act' or from https://github.com/nektos/act"; \
		exit 1; \
	fi
	@echo "[act] running CI jobs..."
	$(ACT) -j lint-and-check
	$(ACT) -j go-unit-tests
	$(ACT) -j node-unit-tests
	$(ACT) -j playwright-tests
	$(ACT) -j acceptance-tests
	$(ACT) -j test-matrix

hook-fast-pre-commit: ## Run fast local checks for pre-commit
	$(MAKE) docs-check
	$(GO) vet ./...
	$(NPM) --prefix frontend run lint
	$(NPM) --prefix frontend run check
	$(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...

hook-fast-pre-push: ## Run fast local checks for pre-push
	$(MAKE) test-unit
	$(NPM) --prefix frontend run test:unit

hooks-run-pre-commit: hook-fast-pre-commit ## Backward-compatible alias for pre-commit hook workload

hooks-run-pre-push: hook-fast-pre-push ## Backward-compatible alias for pre-push hook workload

hooks-install: ## Install repository git hooks from .githooks
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit .githooks/pre-push
	@echo "Installed git hooks from .githooks"
