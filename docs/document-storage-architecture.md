# Document Storage Architecture

This document describes the current storage architecture for `Document` records after the Postgres + Garage migration work.

## Storage Model

- `smith-api` remains the write/read API boundary for documents.
- Document metadata is stored in PostgreSQL table `smith_documents` when `SMITH_DOCUMENT_STORE_BACKEND=postgres-garage`.
- Document content blobs are stored in Garage (S3-compatible object storage).
- etcd remains authoritative for loop state, journal, locks, overrides, and audit records.
- etcd document storage is still supported as a compatibility backend (`SMITH_DOCUMENT_STORE_BACKEND=etcd`) and as migration fallback when configured.

## Component Updates

- `cmd/smith-api`
  - Document handlers (`/v1/documents`, `/v1/documents/stream`, `/v1/documents/{id}`) now use the document-store abstraction rather than directly calling etcd document methods.
  - Startup now supports migration orchestration (startup backfill + optional dual-write/read-through fallback behavior).
- `internal/source/docstore`
  - Added `PostgresGarageStore` for metadata/content split persistence.
  - Added `MigratingStore` wrapper for fallback merge, read-through, and dual-write behavior.
- `cmd/smith-chat` + `internal/chat/smithbridge`
  - Chat document context fetch now resolves document payloads via API (`/v1/documents/{id}`) instead of direct etcd document reads.

## ERD

```mermaid
erDiagram
    smith_documents {
        text id PK
        text project_id
        text title
        text format
        text source_type
        text source_ref
        text status
        jsonb metadata
        text content_key
        bigint content_size
        text content_sha256
        text correlation_id
        text schema_version
        timestamptz created_at
        timestamptz updated_at
    }

    garage_object {
        text key PK
        blob content
        text etag
        datetime last_modified
    }

    smith_documents ||--|| garage_object : "content_key"
```

## Write Sequence (Create / Update)

```mermaid
sequenceDiagram
    participant UI as Console/UI
    participant API as smith-api
    participant PG as Postgres
    participant G as Garage (S3)
    participant ETCD as etcd (optional dual-write)

    UI->>API: POST/PUT /v1/documents
    API->>G: PutObject(content_key, content)
    G-->>API: 200 OK
    API->>PG: UPSERT smith_documents(metadata + content_key)
    PG-->>API: 200 OK
    opt migration dual-write enabled
      API->>ETCD: PutDocument(doc)
      ETCD-->>API: OK
    end
    API-->>UI: Document response
```

## Read Sequence (Read-Through)

```mermaid
sequenceDiagram
    participant API as smith-api
    participant PG as Postgres
    participant G as Garage (S3)
    participant ETCD as etcd (fallback)

    API->>PG: SELECT smith_documents by id
    alt found in Postgres
      PG-->>API: metadata + content_key
      API->>G: GetObject(content_key)
      G-->>API: content
      API-->>API: return assembled Document
    else missing in Postgres
      API->>ETCD: GetDocument(id)
      ETCD-->>API: legacy doc
      opt read-through enabled
        API->>G: PutObject(content_key, content)
        API->>PG: UPSERT migrated metadata
      end
      API-->>API: return Document
    end
```

## Startup Backfill Sequence

```mermaid
sequenceDiagram
    participant API as smith-api startup
    participant ETCD as etcd
    participant PG as Postgres
    participant G as Garage (S3)

    API->>ETCD: ListDocuments()
    ETCD-->>API: legacy documents
    API->>PG: ListDocuments()
    PG-->>API: current migrated set
    loop each newer/missing legacy doc
      API->>G: PutObject(content)
      API->>PG: UPSERT metadata + content_key
    end
    API-->>API: Backfill summary logged
```

## Configuration

Primary knobs:

- `SMITH_DOCUMENT_STORE_BACKEND` = `etcd` or `postgres-garage`
- `SMITH_DOCUMENTS_POSTGRES_DSN`
- `SMITH_DOCUMENTS_GARAGE_ENDPOINT`
- `SMITH_DOCUMENTS_GARAGE_BUCKET`
- `SMITH_DOCUMENTS_GARAGE_ACCESS_KEY_ID`
- `SMITH_DOCUMENTS_GARAGE_SECRET_ACCESS_KEY`

Migration knobs:

- `SMITH_DOCUMENTS_MIGRATION_BACKFILL_ON_STARTUP`
- `SMITH_DOCUMENTS_MIGRATION_READ_THROUGH_ON_MISS`
- `SMITH_DOCUMENTS_MIGRATION_MERGE_LIST_FALLBACK`
- `SMITH_DOCUMENTS_MIGRATION_DUAL_WRITE_ETCD`

Helm values are under `api.documents.*`.

When in-chart dependencies are enabled, additional values live under `documentDependencies.*`.

Current Helm chart behavior:

- The chart wires API environment variables for Postgres + Garage connectivity.
- The chart can optionally provision single-node Postgres and Garage (`documentDependencies.postgres.enabled` and `documentDependencies.garage.enabled`).
- Credentials can be sourced from a Kubernetes Secret (`documentDependencies.credentials.*`) instead of plaintext values.
- Garage bootstrap can be automated via optional hook Job (`documentDependencies.garage.bootstrap.enabled`) that applies layout and provisions key/bucket permissions.
