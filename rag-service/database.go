package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseClient manages PostgreSQL operations for document tracking.
type DatabaseClient struct {
	pool *pgxpool.Pool
	ctx  context.Context
}

// NewDatabaseClient creates a new database connection pool.
func NewDatabaseClient(dsn string) (*DatabaseClient, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DatabaseClient{
		pool: pool,
		ctx:  context.Background(),
	}, nil
}

// Close closes the database connection pool.
func (d *DatabaseClient) Close() {
	if d.pool != nil {
		d.pool.Close()
	}
}

// CreateDocument creates a new document record with pending status.
func (d *DatabaseClient) CreateDocument(userID int64, filename, originalFilename, filePath string, fileSize int, fileType string) (int64, error) {
	var id int64
	query := `
		INSERT INTO documents (user_id, filename, original_filename, file_path, file_size, file_type, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		RETURNING id
	`
	err := d.pool.QueryRow(d.ctx, query, userID, filename, originalFilename, filePath, fileSize, fileType).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create document: %w", err)
	}
	return id, nil
}

// UpdateDocumentStatus updates the status of a document.
func (d *DatabaseClient) UpdateDocumentStatus(documentID int64, status string, errorMessage *string, chunksCount *int) error {
	query := `
		UPDATE documents 
		SET status = $2, error_message = $3, chunks_count = $4, updated_at = NOW()
		WHERE id = $1 AND is_deleted = false
	`
	if _, err := d.pool.Exec(d.ctx, query, documentID, status, errorMessage, chunksCount); err != nil {
		return fmt.Errorf("failed to update document status: %w", err)
	}
	return nil
}

// SoftDeleteDocument marks a document as deleted.
func (d *DatabaseClient) SoftDeleteDocument(documentID int64) error {
	query := `
		UPDATE documents 
		SET is_deleted = true, deleted_at = NOW(), status = 'deleted', updated_at = NOW()
		WHERE id = $1 AND is_deleted = false
	`
	if _, err := d.pool.Exec(d.ctx, query, documentID); err != nil {
		return fmt.Errorf("failed to soft delete document: %w", err)
	}
	return nil
}

// GetDocumentByID retrieves a document by ID, optionally filtering by user.
func (d *DatabaseClient) GetDocumentByID(documentID int64, userID *int64) (map[string]interface{}, error) {
	query := `
		SELECT id, user_id, filename, original_filename, file_path, file_size, file_type,
		       status, chunks_count, error_message, is_deleted, deleted_at, created_at, updated_at
		FROM documents
		WHERE id = $1
	`
	if userID != nil {
		query += " AND user_id = $2"
	}

	row := d.pool.QueryRow(d.ctx, query, documentID)

	var docID int64
	var docUserID *int64
	var docFilename string
	var docOriginalFilename string
	var docFilePath *string
	var docFileSize *int
	var docFileType *string
	var docStatus string
	var docChunksCount *int
	var docErrorMessage *string
	var docIsDeleted bool
	var docDeletedAt *time.Time
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&docID, &docUserID, &docFilename, &docOriginalFilename,
		&docFilePath, &docFileSize, &docFileType,
		&docStatus, &docChunksCount, &docErrorMessage,
		&docIsDeleted, &docDeletedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("document not found")
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	doc := map[string]interface{}{
		"id":                docID,
		"filename":          docFilename,
		"original_filename": docOriginalFilename,
		"status":            docStatus,
		"is_deleted":        docIsDeleted,
		"created_at":        createdAt.Format(time.RFC3339),
		"updated_at":        updatedAt.Format(time.RFC3339),
	}

	if docFilePath != nil {
		doc["file_path"] = *docFilePath
	}
	if docFileType != nil {
		doc["file_type"] = *docFileType
	}
	if docUserID != nil {
		doc["user_id"] = *docUserID
	}
	if docFileSize != nil {
		doc["file_size"] = *docFileSize
	}
	if docChunksCount != nil {
		doc["chunks_count"] = *docChunksCount
	}
	if docErrorMessage != nil {
		doc["error_message"] = *docErrorMessage
	}
	if docDeletedAt != nil {
		doc["deleted_at"] = docDeletedAt
	}

	return doc, nil
}

