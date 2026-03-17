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
    go build -trimpath -ldflags "-s -w -buildid=" -o /out/smith-replica ./cmd/smith-replica && \
    go build -trimpath -ldflags "-s -w -buildid=" -o /out/smith ./cmd/smith

FROM debian:bookworm-slim AS codex-downloader
ARG TARGETARCH
ARG CODEX_VERSION=rust-v0.114.0
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

FROM debian:bookworm-slim AS goose-downloader
ARG TARGETARCH
ARG GOOSE_VERSION=stable
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl bzip2 tar && \
    rm -rf /var/lib/apt/lists/*
RUN mkdir -p /out
RUN case "${TARGETARCH}" in \
      amd64) goose_arch="x86_64" ;; \
      arm64) goose_arch="aarch64" ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac && \
    curl -fsSL -o /tmp/goose.tar.bz2 "https://github.com/block/goose/releases/download/${GOOSE_VERSION}/goose-${goose_arch}-unknown-linux-gnu.tar.bz2" && \
    tar -xjf /tmp/goose.tar.bz2 -C /tmp && \
    install -m 0755 /tmp/goose /out/goose

FROM node:22-trixie-slim
RUN apt-get update && \
    apt-get install -y --no-install-recommends git bash ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/smith-replica /bin/smith-replica
COPY --from=builder /out/smith /bin/smith
COPY --from=codex-downloader /out/codex /usr/local/bin/codex
COPY --from=goose-downloader /out/goose /usr/local/bin/goose
WORKDIR /workspace
RUN mkdir -p /workspace && chown -R node:node /workspace
USER node
ENTRYPOINT ["/bin/smith-replica"]
