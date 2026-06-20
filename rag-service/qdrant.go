package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

// QdrantClient wraps Qdrant operations for the RAG service.
type QdrantClient struct {
	client         *qdrant.Client
	collectionName string
	vectorSize     int64
	ctx            context.Context
}

// DocumentChunk represents a chunk of text with its metadata.
type DocumentChunk struct {
	ID          string
	Vector      []float32
	PageContent string
	Source      string
	DocumentID  int64 // Added: document_id for tracking and deletion
	Score       *float64
}

// NewQdrantClient creates a new Qdrant client connection.
func NewQdrantClient(host string, port int, collectionName string, vectorSize int) (*QdrantClient, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	return &QdrantClient{
		client:         client,
		collectionName: collectionName,
		vectorSize:     int64(vectorSize),
		ctx:            context.Background(),
	}, nil
}

// Close closes the Qdrant client connection.
func (c *QdrantClient) Close() {
	c.client.Close()
}

// EnsureCollection creates the collection if it doesn't exist.
func (c *QdrantClient) EnsureCollection() error {
	exists, err := c.client.CollectionExists(c.ctx, c.collectionName)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}

	if exists {
		return nil
	}

	fmt.Printf("Creating Qdrant collection '%s' with Cosine similarity...\n", c.collectionName)

	vectorsConfig := qdrant.NewVectorsConfig(&qdrant.VectorParams{
		Size:     uint64(c.vectorSize),
		Distance: qdrant.Distance_Cosine,
	})

	err = c.client.CreateCollection(c.ctx, &qdrant.CreateCollection{
		CollectionName: c.collectionName,
		VectorsConfig:  vectorsConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}

// UpsertChunks adds or updates chunks with their embeddings in Qdrant.
func (c *QdrantClient) UpsertChunks(chunks []DocumentChunk) error {
	var points []*qdrant.PointStruct

	for _, chunk := range chunks {
		payload := make(map[string]*qdrant.Value)
		payload["page_content"] = qdrant.NewValueString(chunk.PageContent)
		payload["source"] = qdrant.NewValueString(chunk.Source)
		payload["document_id"] = qdrant.NewValueInt(int64(chunk.DocumentID))

		point := &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(uuid.New().String()),
			Vectors: qdrant.NewVectorsDense(chunk.Vector),
			Payload: payload,
		}
		points = append(points, point)
	}

	_, err := c.client.Upsert(c.ctx, &qdrant.UpsertPoints{
		CollectionName: c.collectionName,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert vectors: %w", err)
	}

	return nil
}

// DeletePoints removes points from Qdrant by their IDs.
func (c *QdrantClient) DeletePoints(pointIDs []string) error {
	if len(pointIDs) == 0 {
		return nil
	}

	// Build point IDs using the correct PointId structure
	var pointIds []*qdrant.PointId
	for _, id := range pointIDs {
		pointIds = append(pointIds, qdrant.NewIDUUID(id))
	}

	_, err := c.client.Delete(c.ctx, &qdrant.DeletePoints{
		CollectionName: c.collectionName,
		Points:         qdrant.NewPointsSelectorIDs(pointIds),
	})
	if err != nil {
		return fmt.Errorf("failed to delete points from Qdrant: %w", err)
	}

	return nil
}

// DeletePointsByDocumentID removes points from Qdrant by document_id filter.
func (c *QdrantClient) DeletePointsByDocumentID(documentID int64) (int64, error) {
	// Create a condition that matches the document_id
	condition := qdrant.NewMatchInt("document_id", documentID)

	// Create a filter with the condition
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{condition},
	}

	// Delete points matching the filter
	_, err := c.client.Delete(c.ctx, &qdrant.DeletePoints{
		CollectionName: c.collectionName,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: filter,
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to delete points by document_id from Qdrant: %w", err)
	}

	return documentID, nil
}

// SearchSimilar searches for the top K most similar chunks to the query vector.
func (c *QdrantClient) SearchSimilar(queryVector []float32, topK int) ([]DocumentChunk, error) {
	limit := uint64(topK)

	searchResult, err := c.client.Query(c.ctx, &qdrant.QueryPoints{
		CollectionName: c.collectionName,
		Query:          qdrant.NewQueryDense(queryVector),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search Qdrant: %w", err)
	}

	var chunks []DocumentChunk
	for _, point := range searchResult {
		var content, source string

		if p, ok := point.Payload["page_content"]; ok {
			content = p.GetStringValue()
		}
		if p, ok := point.Payload["source"]; ok {
			source = p.GetStringValue()
		}

		score := float64(point.Score)

		chunks = append(chunks, DocumentChunk{
			ID:          point.Id.String(),
			Vector:      queryVector,
			PageContent: content,
			Source:      source,
			Score:       &score,
		})
	}

	return chunks, nil
}
