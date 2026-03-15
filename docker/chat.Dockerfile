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
    go build -trimpath -ldflags "-s -w -buildid=" -o /out/smith-chat ./cmd/smith-chat

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

FROM debian:bookworm-slim
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates libgomp1 && \
    rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/smith-chat /bin/smith-chat
COPY --from=goose-downloader /out/goose /usr/local/bin/goose
ENTRYPOINT ["/bin/smith-chat"]
