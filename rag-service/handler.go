package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UploadResponse is the response returned after file upload.
type UploadResponse struct {
	Success     bool   `json:"success"`
	FileID      int64  `json:"file_id"`
	Filename    string `json:"filename"`
	ChunksCount int    `json:"chunks_count"`
	Message     string `json:"message"`
	Status      string `json:"status"`
}

// DeleteResponse is the response returned after document deletion.
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// DocumentInfo contains information about a document.
type DocumentInfo struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Filename     string    `json:"filename"`
	OriginalName string    `json:"original_filename"`
	FilePath     string    `json:"file_path"`
	FileSize     int       `json:"file_size"`
	FileType     string    `json:"file_type"`
	Status       string    `json:"status"`
	ChunksCount  int       `json:"chunks_count"`
	ErrorMessage string    `json:"error_message,omitempty"`
	IsDeleted    bool      `json:"is_deleted"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DocumentsListResponse is the response for listing documents.
type DocumentsListResponse struct {
	Documents []DocumentInfo `json:"documents"`
	Total     int            `json:"total"`
}

// SourceInfo contains information about a retrieved source chunk.
type SourceInfo struct {
	Content  string  `json:"content"`
	Filename string  `json:"filename"`
	Score    float64 `json:"score"`
}

// QueryResponse is the response returned after processing a query.
type QueryResponse struct {
	IsSQLQuery bool         `json:"is_sql_query,omitempty"`
	Answer     string       `json:"answer,omitempty"`
	Sources    []SourceInfo `json:"sources,omitempty"`
	Error      string       `json:"error,omitempty"`
}

// HealthResponse is the response for the health check endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// RAGHandler handles all HTTP requests for the RAG service.
type RAGHandler struct {
	config  *Config
	ollama  *OllamaClient
	qdrant  *QdrantClient
	db      *DatabaseClient
	parser  *FileParser
	chunker *Chunker
}

// NewRAGHandler creates a new RAGHandler.
func NewRAGHandler(config *Config, ollama *OllamaClient, qdrant *QdrantClient, db *DatabaseClient, parser *FileParser, chunker *Chunker) *RAGHandler {
	return &RAGHandler{
		config:  config,
		ollama:  ollama,
		qdrant:  qdrant,
		db:      db,
		parser:  parser,
		chunker: chunker,
	}
}

// SetupRoutes configures all HTTP routes on the Gin engine.
func (h *RAGHandler) SetupRoutes(router *gin.Engine) {
	// Apply CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     h.config.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * 60 * 60, // 12 hours
	}))

	// Health check
	router.GET("/health", h.HealthHandler)

	// File upload
	router.POST("/upload", h.UploadHandler)

	// Document management
	router.GET("/documents", h.ListDocumentsHandler)
	router.GET("/documents/:id", h.GetDocumentHandler)
	router.DELETE("/documents/:id", h.DeleteDocumentHandler)

	// Query
	router.POST("/query", h.QueryHandler)

	// Allow OPTIONS for CORS preflight
	router.OPTIONS("/*any", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNoContent)
	})
}

// HealthHandler returns the service health status.
func (h *RAGHandler) HealthHandler(c *gin.Context) {
	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, response)
}

// saveFile saves the uploaded file to the storage directory.
func (h *RAGHandler) saveFile(data []byte, filename string) (string, error) {
	// Create user directory if needed
	userDir := filepath.Join(h.config.DocumentStoragePath, "files")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create a unique filename to avoid collisions
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	uniqueName := fmt.Sprintf("%s_%s%s", name, uuid.New().String(), ext)
	filePath := filepath.Join(userDir, uniqueName)

	// Write file to disk
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return filePath, nil
}

// UploadHandler accepts a file, parses it, chunks it, generates embeddings, and stores in Qdrant.
func (h *RAGHandler) UploadHandler(c *gin.Context) {
	logPrefix := "[UPLOAD]"

	// Get the uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("%s ERROR: Failed to get uploaded file: %v", logPrefix, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No file uploaded. Provide a 'file' form field.",
		})
		return
	}

	log.Printf("%s Received file: %s (size: %d bytes)", logPrefix, file.Filename, file.Size)

	// Limit file size to 50MB
	maxSize := int64(50 * 1024 * 1024)
	if file.Size > maxSize {
		log.Printf("%s ERROR: File too large: %d bytes", logPrefix, file.Size)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "File too large. Maximum size is 50MB.",
		})
		return
	}

	// Get optional user_id from form (passed from backend)
	userID := int64(1) // default user
	if userIDStr := c.PostForm("user_id"); userIDStr != "" {
		if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
			log.Printf("%s WARNING: Invalid user_id: %v, using default", logPrefix, err)
		}
	}

	// Get optional document_id from form (for re-uploading)
	var existingDocID *int64
	if docIDStr := c.PostForm("document_id"); docIDStr != "" {
		var docID int64
		if _, err := fmt.Sscanf(docIDStr, "%d", &docID); err == nil {
			existingDocID = &docID
		}
	}

	// Open the file
	src, err := file.Open()
	if err != nil {
		log.Printf("%s ERROR: Failed to open file: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to open uploaded file.",
		})
		return
	}
	defer src.Close()

	// Read file content
	data, err := io.ReadAll(io.LimitReader(src, maxSize))
	if err != nil {
		log.Printf("%s ERROR: Failed to read file content: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to read uploaded file.",
		})
		return
	}

	log.Printf("%s File content read: %d bytes", logPrefix, len(data))

	// Determine file type
	fileType := ""
	switch {
	case strings.HasSuffix(strings.ToLower(file.Filename), ".pdf"):
		fileType = "application/pdf"
	case strings.HasSuffix(strings.ToLower(file.Filename), ".txt"):
		fileType = "text/plain"
	case strings.HasSuffix(strings.ToLower(file.Filename), ".docx"):
		fileType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	// Save file to disk
	filePath, err := h.saveFile(data, file.Filename)
	if err != nil {
		log.Printf("%s ERROR: Failed to save file: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to save file: " + err.Error(),
		})
		return
	}
	log.Printf("%s File saved to: %s", logPrefix, filePath)

	// Parse the file to extract text
	log.Printf("%s Parsing file: %s", logPrefix, file.Filename)
	text, err := h.parser.ParseFile(data, file.Filename)
	if err != nil {
		log.Printf("%s ERROR: Failed to parse file: %v", logPrefix, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to parse file: " + err.Error(),
		})
		return
	}

	if strings.TrimSpace(text) == "" {
		log.Printf("%s ERROR: No text content found in file", logPrefix)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No text content found in the file.",
		})
		return
	}

	log.Printf("%s File parsed successfully: %d characters extracted", logPrefix, len(text))

	// Chunk the text
	log.Printf("%s Chunking text (size=%d, overlap=%d)", logPrefix, h.chunker.ChunkSize, h.chunker.ChunkOverlap)
	chunks := h.chunker.Chunk(text)
	if len(chunks) == 0 {
		log.Printf("%s ERROR: No chunks created", logPrefix)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to create chunks from the file content.",
		})
		return
	}

	log.Printf("%s Created %d chunks from file", logPrefix, len(chunks))

	// Create the document record in database BEFORE processing (needed for document_id in Qdrant)
	var docID int64
	var existingDocIDVal int64 // value to use when setting DocumentID
	if existingDocID != nil {
		docID = *existingDocID
		existingDocIDVal = *existingDocID
		log.Printf("%s Using existing document: %d", logPrefix, docID)
	} else {
		log.Printf("%s Creating new document record for user %d", logPrefix, userID)
		var err error
		docID, err = h.db.CreateDocument(userID, file.Filename, file.Filename, filePath, int(file.Size), fileType)
		if err != nil {
			log.Printf("%s ERROR: Failed to create document record: %v", logPrefix, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create document record: " + err.Error(),
			})
			return
		}
		existingDocIDVal = docID
		log.Printf("%s Document record created with ID: %d", logPrefix, docID)
	}

	// Generate embeddings for each chunk and build document chunks
	var documentChunks []DocumentChunk
	var chunkInfos []ChunkInfo
	for i, chunkText := range chunks {
		select {
		case <-c.Request.Context().Done():
			log.Printf("%s ERROR: Request timed out during embedding generation", logPrefix)
			c.JSON(http.StatusRequestTimeout, gin.H{
				"success": false,
				"error":   "Request timed out while generating embeddings.",
			})
			return
		default:
		}

		log.Printf("%s Generating embedding for chunk %d/%d...", logPrefix, i+1, len(chunks))
		vector, err := h.ollama.GetEmbedding(chunkText)
		if err != nil {
			log.Printf("%s ERROR: Failed to generate embedding for chunk %d: %v", logPrefix, i+1, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to generate embedding for chunk %d: %s", i+1, err.Error()),
			})
			return
		}
		log.Printf("%s Embedding %d generated successfully (dim=%d)", logPrefix, i+1, len(vector))

		// Generate a UUID for this point
		pointID := uuid.New().String()

		documentChunks = append(documentChunks, DocumentChunk{
			ID:          pointID,
			PageContent: chunkText,
			Source:      file.Filename,
			Vector:      vector,
			DocumentID:  existingDocIDVal, // Add document_id to the chunk for tracking
		})

		chunkInfos = append(chunkInfos, ChunkInfo{
			PointID:     pointID,
			ChunkIndex:  i,
			PageContent: chunkText,
			Source:      file.Filename,
			VectorDim:   len(vector),
		})
	}

	// Upsert all chunks into Qdrant (with document_id in payload)
	log.Printf("%s Upserting %d chunks to Qdrant...", logPrefix, len(documentChunks))
	if err := h.qdrant.UpsertChunks(documentChunks); err != nil {
		log.Printf("%s ERROR: Failed to upsert chunks to Qdrant: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to store chunks in Qdrant: " + err.Error(),
		})
		return
	}
	log.Printf("%s Successfully stored %d chunks in Qdrant", logPrefix, len(documentChunks))

	// Store chunk-point mappings in PostgreSQL
	log.Printf("%s Storing chunk mappings in PostgreSQL...", logPrefix)
	if err := h.db.StoreDocumentChunks(docID, chunkInfos); err != nil {
		log.Printf("%s ERROR: Failed to store chunk mappings: %v", logPrefix, err)
		// Don't fail the whole operation, just log the error
	}

	// Update document status to completed
	chunksInt := len(chunks)
	if err := h.db.UpdateDocumentStatus(docID, "completed", nil, &chunksInt); err != nil {
		log.Printf("%s WARNING: Failed to update document status to completed: %v", logPrefix, err)
	}

	log.Printf("%s Upload complete: docID=%d, chunks=%d", logPrefix, docID, len(chunks))
	response := UploadResponse{
		Success:     true,
		FileID:      docID,
		Filename:    file.Filename,
		ChunksCount: len(chunks),
		Message:     "File processed successfully",
		Status:      "completed",
	}
	c.JSON(http.StatusOK, response)
}

// ListDocumentsHandler returns a list of documents for the user.
func (h *RAGHandler) ListDocumentsHandler(c *gin.Context) {
	logPrefix := "[DOCUMENTS]"

	// Get optional user_id from query
	userID := int64(1) // default user
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
			log.Printf("%s WARNING: Invalid user_id: %v, using default", logPrefix, err)
		}
	}

	// Get pagination params
	limit := 50
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		fmt.Sscanf(offsetStr, "%d", &offset)
	}

	log.Printf("%s Listing documents for user %d (limit=%d, offset=%d)", logPrefix, userID, limit, offset)

	documents, err := h.db.ListUserDocuments(userID, limit, offset)
	if err != nil {
		log.Printf("%s ERROR: Failed to list documents: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list documents: " + err.Error(),
		})
		return
	}

	// Convert to DocumentInfo slice
	var docInfos []DocumentInfo
	for _, doc := range documents {
		var docInfo DocumentInfo
		if id, ok := doc["id"].(int64); ok {
			docInfo.ID = id
		}
		if uid, ok := doc["user_id"].(int64); ok {
			docInfo.UserID = uid
		}
		if fn, ok := doc["filename"].(string); ok {
			docInfo.Filename = fn
		}
		if ofn, ok := doc["original_filename"].(string); ok {
			docInfo.OriginalName = ofn
		}
		if fp, ok := doc["file_path"].(string); ok {
			docInfo.FilePath = fp
		}
		if fs, ok := doc["file_size"].(int); ok {
			docInfo.FileSize = fs
		}
		if ft, ok := doc["file_type"].(string); ok {
			docInfo.FileType = ft
		}
		if st, ok := doc["status"].(string); ok {
			docInfo.Status = st
		}
		if cc, ok := doc["chunks_count"].(int); ok {
			docInfo.ChunksCount = cc
		}
		if em, ok := doc["error_message"].(string); ok {
			docInfo.ErrorMessage = em
		}
		if del, ok := doc["is_deleted"].(bool); ok {
			docInfo.IsDeleted = del
		}
		if ca, ok := doc["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ca); err == nil {
				docInfo.CreatedAt = t
			}
		}
		if ua, ok := doc["updated_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ua); err == nil {
				docInfo.UpdatedAt = t
			}
		}
		docInfos = append(docInfos, docInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": docInfos,
		"total":     len(docInfos),
	})
}

// GetDocumentHandler returns a single document by ID.
func (h *RAGHandler) GetDocumentHandler(c *gin.Context) {
	logPrefix := "[DOCUMENT]"

	idStr := c.Param("id")
	var docID int64
	if _, err := fmt.Sscanf(idStr, "%d", &docID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid document ID",
		})
		return
	}

	log.Printf("%s Getting document %d", logPrefix, docID)

	doc, err := h.db.GetDocumentByID(docID, nil)
	if err != nil {
		log.Printf("%s ERROR: %v", logPrefix, err)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"document": doc,
	})
}

// DeleteDocumentHandler deletes a document and its associated data.
func (h *RAGHandler) DeleteDocumentHandler(c *gin.Context) {
	logPrefix := "[DELETE]"

	idStr := c.Param("id")
	var docID int64
	if _, err := fmt.Sscanf(idStr, "%d", &docID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid document ID",
		})
		return
	}

	log.Printf("%s Deleting document %d", logPrefix, docID)

	// Delete points from Qdrant using document_id filter
	log.Printf("%s Deleting Qdrant points with document_id=%d", logPrefix, docID)
	if _, err := h.qdrant.DeletePointsByDocumentID(docID); err != nil {
		log.Printf("%s WARNING: Failed to delete points from Qdrant: %v", logPrefix, err)
		// Continue with database deletion even if Qdrant delete fails
	} else {
		log.Printf("%s Successfully deleted Qdrant points for document %d", logPrefix, docID)
	}

	// Delete chunk records from database
	if err := h.db.DeleteDocumentChunks(docID); err != nil {
		log.Printf("%s WARNING: Failed to delete document chunks from database: %v", logPrefix, err)
	}

	// Soft delete the document
	if err := h.db.SoftDeleteDocument(docID); err != nil {
		log.Printf("%s ERROR: Failed to soft delete document: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete document: " + err.Error(),
		})
		return
	}

	// Delete the physical file
	log.Printf("%s Deleting physical file for document %d", logPrefix, docID)
	doc, err := h.db.GetDocumentByID(docID, nil)
	if err == nil {
		if filePath, ok := doc["file_path"].(string); ok && filePath != "" {
			if err := os.Remove(filePath); err != nil {
				log.Printf("%s WARNING: Failed to delete file %s: %v", logPrefix, filePath, err)
			} else {
				log.Printf("%s Successfully deleted file: %s", logPrefix, filePath)
			}
		}
	}

	log.Printf("%s Document %d deleted successfully", logPrefix, docID)
	c.JSON(http.StatusOK, DeleteResponse{
		Success: true,
		Message: fmt.Sprintf("Document '%s' deleted successfully", idStr),
	})
}

// QueryHandler processes a user query - classifies it and either returns SQL flag or performs RAG.
func (h *RAGHandler) QueryHandler(c *gin.Context) {
	logPrefix := "[QUERY]"

	var request struct {
		Query string `json:"query" binding:"required"`
	}

	log.Printf("%s Received query request", logPrefix)

	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("%s ERROR: Invalid request body: %v", logPrefix, err)
		c.JSON(http.StatusBadRequest, QueryResponse{
			Error: "Invalid request. Provide a 'query' field.",
		})
		return
	}

	query := strings.TrimSpace(request.Query)
	if query == "" {
		log.Printf("%s ERROR: Empty query", logPrefix)
		c.JSON(http.StatusBadRequest, QueryResponse{
			Error: "Query cannot be empty.",
		})
		return
	}

	log.Printf("%s Processing query: %.100s...", logPrefix, query)

	// Step 1: Classify if the query is SQL-related
	log.Printf("%s Step 1: Classifying query as SQL or RAG", logPrefix)
	isSQL, err := h.ollama.IsSQLQuery(query)
	if err != nil {
		log.Printf("%s ERROR: Failed to classify query: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, QueryResponse{
			Error: "Failed to classify query: " + err.Error(),
		})
		return
	}

	log.Printf("%s Step 1 result: IsSQL=%v", logPrefix, isSQL)
	if isSQL {
		log.Printf("%s Query classified as SQL-related, returning flag", logPrefix)
		c.JSON(http.StatusOK, QueryResponse{
			IsSQLQuery: true,
		})
		return
	}

	// Step 2: Generate embedding for the question
	log.Printf("%s Step 2: Generating query embedding", logPrefix)
	queryVector, err := h.ollama.GetEmbedding(query)
	if err != nil {
		log.Printf("%s ERROR: Failed to generate query embedding: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, QueryResponse{
			Error: "Failed to generate query embedding: " + err.Error(),
		})
		return
	}
	log.Printf("%s Step 2 complete: embedding dim=%d", logPrefix, len(queryVector))

	// Step 3: Search Qdrant for similar chunks
	log.Printf("%s Step 3: Searching Qdrant (topK=%d)", logPrefix, h.config.TopK)
	retrievedChunks, err := h.qdrant.SearchSimilar(queryVector, h.config.TopK)
	if err != nil {
		log.Printf("%s ERROR: Failed to search documents: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, QueryResponse{
			Error: "Failed to search documents: " + err.Error(),
		})
		return
	}
	log.Printf("%s Step 3 complete: found %d chunks", logPrefix, len(retrievedChunks))

	// Step 4: Build context from retrieved chunks
	var retrievedTexts []string
	var sources []SourceInfo
	for _, chunk := range retrievedChunks {
		retrievedTexts = append(retrievedTexts, chunk.PageContent)
		sources = append(sources, SourceInfo{
			Content:  chunk.PageContent,
			Filename: chunk.Source,
			Score:    *chunk.Score,
		})
	}

	documentsContext := strings.Join(retrievedTexts, "\n\n")
	log.Printf("%s Step 4 complete: context length=%d chars", logPrefix, len(documentsContext))

	// Step 5: Generate answer using LLM
	log.Printf("%s Step 5: Generating answer from LLM", logPrefix)
	answer, err := h.generateAnswer(query, documentsContext)
	if err != nil {
		log.Printf("%s ERROR: Failed to generate answer: %v", logPrefix, err)
		c.JSON(http.StatusInternalServerError, QueryResponse{
			Error: "Failed to generate answer: " + err.Error(),
		})
		return
	}

	log.Printf("%s Query complete. Answer length=%d chars, sources=%d", logPrefix, len(answer), len(sources))
	c.JSON(http.StatusOK, QueryResponse{
		IsSQLQuery: false,
		Answer:     answer,
		Sources:    sources,
	})
}

// generateAnswer creates a prompt with context and calls the LLM to generate an answer.
func (h *RAGHandler) generateAnswer(question, context string) (string, error) {
	messages := []Message{
		{
			Role: "system",
			Content: `You are a helpful assistant. Answer the user's question based only on the provided context documents.
If the context doesn't contain enough information to answer the question, say "I don't have enough information in the provided documents to answer that question."
Do not make up information that isn't in the context.
Provide clear, concise, and helpful answers.`,
		},
		{
			Role:    "user",
			Content: "Context:\n\n" + context + "\n\nQuestion: " + question,
		},
	}

	return h.ollama.Chat(messages)
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string {
	return &s
}
