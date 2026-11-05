package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

const embeddingTextLimit = 8 << 10

// Embedding is one vector returned by an embedder.
type Embedding struct {
	Model  string
	Dim    int
	Vector []float32
}

// Embedder produces one embedding for text.
type Embedder func(ctx context.Context, text, mediaType string) (Embedding, error)

// SemanticRow is one vec0 KNN hit.
type SemanticRow struct {
	DocID       string
	ContentHash string
	Distance    float64
}

// EmbeddingSpace describes one registered vec0 space.
type EmbeddingSpace struct {
	Model       string
	Dim         int
	Metric      string
	ElementType string
	Table       string
	Active      bool
	Rows        int
}

func embeddingText(concept contractv1.OkfConcept) string {
	text := concept.Body
	if concept.ContentKind == "blob" && !concept.BodyText {
		text = ""
		if concept.ContentMetadata != nil {
			if value, ok := concept.ContentMetadata["text"].(string); ok {
				text = value
			}
		}
	}
	var builder strings.Builder
	if concept.Title != nil {
		builder.WriteString(*concept.Title)
		builder.WriteString("\n")
	}
	if concept.Description != nil {
		builder.WriteString(*concept.Description)
		builder.WriteString("\n")
	}
	builder.WriteString(text)
	value := builder.String()
	if len(value) > embeddingTextLimit {
		value = value[:embeddingTextLimit]
	}
	return value
}

func (s *Store) storeEmbedding(
	ctx context.Context,
	tx *sql.Tx,
	root string,
	concept contractv1.OkfConcept,
	embedder Embedder,
	now int64,
) (bool, error) {
	text := embeddingText(concept)
	if strings.TrimSpace(text) == "" {
		return false, nil
	}
	mediaType := concept.MediaType
	if mediaType == "" {
		mediaType = "text/markdown"
	}
	embedding, err := embedder(ctx, text, mediaType)
	if err != nil {
		return true, nil
	}
	return false, storeEmbeddingVector(ctx, tx, root, concept, embedding, now)
}

