package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaClient handles all communication with the Ollama API.
type OllamaClient struct {
	BaseURL      string
	EmbeddingModel string
	ChatModel    string
	httpClient   *http.Client
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
func (c *OllamaClient) GetEmbedding(text string) ([]float32, error) {
	reqBody, err := json.Marshal(OllamaEmbedRequest{
		Model:  c.EmbeddingModel,
		Prompt: text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	resp, err := c.httpClient.Post(c.BaseURL+"/api/embeddings", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama embeddings API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama embeddings API returned status: %s", resp.Status)
	}

	var embedResp OllamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if len(embedResp.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned from Ollama")
	}

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
			Role:    "system",
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

	response = bytes.TrimSpace([]byte(response))
	return bytes.EqualFold(response, []byte("yes")), nil
}