package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port                int
	QdrantHost          string
	QdrantPort          int
	OllamaURL           string
	EmbeddingModel      string
	ChatModel           string
	VectorSize          int
	CollectionName      string
	CORSOrigins         []string
	ChunkSize           int
	ChunkOverlap        int
	TopK                int
	PostgresHost        string
	PostgresPort        int
	PostgresUser        string
	PostgresPassword    string
	PostgresDB          string
	DisableTLS          bool // set true for local dev (docker uses internal network)
	DocumentStoragePath string
}

// loadConfig reads configuration from environment variables with sensible defaults.
func loadConfig() *Config {
	config := &Config{
		Port:                getEnvInt("PORT", 8081),
		QdrantHost:          getEnvString("QDRANT_HOST", "localhost"),
		QdrantPort:          getEnvInt("QDRANT_PORT", 6334),
		OllamaURL:           getEnvString("OLLAMA_URL", "http://localhost:11434"),
		EmbeddingModel:      getEnvString("EMBEDDING_MODEL", "embeddinggemma"),
		ChatModel:           getEnvString("CHAT_MODEL", "qwen3:4b"),
		VectorSize:          getEnvInt("VECTOR_SIZE", 768),
		CollectionName:      getEnvString("COLLECTION_NAME", "documents"),
		ChunkSize:           getEnvInt("CHUNK_SIZE", 1000),
		ChunkOverlap:        getEnvInt("CHUNK_OVERLAP", 200),
		TopK:                getEnvInt("TOP_K", 3),
		PostgresHost:        getEnvString("POSTGRES_HOST", "localhost"),
		PostgresPort:        getEnvInt("POSTGRES_PORT", 5432),
		PostgresUser:        getEnvString("POSTGRES_USER", "bytebudd"),
		PostgresPassword:    getEnvString("POSTGRES_PASSWORD", "bytebudd_secret"),
		PostgresDB:          getEnvString("POSTGRES_DB", "bytebudd"),
		DisableTLS:          getEnvBool("DISABLE_TLS", true),
		DocumentStoragePath: getEnvString("DOCUMENT_STORAGE_PATH", "/app/data/documents"),
	}

	// Parse CORS origins from comma-separated string
	if origins := getEnvString("CORS_ORIGINS", "http://localhost,http://localhost:3000"); origins != "" {
		config.CORSOrigins = strings.Split(origins, ",")
		for i, origin := range config.CORSOrigins {
			config.CORSOrigins[i] = strings.TrimSpace(origin)
		}
	}

	return config
}

// getEnvString returns the environment variable value or the default if not set.
func getEnvString(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns the environment variable as an integer or the default if not set.
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvBool returns the environment variable as a boolean or the default if not set.
func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getDSN returns a PostgreSQL DSN string for database connection.
func (c *Config) getDSN() string {
	sslMode := "require"
	if c.DisableTLS {
		sslMode = "disable"
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.PostgresHost, c.PostgresPort, c.PostgresUser, c.PostgresPassword, c.PostgresDB, sslMode)
}

// getCORSConfig returns the Gin CORS configuration based on loaded settings.
func (c *Config) getCORSConfig() gin.H {
	return gin.H{
		"allowOrigins":     c.CORSOrigins,
		"allowMethods":     []string{"GET", "POST", "OPTIONS"},
		"allowHeaders":     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		"exposeHeaders":    []string{"Content-Length"},
		"allowCredentials": true,
		"maxAge":           12 * 60 * 60, // 12 hours
	}
}
