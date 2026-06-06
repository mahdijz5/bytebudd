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

	// Create file parser
	parser := NewFileParser()

	// Create chunker with configurable parameters
	chunker := NewChunker(config.ChunkSize, config.ChunkOverlap)

	// Create RAG handler
	handler := NewRAGHandler(config, ollama, qdrant, parser, chunker)

	// Setup Gin router
	router := gin.Default()
	handler.SetupRoutes(router)

	// Start server
	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("RAG Service starting on %s", addr)
	log.Printf("  - Embedding Model: %s", config.EmbeddingModel)
	log.Printf("  - Chat Model: %s", config.ChatModel)
	log.Printf("  - Qdrant: %s:%d", config.QdrantHost, config.QdrantPort)
	log.Printf("  - Ollama: %s", config.OllamaURL)
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