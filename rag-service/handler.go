package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// UploadResponse is the response returned after file upload.
type UploadResponse struct {
	Success     bool   `json:"success"`
	FileID      string `json:"file_id"`
	Filename    string `json:"filename"`
	ChunksCount int    `json:"chunks_count"`
	Message     string `json:"message"`
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
	parser  *FileParser
	chunker *Chunker
}

// NewRAGHandler creates a new RAGHandler.
func NewRAGHandler(config *Config, ollama *OllamaClient, qdrant *QdrantClient, parser *FileParser, chunker *Chunker) *RAGHandler {
	return &RAGHandler{
		config:  config,
		ollama:  ollama,
		qdrant:  qdrant,
		parser:  parser,
		chunker: chunker,
	}
}

// SetupRoutes configures all HTTP routes on the Gin engine.
func (h *RAGHandler) SetupRoutes(router *gin.Engine) {
	// Apply CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     h.config.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * 60 * 60, // 12 hours
	}))

	// Health check
	router.GET("/health", h.HealthHandler)

	// File upload
	router.POST("/upload", h.UploadHandler)

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

	// Generate embeddings for each chunk and build document chunks
	var documentChunks []DocumentChunk
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

		documentChunks = append(documentChunks, DocumentChunk{
			PageContent: chunkText,
			Source:      file.Filename,
			Vector:      vector,
		})
	}

	// Upsert all chunks into Qdrant
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

	// Generate a simple file ID based on filename and timestamp
	fileID := "upload_" + strings.ReplaceAll(strings.ReplaceAll(file.Filename, ".", "_"), " ", "_") + "_" + time.Now().Format("20060102150405")

	response := UploadResponse{
		Success:     true,
		FileID:      fileID,
		Filename:    file.Filename,
		ChunksCount: len(documentChunks),
		Message:     "File processed successfully",
	}

	log.Printf("%s Upload complete: fileID=%s, chunks=%d", logPrefix, fileID, len(documentChunks))
	c.JSON(http.StatusOK, response)
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
