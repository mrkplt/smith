# syntax=docker/dockerfile:1.7

FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -buildid=" -o /out/smith ./cmd/smith && \
    go build -trimpath -ldflags "-s -w -buildid=" -o /out/task ./cmd/task

FROM debian:bookworm-slim AS codex-downloader
ARG TARGETARCH
ARG CODEX_VERSION=rust-v0.116.0
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl tar && \
    rm -rf /var/lib/apt/lists/*
RUN mkdir -p /out
RUN case "${TARGETARCH}" in \
      amd64) codex_arch="x86_64" ;; \
      arm64) codex_arch="aarch64" ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac && \
    curl -fsSL -o /tmp/codex.tar.gz "https://github.com/openai/codex/releases/download/${CODEX_VERSION}/codex-${codex_arch}-unknown-linux-gnu.tar.gz" && \
    tar -xzf /tmp/codex.tar.gz -C /tmp && \
    codex_bin="$(find /tmp -maxdepth 2 -type f \( -name codex -o -name "codex-${codex_arch}-unknown-linux-gnu" \) | head -n1)" && \
    test -n "${codex_bin}" && \
    install -m 0755 "${codex_bin}" /out/codex

FROM node:22-trixie-slim
ARG CLAUDE_CODE_VERSION=2.1.81
RUN apt-get update && \
    apt-get install -y --no-install-recommends git bash ca-certificates curl gh && \
    rm -rf /var/lib/apt/lists/*
RUN npm install -g "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}"
RUN mkdir -p /home/node/.codex /home/node/.claude
COPY --from=builder /out/smith /bin/smith
COPY --from=builder /out/task /usr/local/bin/task
COPY --from=codex-downloader /out/codex /usr/local/bin/codex
COPY docker/replica.AGENTS.md /home/node/.codex/AGENTS.md
COPY docker/replica.AGENTS.md /home/node/.claude/CLAUDE.md
WORKDIR /workspace
RUN mkdir -p /workspace && chown -R node:node /workspace /home/node/.codex /home/node/.claude
USER node
ENTRYPOINT ["/bin/smith", "replica", "run"]
