package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration from environment variables
	config := loadConfig()

	// Create Ollama client with configurable models
	ollama := NewOllamaClient(
		config.OllamaURL,
		config.EmbeddingModel,
		config.ChatModel,
	)

	// Connect to Qdrant
	qdrant, err := NewQdrantClient(config.QdrantHost, config.QdrantPort, config.CollectionName, config.VectorSize)
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer qdrant.Close()

	// Ensure the collection exists
	if err := qdrant.EnsureCollection(); err != nil {
		log.Fatalf("Failed to ensure Qdrant collection: %v", err)
	}

	// Connect to PostgreSQL for document tracking
	db, err := NewDatabaseClient(config.getDSN())
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	log.Printf("Connected to PostgreSQL database")

	// Create file parser
	parser := NewFileParser()

	// Create chunker with configurable parameters
	chunker := NewChunker(config.ChunkSize, config.ChunkOverlap)

	// Create RAG handler
	handler := NewRAGHandler(config, ollama, qdrant, db, parser, chunker)

	// Setup Gin router
	router := gin.Default()
	handler.SetupRoutes(router)

	// Create document storage directory if it doesn't exist
	if err := os.MkdirAll(config.DocumentStoragePath, 0755); err != nil {
		log.Fatalf("Failed to create document storage directory: %v", err)
	}
	log.Printf("Document storage path: %s", config.DocumentStoragePath)

	// Start server
	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("RAG Service starting on %s", addr)
	log.Printf("  - Embedding Model: %s", config.EmbeddingModel)
	log.Printf("  - Chat Model: %s", config.ChatModel)
	log.Printf("  - Qdrant: %s:%d", config.QdrantHost, config.QdrantPort)
	log.Printf("  - Ollama: %s", config.OllamaURL)
	log.Printf("  - PostgreSQL: %s:%d", config.PostgresHost, config.PostgresPort)
	log.Printf("  - Collection: %s", config.CollectionName)

	// Graceful shutdown
	go func() {
		if err := router.Run(addr); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	log.Println("Server stopped")
}
