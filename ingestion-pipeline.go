package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

const (
	collectionName = "company_docs"
	ollamaEndpoint = "http://localhost:11434/api/embeddings"
	embeddingModel = "embeddinggemma"
	vectorSize     = 768 // Output dimension size for nomic-embed-text
)

type Document struct {
	PageContent string
	Source      string
}

type OllamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func main() {
	fmt.Println("=== Go Native RAG Document Ingestion Pipeline ===\n")
	ctx := context.Background()

	// 1. Connect to Qdrant via official high-performance gRPC client
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
		
	})
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant gRPC: %v", err)
	}
	defer client.Close()

	// 2. Setup the storage collection
	setupCollection(ctx, client)

	// 3. Load text files natively from disk
	docsPath := "docs"
	documents, err := loadDocuments(docsPath)
	if err != nil {
		log.Fatalf("Error loading documents: %v", err)
	}

	// 4. Chunk text with safe sliding windows (Chunk Size: 1000, Overlap: 200)
	chunks := splitDocuments(documents, 200, 20)

	// 5. Build vector point payload and execute a batch upsert
	fmt.Printf("\nProcessing and ingesting %d text chunks...\n", len(chunks))
	var points []*qdrant.PointStruct

	for i, chunk := range chunks {
		fmt.Printf("[%d/%d] Generating local embedding for chunk (Source: %s)...\n", i+1, len(chunks), chunk.Source)

		vector, err := getOllamaEmbedding(chunk.PageContent)
		if err != nil {
			log.Fatalf("Ollama embedding generation failed: %v", err)
		}

		payload := qdrant.NewValueMap(map[string]interface{}{
			"page_content": chunk.PageContent,
			"source":       chunk.Source,
		})

		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(uuid.New().String()),
			Vectors: qdrant.NewVectorsDense(vector),
			Payload: payload,
		})
	}

	_, err = client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Points:         points,
	})
	if err != nil {
		log.Fatalf("Failed to upsert vectors into Qdrant collection: %v", err)
	}

	fmt.Println("\n✅ Ingestion complete! Local Qdrant and Ollama pipeline is running cleanly.")
}

func setupCollection(ctx context.Context, client *qdrant.Client) {
	exists, err := client.CollectionExists(ctx, collectionName)
	if err != nil {
		log.Fatalf("Failed checking collection existence: %v", err)
	}

	if exists {
		fmt.Printf("✅ Qdrant collection '%s' already exists.\n", collectionName)
		return
	}

	fmt.Printf("Creating clean Qdrant collection '%s' using Cosine similarity...\n", collectionName)
	err = client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig:  qdrant.NewVectorsConfig(&qdrant.VectorParams{
    Size:     vectorSize,
    Distance: qdrant.Distance_Cosine,
}),
	})
	if err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	}
}

func loadDocuments(docsPath string) ([]Document, error) {
	var docs []Document
	if _, err := os.Stat(docsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("target directory ./%s does not exist", docsPath)
	}

	err := filepath.Walk(docsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			docs = append(docs, Document{
				PageContent: string(content),
				Source:      path,
			})
			fmt.Printf("File loaded successfully: %s (%d chars)\n", path, len(content))
		}
		return nil
	})

	if len(docs) == 0 {
		return nil, fmt.Errorf("no plain text (.txt) files found inside ./%s", docsPath)
	}
	return docs, err
}

func splitDocuments(documents []Document, chunkSize, chunkOverlap int) []Document {
	var chunks []Document
	for _, doc := range documents {
		runes := []rune(doc.PageContent) // Explicitly cast to rune to prevent cutting Unicode characters
		length := len(runes)

		start := 0
		for start < length {
			end := start + chunkSize
			if end > length {
				end = length
			}

			chunks = append(chunks, Document{
				PageContent: string(runes[start:end]),
				Source:      doc.Source,
			})

			if end == length {
				break
			}
			start += (chunkSize - chunkOverlap)
		}
	}
	return chunks
}

func getOllamaEmbedding(prompt string) ([]float32, error) {
	reqBody, err := json.Marshal(OllamaEmbedRequest{
		Model:  embeddingModel,
		Prompt: prompt,
	})
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 500 * time.Second}
	resp, err := httpClient.Post(ollamaEndpoint, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned bad status: %s", resp.Status)
	}

	var embedResp OllamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, err
	}

	return embedResp.Embedding, nil
}