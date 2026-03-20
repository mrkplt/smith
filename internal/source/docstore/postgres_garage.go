package docstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"smith/internal/source/model"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresGarageConfig struct {
	PostgresDSN           string
	PostgresMaxConns      int32
	GarageEndpoint        string
	GarageRegion          string
	GarageBucket          string
	GarageAccessKeyID     string
	GarageSecretAccessKey string
	GarageForcePathStyle  bool
	WatchPollInterval     time.Duration
}

type PostgresGarageStore struct {
	pool              *pgxpool.Pool
	s3Client          *s3.Client
	bucket            string
	watchPollInterval time.Duration
}

func NewPostgresGarageStore(ctx context.Context, cfg PostgresGarageConfig) (*PostgresGarageStore, error) {
	if strings.TrimSpace(cfg.PostgresDSN) == "" {
		return nil, errors.New("documents postgres dsn is required")
	}
	if strings.TrimSpace(cfg.GarageEndpoint) == "" {
		return nil, errors.New("documents garage endpoint is required")
	}
	if strings.TrimSpace(cfg.GarageBucket) == "" {
		return nil, errors.New("documents garage bucket is required")
	}
	if strings.TrimSpace(cfg.GarageAccessKeyID) == "" || strings.TrimSpace(cfg.GarageSecretAccessKey) == "" {
		return nil, errors.New("documents garage credentials are required")
	}
	if strings.TrimSpace(cfg.GarageRegion) == "" {
		cfg.GarageRegion = "us-east-1"
	}
	if cfg.PostgresMaxConns <= 0 {
		cfg.PostgresMaxConns = 10
	}
	if cfg.WatchPollInterval <= 0 {
		cfg.WatchPollInterval = 2 * time.Second
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	poolConfig.MaxConns = cfg.PostgresMaxConns
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.GarageRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.GarageAccessKeyID, cfg.GarageSecretAccessKey, "")),
		awsconfig.WithBaseEndpoint(cfg.GarageEndpoint),
	)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("configure garage s3 client: %w", err)
	}
	s3Client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.GarageForcePathStyle
	})

	store := &PostgresGarageStore{
		pool:              pool,
		s3Client:          s3Client,
		bucket:            strings.TrimSpace(cfg.GarageBucket),
		watchPollInterval: cfg.WatchPollInterval,
	}

	if err := store.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := store.ensureBucket(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (p *PostgresGarageStore) Close() error {
	if p == nil || p.pool == nil {
		return nil
	}
	p.pool.Close()
	return nil
}

func (p *PostgresGarageStore) PutDocument(ctx context.Context, doc model.Document) error {
	if strings.TrimSpace(doc.ID) == "" {
		return errors.New("document id is required")
	}
	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	doc.SchemaVersion = model.SchemaVersion

	contentBytes := []byte(doc.Content)
	hash := sha256.Sum256(contentBytes)
	hashHex := hex.EncodeToString(hash[:])
	contentKey := fmt.Sprintf("documents/%s/%d-%s", doc.ID, now.UnixNano(), hashHex[:12])

	if err := p.putContent(ctx, contentKey, contentBytes); err != nil {
		return err
	}

	metadataJSON, err := json.Marshal(copyStringMap(doc.Metadata))
	if err != nil {
		return fmt.Errorf("marshal document metadata: %w", err)
	}

	_, err = p.pool.Exec(ctx, `
INSERT INTO smith_documents (
	id,
	project_id,
	title,
	format,
	source_type,
	source_ref,
	status,
	metadata,
	content_key,
	content_size,
	content_sha256,
	correlation_id,
	schema_version,
	created_at,
	updated_at
) VALUES (
	$1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12,$13,$14,$15
)
ON CONFLICT (id) DO UPDATE SET
	project_id = EXCLUDED.project_id,
	title = EXCLUDED.title,
	format = EXCLUDED.format,
	source_type = EXCLUDED.source_type,
	source_ref = EXCLUDED.source_ref,
	status = EXCLUDED.status,
	metadata = EXCLUDED.metadata,
	content_key = EXCLUDED.content_key,
	content_size = EXCLUDED.content_size,
	content_sha256 = EXCLUDED.content_sha256,
	correlation_id = EXCLUDED.correlation_id,
	schema_version = EXCLUDED.schema_version,
	updated_at = EXCLUDED.updated_at,
	created_at = LEAST(smith_documents.created_at, EXCLUDED.created_at)
`,
		doc.ID,
		doc.ProjectID,
		doc.Title,
		doc.Format,
		doc.SourceType,
		doc.SourceRef,
		doc.Status,
		string(metadataJSON),
		contentKey,
		int64(len(contentBytes)),
		hashHex,
		doc.CorrelationID,
		doc.SchemaVersion,
		doc.CreatedAt,
		doc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert document metadata: %w", err)
	}
	return nil
}

func (p *PostgresGarageStore) GetDocument(ctx context.Context, docID string) (model.Document, bool, error) {
	row := p.pool.QueryRow(ctx, `
SELECT
	id,
	project_id,
	title,
	format,
	source_type,
	source_ref,
	status,
	metadata,
	content_key,
	correlation_id,
	schema_version,
	created_at,
	updated_at
FROM smith_documents
WHERE id = $1
`, docID)
	record, err := scanDocumentRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Document{}, false, nil
		}
		return model.Document{}, false, err
	}
	content, err := p.getContent(ctx, record.ContentKey)
	if err != nil {
		return model.Document{}, false, err
	}
	return record.toModel(content), true, nil
}

