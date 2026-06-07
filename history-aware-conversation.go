package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

const (
	collectionName = "company_docs"
	ollamaBaseURL  = "http://localhost:11434"
	embedModel     = "embeddinggemma"
	chatModel      = "qwen3:4b"  
)

// Message holds chat roles ("system", "user", "assistant") and content strings
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

type OllamaChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type OllamaChatResponse struct {
	Message Message `json:"message"`
}

// Global variable tracking the active chat memory across turns
var chatHistory []Message

func main() {
	ctx := context.Background()

	// 1. Connect to Qdrant via official gRPC client
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer client.Close()

	// 2. Launch the standard console conversation loop
	fmt.Println("Ask me questions! Type 'quit' to exit.")
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\nYour question: ")
		if !scanner.Scan() {
			break
		}
		question := strings.TrimSpace(scanner.Text())

		if strings.ToLower(question) == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		if question == "" {
			continue
		}

		askQuestion(ctx, client, question)
	}
}

func askQuestion(ctx context.Context, client *qdrant.Client, userQuestion string) string {
	fmt.Printf("\n--- You asked: %s ---\n", userQuestion)
	searchQuestion := userQuestion

	// Step 1: Query Condensation (If history exists, rewrite follow-up question)
	if len(chatHistory) > 0 {
		var condenseMessages []Message
		condenseMessages = append(condenseMessages, Message{
			Role:    "system",
			Content: "Given the chat history, rewrite the new question to be standalone and searchable. Just return the rewritten question without commentary.",
		})
		condenseMessages = append(condenseMessages, chatHistory...)
		condenseMessages = append(condenseMessages, Message{
			Role:    "user",
			Content: fmt.Sprintf("New question: %s", userQuestion),
		})

		rewrittenQuestion, err := callOllamaChat(condenseMessages)
		if err != nil {
			log.Fatalf("Query condensation error: %v", err)
		}
		searchQuestion = strings.TrimSpace(rewrittenQuestion)
		fmt.Printf("Searching for: %s\n", searchQuestion)
	}

	// Step 2: Vector Search Query Translation
	queryVector, err := getOllamaEmbedding(searchQuestion)
	if err != nil {
		log.Fatalf("Failed to extract query embedding array: %v", err)
	}

	limitK := uint64(3) // Top 3 context chunks
	searchResult, err := client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collectionName,
		Query:          qdrant.NewQueryDense(queryVector),
		Limit:          &limitK,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		log.Fatalf("Document database lookup failed: %v", err)
	}

	fmt.Printf("Found %d relevant documents:\n", len(searchResult))
	var retrievedTexts []string
	for i, point := range searchResult {
		content := point.Payload["page_content"].GetStringValue()
		
		retrievedTexts = append(retrievedTexts, fmt.Sprintf("- %s", content))

		// Doc Preview: isolate and display the first 2 lines
		lines := strings.Split(content, "\n")
		previewLines := lines
		if len(lines) > 2 {
			previewLines = lines[:2]
		}
		fmt.Printf("  Doc %d: %s...\n", i+1, strings.Join(previewLines, "\n"))
	}

	// Step 3: Package Context Documents and original text question
	documentsContext := strings.Join(retrievedTexts, "\n")
	combinedInput := fmt.Sprintf(`Based on the following documents, please answer this question: %s

Documents:
%s

Please provide a clear, helpful answer using only the information from these documents. If you can't find the answer in the documents, say "I don't have enough information to answer that question based on the provided documents."`, userQuestion, documentsContext)

	// Step 4: Call Chat Inference including system instructions and chat memory context
	var finalMessages []Message
	finalMessages = append(finalMessages, Message{
		Role:    "system",
		Content: "You are a helpful assistant that answers questions based on provided documents and conversation history.",
	})
	finalMessages = append(finalMessages, chatHistory...)
	finalMessages = append(finalMessages, Message{
		Role:    "user",
		Content: combinedInput,
	})

	answer, err := callOllamaChat(finalMessages)
	if err != nil {
		log.Fatalf("Final context calculation step failed: %v", err)
	}

	// Step 5: Append user query and clean answer text down into chat memory storage
	chatHistory = append(chatHistory, Message{Role: "user", Content: userQuestion})
	chatHistory = append(chatHistory, Message{Role: "assistant", Content: answer})

	fmt.Printf("Answer: %s\n", answer)
	return answer
}

func getOllamaEmbedding(prompt string) ([]float32, error) {
	reqBody, _ := json.Marshal(OllamaEmbedRequest{
		Model:  embedModel,
		Prompt: prompt,
	})

	resp, err := http.Post(ollamaBaseURL+"/api/embeddings", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var embedResp OllamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, err
	}
	return embedResp.Embedding, nil
}

func callOllamaChat(messages []Message) (string, error) {
	reqBody, _ := json.Marshal(OllamaChatRequest{
		Model:    chatModel,
		Messages: messages,
		Stream:   false,
	})

	client := &http.Client{Timeout: 600 * time.Second}
	resp, err := client.Post(ollamaBaseURL+"/api/chat", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var chatResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}
	return chatResp.Message.Content, nil
}