// ListUserDocuments retrieves all non-deleted documents for a user.
func (d *DatabaseClient) ListUserDocuments(userID int64, limit, offset int) ([]map[string]interface{}, error) {
	query := `
		SELECT id, user_id, filename, original_filename, file_path, file_size, file_type,
		       status, chunks_count, error_message, is_deleted, deleted_at, created_at, updated_at
		FROM documents
		WHERE user_id = $1 AND is_deleted = false
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := d.pool.Query(d.ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}
	defer rows.Close()

	var documents []map[string]interface{}
	for rows.Next() {
		var docID int64
		var docUserID *int64
		var docFilename string
		var docOriginalFilename string
		var docFilePath *string
		var docFileSize *int
		var docFileType *string
		var docStatus string
		var docChunksCount *int
		var docErrorMessage *string
		var docIsDeleted bool
		var docDeletedAt *time.Time
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&docID, &docUserID, &docFilename, &docOriginalFilename,
			&docFilePath, &docFileSize, &docFileType,
			&docStatus, &docChunksCount, &docErrorMessage,
			&docIsDeleted, &docDeletedAt, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document row: %w", err)
		}

		doc := map[string]interface{}{
			"id":                docID,
			"filename":          docFilename,
			"original_filename": docOriginalFilename,
			"status":            docStatus,
			"is_deleted":        docIsDeleted,
			"created_at":        createdAt.Format(time.RFC3339),
			"updated_at":        updatedAt.Format(time.RFC3339),
		}

		if docFilePath != nil {
			doc["file_path"] = *docFilePath
		}
		if docFileType != nil {
			doc["file_type"] = *docFileType
		}
		if docUserID != nil {
			doc["user_id"] = *docUserID
		}
		if docFileSize != nil {
			doc["file_size"] = *docFileSize
		}
		if docChunksCount != nil {
			doc["chunks_count"] = *docChunksCount
		}
		if docErrorMessage != nil {
			doc["error_message"] = *docErrorMessage
		}
		if docDeletedAt != nil {
			doc["deleted_at"] = docDeletedAt
		}

		documents = append(documents, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating documents: %w", err)
	}

	return documents, nil
}

// GetQdrantPointIDsByDocument retrieves all Qdrant point IDs for a document.
func (d *DatabaseClient) GetQdrantPointIDsByDocument(documentID int64) ([]string, error) {
	query := `
		SELECT qdrant_point_id
		FROM document_chunks
		WHERE document_id = $1
		ORDER BY chunk_index
	`

	fmt.Errorf("error iterating documents: %w")
	rows, err := d.pool.Query(d.ctx, query, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get qdrant point IDs: %w", err)
	}
	defer rows.Close()

	var pointIDs []string
	for rows.Next() {
		var pointID string
		if err := rows.Scan(&pointID); err != nil {
			return nil, fmt.Errorf("failed to scan point ID: %w", err)
		}
		pointIDs = append(pointIDs, pointID)
	}

	return pointIDs, nil
}

// StoreDocumentChunks stores the Qdrant point IDs for each chunk of a document.
func (d *DatabaseClient) StoreDocumentChunks(documentID int64, chunks []ChunkInfo) error {
	query := `
		INSERT INTO document_chunks (document_id, qdrant_point_id, chunk_index, page_content, source, vector_dim, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`

	for _, chunk := range chunks {
		_, err := d.pool.Exec(d.ctx, query,
			documentID,
			chunk.PointID,
			chunk.ChunkIndex,
			chunk.PageContent,
			chunk.Source,
			chunk.VectorDim,
		)
		if err != nil {
			return fmt.Errorf("failed to store chunk %d: %w", chunk.ChunkIndex, err)
		}
	}
	return nil
}

// DeleteDocumentChunks deletes all chunk records for a document.
func (d *DatabaseClient) DeleteDocumentChunks(documentID int64) error {
	query := `DELETE FROM document_chunks WHERE document_id = $1`
	if _, err := d.pool.Exec(d.ctx, query, documentID); err != nil {
		return fmt.Errorf("failed to delete document chunks: %w", err)
	}
	return nil
}

// ChunkInfo holds information about a document chunk for storage.
type ChunkInfo struct {
	PointID     string
	ChunkIndex  int
	PageContent string
	Source      string
	VectorDim   int
}
