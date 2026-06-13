package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OllamaClient handles all communication with the Ollama API.
type OllamaClient struct {
	BaseURL        string
	EmbeddingModel string
	ChatModel      string
	httpClient     *http.Client
}

// OllamaEmbedRequest is the payload for generating embeddings.
type OllamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbedResponse is the response from the embeddings API.
type OllamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// OllamaChatRequest is the payload for chat completions.
type OllamaChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// Message holds chat roles and content strings.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaChatResponse is the response from the chat API.
type OllamaChatResponse struct {
	Message Message `json:"message"`
}

// NewOllamaClient creates a new Ollama client with the given configuration.
func NewOllamaClient(baseURL, embeddingModel, chatModel string) *OllamaClient {
	return &OllamaClient{
		BaseURL:        baseURL,
		EmbeddingModel: embeddingModel,
		ChatModel:      chatModel,
		httpClient:     &http.Client{Timeout: 120 * time.Second},
	}
}

// GetEmbedding generates an embedding vector for the given text using the configured embedding model.
// Uses the same endpoint as ingestion-pipeline.go: /api/embeddings
func (c *OllamaClient) GetEmbedding(text string) ([]float32, error) {
	// Build request exactly like ingestion-pipeline.go
	reqBody, err := json.Marshal(OllamaEmbedRequest{
		Model:  c.EmbeddingModel,
		Prompt: text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	// Use the same endpoint as ingestion-pipeline.go
	embeddingURL := c.BaseURL + "/api/embeddings"

	// DETAILED LOGGING: Log the exact request being sent
	fmt.Printf("[EMBEDDING] URL: %s\n", embeddingURL)
	fmt.Printf("[EMBEDDING] Model: %s\n", c.EmbeddingModel)
	fmt.Printf("[EMBEDDING] Request body (raw): %s\n", string(reqBody))
	fmt.Printf("[EMBEDDING] Text length: %d chars\n", len(text))
	fmt.Printf("[EMBEDDING] Text preview: %.200s...\n", text)

	// Try with a longer timeout like ingestion-pipeline.go (500s)
	httpClient := &http.Client{Timeout: 500 * time.Second}

	resp, err := httpClient.Post(embeddingURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama embeddings API (%s): %w", embeddingURL, err)
	}
	defer resp.Body.Close()

	fmt.Printf("[EMBEDDING] Response status code: %d (%s)\n", resp.StatusCode, resp.Status)

	// Read and log the response body for debugging
	var responseBody bytes.Buffer
	if _, err := responseBody.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	fmt.Printf("[EMBEDDING] Response body: %s\n", responseBody.String())

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama embeddings API returned status: %s (model: %s, response: %s)", resp.Status, c.EmbeddingModel, responseBody.String())
	}

	// Try to decode as OllamaEmbedResponse
	var embedResp OllamaEmbedResponse
	if err := json.Unmarshal(responseBody.Bytes(), &embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w (raw: %s)", err, responseBody.String())
	}

	if len(embedResp.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned from Ollama (model: %s, response: %s)", c.EmbeddingModel, responseBody.String())
	}

	fmt.Printf("[EMBEDDING] Success! Got %d vectors\n", len(embedResp.Embedding))
	return embedResp.Embedding, nil
}

// Chat sends a chat message to the configured model and returns the response text.
func (c *OllamaClient) Chat(messages []Message) (string, error) {
	reqBody, err := json.Marshal(OllamaChatRequest{
		Model:    c.ChatModel,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	resp, err := c.httpClient.Post(c.BaseURL+"/api/chat", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama chat API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama chat API returned status: %s", resp.Status)
	}

	var chatResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode chat response: %w", err)
	}

	return chatResp.Message.Content, nil
}

// IsSQLQuery classifies whether a question is SQL-related using the LLM.
func (c *OllamaClient) IsSQLQuery(question string) (bool, error) {
	messages := []Message{
		{
			Role: "system",
			Content: `You are a query classifier. Given a user question, determine if it requires a SQL query to answer.
Respond with ONLY "yes" or "no" (lowercase). No other text.

A question is SQL-related if:
- It asks about data in a database (tables, records, counts, sums, etc.)
- It asks about employees, products, orders, sales, inventory, or any structured data
- It asks for statistics, aggregations, or comparisons from database records

A question is NOT SQL-related if:
- It asks about general knowledge, explanations, text documents, or files
- It asks for creative writing, summaries of documents, or general advice`,
		},
		{
			Role:    "user",
			Content: question,
		},
	}

	response, err := c.Chat(messages)
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return strings.HasPrefix(response, "yes"), nil
}