func (p *PostgresGarageStore) ListDocuments(ctx context.Context) ([]model.Document, error) {
	rows, err := p.pool.Query(ctx, `
SELECT
	id,
	project_id,
	title,
	format,
	source_type,
	source_ref,
	status,
	metadata,
	content_key,
	correlation_id,
	schema_version,
	created_at,
	updated_at
FROM smith_documents
ORDER BY updated_at DESC, id DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Document, 0)
	for rows.Next() {
		record, scanErr := scanDocumentRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		content, contentErr := p.getContent(ctx, record.ContentKey)
		if contentErr != nil {
			return nil, contentErr
		}
		out = append(out, record.toModel(content))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *PostgresGarageStore) DeleteDocument(ctx context.Context, docID string) error {
	var contentKey string
	err := p.pool.QueryRow(ctx, "SELECT content_key FROM smith_documents WHERE id = $1", docID).Scan(&contentKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	if _, err := p.pool.Exec(ctx, "DELETE FROM smith_documents WHERE id = $1", docID); err != nil {
		return err
	}

	if strings.TrimSpace(contentKey) != "" {
		_, _ = p.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(p.bucket),
			Key:    aws.String(contentKey),
		})
	}
	return nil
}

func (p *PostgresGarageStore) WatchDocuments(ctx context.Context) <-chan model.Document {
	out := make(chan model.Document)
	go func() {
		defer close(out)
		ticker := time.NewTicker(p.watchPollInterval)
		defer ticker.Stop()

		lastUpdated := time.Now().UTC()
		lastID := ""

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				docs, nextUpdated, nextID, err := p.listUpdatedSince(ctx, lastUpdated, lastID, 256)
				if err != nil {
					continue
				}
				for _, doc := range docs {
					select {
					case <-ctx.Done():
						return
					case out <- doc:
					}
				}
				if len(docs) > 0 {
					lastUpdated = nextUpdated
					lastID = nextID
				}
			}
		}
	}()
	return out
}

func (p *PostgresGarageStore) ensureSchema(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS smith_documents (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	title TEXT NOT NULL,
	format TEXT NOT NULL DEFAULT '',
	source_type TEXT NOT NULL DEFAULT '',
	source_ref TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	content_key TEXT NOT NULL,
	content_size BIGINT NOT NULL DEFAULT 0,
	content_sha256 TEXT NOT NULL DEFAULT '',
	correlation_id TEXT NOT NULL DEFAULT '',
	schema_version TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS smith_documents_updated_idx ON smith_documents (updated_at DESC, id DESC);
`)
	if err != nil {
		return fmt.Errorf("ensure documents schema: %w", err)
	}
	return nil
}

func (p *PostgresGarageStore) ensureBucket(ctx context.Context) error {
	_, err := p.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(p.bucket)})
	if err == nil {
		return nil
	}
	_, createErr := p.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(p.bucket)})
	if createErr != nil {
		return fmt.Errorf("ensure garage bucket %q: %w", p.bucket, createErr)
	}
	return nil
}

func (p *PostgresGarageStore) putContent(ctx context.Context, contentKey string, content []byte) error {
	_, err := p.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(contentKey),
		Body:        bytes.NewReader(content),
		ContentType: aws.String("text/plain; charset=utf-8"),
	})
	if err != nil {
		return fmt.Errorf("put document content %q: %w", contentKey, err)
	}
	return nil
}