func storeEmbeddingVector(
	ctx context.Context,
	tx *sql.Tx,
	root string,
	concept contractv1.OkfConcept,
	embedding Embedding,
	now int64,
) error {
	if embedding.Dim <= 0 || len(embedding.Vector) != embedding.Dim || embedding.Model == "" {
		return fmt.Errorf("invalid embedding metadata")
	}
	normalized := normalizeVector(embedding.Vector)
	if normalized == nil {
		return fmt.Errorf("invalid embedding vector")
	}
	table, err := ensureEmbeddingSpace(ctx, tx, embedding.Model, embedding.Dim, now)
	if err != nil {
		return err
	}
	docKey := root + "/" + concept.ID
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE doc_key = ?", docKey); err != nil {
		return err
	}
	vector := encodeFloat32(normalized)
	hash := concept.ContentHash
	if hash == "" {
		hash = contentHash(root, concept)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO document_embeddings
			(bundle_root, doc_id, model, dim, vector, content_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		root, concept.ID, embedding.Model, embedding.Dim, vector, hash, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO `+table+` (doc_key, bundle_root, doc_id, content_hash, embedding)
		VALUES (?, ?, ?, ?, vec_f32(?))`,
		docKey, root, concept.ID, hash, vector); err != nil {
		return err
	}
	return nil
}

type sourceEmbedding struct {
	model, hash string
	dim         int
	vector      []byte
	createdAt   int64
}

func loadSourceEmbeddings(ctx context.Context, tx *sql.Tx, root, docID string) ([]sourceEmbedding, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT model, dim, vector, content_hash, created_at FROM document_embeddings
		WHERE bundle_root = ? AND doc_id = ?`, root, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]sourceEmbedding, 0)
	for rows.Next() {
		var item sourceEmbedding
		if err := rows.Scan(&item.model, &item.dim, &item.vector, &item.hash, &item.createdAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func restoreSourceEmbeddings(ctx context.Context, tx *sql.Tx, root, docID string, items []sourceEmbedding) error {
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO document_embeddings
				(bundle_root, doc_id, model, dim, vector, content_hash, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			root, docID, item.model, item.dim, item.vector, item.hash, item.createdAt); err != nil {
			return err
		}
	}
	return nil
}

// ReembedBundle re-embeds every concept for a requested model.
func (s *Store) ReembedBundle(
	ctx context.Context,
	root string,
	model string,
	embedder Embedder,
) (int, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return 0, err
	}
	bundle, err := s.LoadBundle(ctx, root)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	count := 0
	for _, concept := range bundle.Concepts {
		text := embeddingText(concept)
		if strings.TrimSpace(text) == "" {
			continue
		}
		mediaType := concept.MediaType
		if mediaType == "" {
			mediaType = "text/markdown"
		}
		embedding, err := embedder(ctx, text, mediaType)
		if err != nil {
			return 0, err
		}
		if model != "" && embedding.Model != model {
			return 0, fmt.Errorf("embedder returned model %q, want %q", embedding.Model, model)
		}
		if err := storeEmbeddingVector(ctx, tx, root, concept, embedding, now); err != nil {
			return 0, err
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func ensureEmbeddingSpace(ctx context.Context, tx *sql.Tx, model string, dim int, now int64) (string, error) {
	const metric = "cosine"
	const elementType = "float32"
	var table string
	err := tx.QueryRowContext(ctx, `
		SELECT table_name FROM embedding_indexes
		WHERE model = ? AND dim = ? AND metric = ? AND element_type = ?`,
		model, dim, metric, elementType).Scan(&table)
	if err == nil {
		return table, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s\x00%s", model, dim, metric, elementType)))
	table = fmt.Sprintf("vec_embeddings_%s_%d", hex.EncodeToString(hash[:6]), dim)
	statement := fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS %s USING vec0(
		bundle_root TEXT PARTITION KEY,
		doc_key TEXT PRIMARY KEY,
		doc_id TEXT,
		content_hash TEXT,
		embedding float[%d] distance_metric=%s)`, table, dim, metric)
	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO embedding_indexes
			(model, dim, metric, element_type, table_name, active, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)`,
		model, dim, metric, elementType, table, now); err != nil {
		return "", err
	}
	return table, nil
}

func normalizeVector(vector []float32) []float32 {
	var sum float64
	for _, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil
		}
		sum += float64(value) * float64(value)
	}
	if sum == 0 {
		return nil
	}
	norm := math.Sqrt(sum)
	result := make([]float32, len(vector))
	for index, value := range vector {
		result[index] = float32(float64(value) / norm)
	}
	return result
}

func encodeFloat32(vector []float32) []byte {
	data := make([]byte, 4*len(vector))
	for index, value := range vector {
		binary.LittleEndian.PutUint32(data[index*4:], math.Float32bits(value))
	}
	return data
}

// SemanticHits returns the bundle's KNN hits for one model and vector.
func (s *Store) SemanticHits(
	ctx context.Context,
	root string,
	model string,
	vector []float32,
	topK int,
) ([]SemanticRow, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	if topK <= 0 {
		topK = 10
	}
	if topK > 100 {
		topK = 100
	}
	vector = normalizeVector(vector)
	if vector == nil {
		return nil, fmt.Errorf("invalid query vector")
	}
	var table string
	err = s.db.QueryRowContext(ctx, `
		SELECT table_name FROM embedding_indexes
		WHERE model = ? AND dim = ? AND metric = 'cosine' AND element_type = 'float32' AND active = 1`,
		model, len(vector)).Scan(&table)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("embedding space not found for model %q", model)
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT doc_id, content_hash, distance FROM `+table+`
		WHERE embedding MATCH vec_f32(?) AND bundle_root = ? AND k = ?`,
		encodeFloat32(vector), root, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := make([]SemanticRow, 0, topK)
	for rows.Next() {
		var hit SemanticRow
		if err := rows.Scan(&hit.DocID, &hit.ContentHash, &hit.Distance); err != nil {
			return nil, err
		}
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}

// EmbeddingDim returns the dimension for a model, preferring source rows.
func (s *Store) EmbeddingDim(ctx context.Context, root, model string) (int, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return 0, err
	}
	var dim int
	err = s.db.QueryRowContext(ctx, `
		SELECT dim FROM document_embeddings
		WHERE bundle_root = ? AND model = ? ORDER BY created_at DESC LIMIT 1`,
		root, model).Scan(&dim)
	if err == nil {
		return dim, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	err = s.db.QueryRowContext(ctx, `
		SELECT dim FROM embedding_indexes
		WHERE model = ? AND metric = 'cosine' AND element_type = 'float32'
		ORDER BY created_at DESC LIMIT 1`, model).Scan(&dim)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("embedding space not found for model %q", model)
	}
	return dim, err
}

// EmbeddingFindings returns stale and mismatched embedding findings.
func (s *Store) EmbeddingFindings(ctx context.Context, root string) ([]contractv1.AuditFinding, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	findings := make([]contractv1.AuditFinding, 0)
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.doc_id, e.model
		FROM document_embeddings e
		JOIN documents d ON d.bundle_root = e.bundle_root AND d.id = e.doc_id
		WHERE e.bundle_root = ? AND e.content_hash != d.content_hash
		ORDER BY e.doc_id, e.model`, root)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var docID, model string
		if err := rows.Scan(&docID, &model); err != nil {
			rows.Close()
			return nil, err
		}
		findings = append(findings, contractv1.AuditFinding{
			Code: contractv1.AuditEmbeddingStale, Severity: contractv1.AuditSeverityWarning,
			Concept: docID, Target: model,
			Message: "embedding content hash differs from the document",
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	type space struct {
		model, table string
		dim          int
	}
	spaces := make([]space, 0)
	rows, err = s.db.QueryContext(ctx, `
		SELECT model, dim, table_name FROM embedding_indexes
		WHERE metric = 'cosine' AND element_type = 'float32'
		ORDER BY model, dim`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item space
		if err := rows.Scan(&item.model, &item.dim, &item.table); err != nil {
			rows.Close()
			return nil, err
		}
		spaces = append(spaces, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for _, item := range spaces {
		var exists int
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			item.table).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			findings = append(findings, contractv1.AuditFinding{
				Code: contractv1.AuditEmbeddingIndexMismatch, Severity: contractv1.AuditSeverityWarning,
				Target: item.model, Message: "embedding index table is missing",
			})
			continue
		}
		var sourceCount, indexCount int
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM document_embeddings
			WHERE bundle_root = ? AND model = ? AND dim = ?`,
			root, item.model, item.dim).Scan(&sourceCount); err != nil {
			return nil, err
		}
		if err := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM "+item.table+" WHERE bundle_root = ?", root).Scan(&indexCount); err != nil {
			return nil, err
		}
		if sourceCount != indexCount {
			findings = append(findings, contractv1.AuditFinding{
				Code: contractv1.AuditEmbeddingIndexMismatch, Severity: contractv1.AuditSeverityWarning,
				Target:  item.model,
				Message: fmt.Sprintf("embedding index has %d rows for %d source vectors", indexCount, sourceCount),
			})
		}
	}
	return findings, nil
}

// EmbeddingSpaces lists registered spaces with their bundle row counts.
func (s *Store) EmbeddingSpaces(ctx context.Context, root string) ([]EmbeddingSpace, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT model, dim, metric, element_type, table_name, active
		FROM embedding_indexes ORDER BY model, dim`)
	if err != nil {
		return nil, err
	}
	spaces := make([]EmbeddingSpace, 0)
	for rows.Next() {
		var space EmbeddingSpace
		var active int
		if err := rows.Scan(&space.Model, &space.Dim, &space.Metric, &space.ElementType,
			&space.Table, &active); err != nil {
			rows.Close()
			return nil, err
		}
		space.Active = active != 0
		spaces = append(spaces, space)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range spaces {
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM document_embeddings
			WHERE bundle_root = ? AND model = ? AND dim = ?`,
			root, spaces[index].Model, spaces[index].Dim).Scan(&spaces[index].Rows); err != nil {
			return nil, err
		}
	}
	return spaces, nil
}

// RebuildEmbeddingSpace re-inserts one bundle's source vectors into vec0.
func (s *Store) RebuildEmbeddingSpace(ctx context.Context, root, model string, dim int) error {
	root, err := normalizeRoot(root)
	if err != nil {
		return err
	}
	var table string
	err = s.db.QueryRowContext(ctx, `
		SELECT table_name FROM embedding_indexes
		WHERE model = ? AND dim = ? AND metric = 'cosine' AND element_type = 'float32'`,
		model, dim).Scan(&table)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("embedding space not found")
	}
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE bundle_root = ?", root); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT doc_id, vector, content_hash FROM document_embeddings
		WHERE bundle_root = ? AND model = ? AND dim = ?`, root, model, dim)
	if err != nil {
		return err
	}
	type source struct {
		id, hash string
		vector   []byte
	}
	items := make([]source, 0)
	for rows.Next() {
		var item source
		if err := rows.Scan(&item.id, &item.vector, &item.hash); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO `+table+` (doc_key, bundle_root, doc_id, content_hash, embedding)
			VALUES (?, ?, ?, ?, vec_f32(?))`,
			root+"/"+item.id, root, item.id, item.hash, item.vector); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DropEmbeddingSpace removes a bundle's source and index rows.
func (s *Store) DropEmbeddingSpace(ctx context.Context, root, model string, dim int, keepSource bool) error {
	root, err := normalizeRoot(root)
	if err != nil {
		return err
	}
	var table string
	err = s.db.QueryRowContext(ctx, `
		SELECT table_name FROM embedding_indexes
		WHERE model = ? AND dim = ? AND metric = 'cosine' AND element_type = 'float32'`,
		model, dim).Scan(&table)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if table != "" {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE bundle_root = ?", root); err != nil {
			return err
		}
	}
	if !keepSource {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM document_embeddings WHERE bundle_root = ? AND model = ? AND dim = ?`,
			root, model, dim); err != nil {
			return err
		}
	}
	if table != "" {
		var remaining int
		if err := tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM document_embeddings WHERE model = ? AND dim = ?",
			model, dim).Scan(&remaining); err != nil {
			return err
		}
		if remaining == 0 {
			if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS "+table); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM embedding_indexes WHERE model = ? AND dim = ?
				  AND metric = 'cosine' AND element_type = 'float32'`, model, dim); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