func (p *PostgresGarageStore) getContent(ctx context.Context, contentKey string) (string, error) {
	res, err := p.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(contentKey),
	})
	if err != nil {
		return "", fmt.Errorf("get document content %q: %w", contentKey, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("read document content %q: %w", contentKey, err)
	}
	return string(raw), nil
}

func (p *PostgresGarageStore) listUpdatedSince(ctx context.Context, updatedAt time.Time, lastID string, limit int) ([]model.Document, time.Time, string, error) {
	rows, err := p.pool.Query(ctx, `
SELECT
	id,
	project_id,
	title,
	format,
	source_type,
	source_ref,
	status,
	metadata,
	content_key,
	correlation_id,
	schema_version,
	created_at,
	updated_at
FROM smith_documents
WHERE updated_at > $1 OR (updated_at = $1 AND id > $2)
ORDER BY updated_at ASC, id ASC
LIMIT $3
`, updatedAt, lastID, limit)
	if err != nil {
		return nil, updatedAt, lastID, err
	}
	defer rows.Close()

	type entry struct {
		record  documentRecord
		content string
	}
	entries := make([]entry, 0)
	for rows.Next() {
		record, scanErr := scanDocumentRecord(rows)
		if scanErr != nil {
			return nil, updatedAt, lastID, scanErr
		}
		content, contentErr := p.getContent(ctx, record.ContentKey)
		if contentErr != nil {
			return nil, updatedAt, lastID, contentErr
		}
		entries = append(entries, entry{record: record, content: content})
	}
	if err := rows.Err(); err != nil {
		return nil, updatedAt, lastID, err
	}
	if len(entries) == 0 {
		return nil, updatedAt, lastID, nil
	}

	docs := make([]model.Document, 0, len(entries))
	for _, item := range entries {
		docs = append(docs, item.record.toModel(item.content))
	}
	last := entries[len(entries)-1].record
	return docs, last.UpdatedAt, last.ID, nil
}

type documentRecord struct {
	ID            string
	ProjectID     string
	Title         string
	Format        string
	SourceType    string
	SourceRef     string
	Status        string
	Metadata      map[string]string
	ContentKey    string
	CorrelationID string
	SchemaVersion string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (d documentRecord) toModel(content string) model.Document {
	return model.Document{
		ID:            d.ID,
		ProjectID:     d.ProjectID,
		Title:         d.Title,
		Content:       content,
		Format:        d.Format,
		SourceType:    d.SourceType,
		SourceRef:     d.SourceRef,
		Status:        d.Status,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
		Metadata:      copyStringMap(d.Metadata),
		CorrelationID: d.CorrelationID,
		SchemaVersion: d.SchemaVersion,
	}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDocumentRecord(row scanner) (documentRecord, error) {
	var (
		record       documentRecord
		metadataJSON []byte
	)
	err := row.Scan(
		&record.ID,
		&record.ProjectID,
		&record.Title,
		&record.Format,
		&record.SourceType,
		&record.SourceRef,
		&record.Status,
		&metadataJSON,
		&record.ContentKey,
		&record.CorrelationID,
		&record.SchemaVersion,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return documentRecord{}, err
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &record.Metadata); err != nil {
			return documentRecord{}, fmt.Errorf("decode document metadata: %w", err)
		}
	}
	if record.Metadata == nil {
		record.Metadata = map[string]string{}
	}
	return record, nil
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func canonicalDocumentStoreBackend(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "etcd"
	}
	aliases := map[string]string{
		"postgres-garage": "postgres-garage",
		"postgres+garage": "postgres-garage",
		"pg-garage":       "postgres-garage",
		"s3":              "postgres-garage",
		"etcd":            "etcd",
	}
	if resolved, ok := aliases[normalized]; ok {
		return resolved
	}
	return normalized
}

func NormalizeBackend(raw string) string {
	return canonicalDocumentStoreBackend(raw)
}

func IsPostgresGarageBackend(raw string) bool {
	return canonicalDocumentStoreBackend(raw) == "postgres-garage"
}

func IsSupportedBackend(raw string) bool {
	resolved := canonicalDocumentStoreBackend(raw)
	return resolved == "etcd" || resolved == "postgres-garage"
}
