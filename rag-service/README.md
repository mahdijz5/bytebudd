# RAG Backend Service

A Go backend service for Retrieval Augmented Generation (RAG) that integrates with Ollama (local LLM) and Qdrant (vector database). This service works alongside the existing Python text-to-SQL backend, handling document-based queries while routing SQL-related queries back to the Python service.

## Features

- **File Upload**: Parse PDF, TXT, and DOCX files
- **Text Chunking**: Configurable fixed-size chunking with overlap
- **Vector Embeddings**: Generate embeddings using Ollama (nomic-embed-text, all-minilm, etc.)
- **Vector Search**: Store and search chunks in Qdrant vector database
- **Smart Query Routing**: Automatically classifies queries as SQL-related or document-related
- **CORS Enabled**: Ready for frontend integration

## Architecture

```
User Query
    │
    ▼
┌─────────────────┐
│  Query Classifier│
│  (Ollama LLM)    │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
 SQL?      Document?
    │         │
    ▼         ▼
Python    ┌──────────┐
Backend   │ Embedding │
          └────┬─────┘
               │
               ▼
          ┌──────────┐
          │  Qdrant  │
          │ (Search) │
          └────┬─────┘
               │
               ▼
          ┌──────────┐
          │  Answer  │
          │ Generator│
          └──────────┘
```

## Prerequisites

- Go 1.21+
- Ollama running on localhost:11434
- Qdrant running on localhost:6334
- Required Ollama models (pull before use):
  - Embedding model: `ollama pull nomic-embed-text`
  - Chat model: `ollama pull llama3`

## Quick Start

### 1. Clone and Install Dependencies

```bash
cd rag-service
go mod download
```

### 2. Configure Environment

Copy the example environment file and adjust settings:

```bash
cp .env.example .env
```

Key configuration options:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8081` | Service port |
| `QDRANT_HOST` | `localhost` | Qdrant host |
| `QDRANT_PORT` | `6334` | Qdrant port |
| `OLLAMA_URL` | `http://localhost:11434` | Ollama base URL |
| `EMBEDDING_MODEL` | `nomic-embed-text` | Embedding model name |
| `CHAT_MODEL` | `llama3` | Chat/model name |
| `VECTOR_SIZE` | `768` | Embedding vector dimension |
| `COLLECTION_NAME` | `documents` | Qdrant collection name |
| `CORS_ORIGINS` | `http://localhost,http://localhost:3000` | Allowed CORS origins |
| `CHUNK_SIZE` | `500` | Text chunk size (characters) |
| `CHUNK_OVERLAP` | `100` | Overlap between chunks |
| `TOP_K` | `3` | Number of similar chunks to retrieve |

### 3. Run the Service

```bash
go run main.go
```

Or build and run:

```bash
go build -o rag-service
./rag-service
```

### 4. Verify Health

```bash
curl http://localhost:8081/health
```

Expected response:
```json
{
  "status": "ok",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

## API Endpoints

### Health Check

```bash
GET /health
```

Response:
```json
{
  "status": "ok",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### Upload File

```bash
POST /upload
Content-Type: multipart/form-data

Form field: file (PDF, TXT, or DOCX)
```

Response:
```json
{
  "success": true,
  "file_id": "upload_my_doc_txt_20240101120000",
  "filename": "my_doc.txt",
  "chunks_count": 15,
  "message": "File processed successfully"
}
```

### Query Document

```bash
POST /query
Content-Type: application/json

{
  "query": "What is the company's return policy?"
}
```

Response (RAG query):
```json
{
  "is_sql_query": false,
  "answer": "Based on the provided documents, the company offers a 30-day return policy...",
  "sources": [
    {
      "content": "The company offers a 30-day return policy for all unused items...",
      "filename": "policies.pdf",
      "score": 0.92
    },
    {
      "content": "Returns must be initiated within 30 days of purchase...",
      "filename": "policies.pdf",
      "score": 0.87
    }
  ]
}
```

Response (SQL query):
```json
{
  "is_sql_query": true
}
```

## Integration with React Frontend

The service is designed to work alongside your existing Python backend. In your React chatbot UI:

1. **Document RAG Tab** → Call this Go backend (`http://localhost:8081`)
2. **SQL Chat Tab** → Call the Python backend (`http://localhost:8000`)

Example fetch call:

```javascript
// Upload a file
const formData = new FormData();
formData.append('file', file);

const uploadResponse = await fetch('http://localhost:8081/upload', {
  method: 'POST',
  body: formData,
});

// Query the RAG service
const queryResponse = await fetch('http://localhost:8081/query', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ query: 'Your question here' }),
});

const data = await queryResponse.json();

if (data.is_sql_query) {
  // Route to Python backend
  await fetch('http://localhost:8000/query', { ... });
} else {
  // Display RAG answer
  console.log(data.answer);
}
```

## Running the Full Application

### Option 1: Running with Docker (Recommended for Production)

#### Step 1: Pull required Ollama models (on your host machine)
```powershell
ollama pull nomic-embed-text
ollama pull llama3
```

#### Step 2: Start all services with Docker Compose
```powershell
# From project root
docker-compose up -d
```

This starts:
- `bytebudd-postgres` - PostgreSQL database
- `bytebudd-mysql` - Sample MySQL database
- `bytebudd-backend` - Python FastAPI backend (text-to-SQL)
- `bytebudd-frontend` - Next.js frontend
- `bytebudd-qdrant` - Qdrant vector database
- `bytebudd-rag-service` - Go RAG service

#### Step 3: Verify all services are running
```powershell
docker-compose ps
```

#### Step 4: View logs
```powershell
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f rag-service
docker-compose logs -f backend
docker-compose logs -f frontend
```

#### Step 5: Stop all services
```powershell
docker-compose down
# To also remove volumes (warning: deletes data)
docker-compose down -v
```

#### Step 6: Stop only RAG-related services
```powershell
docker-compose stop rag-service qdrant
docker-compose start rag-service qdrant
```

---

### Option 2: Running without Docker (Development)

#### Step 1: Install prerequisites
- Go 1.21+
- Ollama (https://ollama.ai)
- Qdrant (run separately or use Docker just for Qdrant)

#### Step 2: Start Qdrant (if not using Docker)
```powershell
# Option A: Using Docker for Qdrant only
docker run -d --name qdrant -p 6333:6333 -p 6334:6334 qdrant/qdrant:1.11-alpine

# Option B: Using Qdrant binary
# Download from https://qdrant.tech/documentation/quick-start/
```

#### Step 3: Pull Ollama models
```powershell
ollama pull nomic-embed-text
ollama pull llama3
```

#### Step 4: Start Python backend (if not using Docker)
```powershell
cd backend
pip install -r requirements.txt
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

#### Step 5: Start Next.js frontend (if not using Docker)
```powershell
cd frontend
npm install
npm run dev
```

#### Step 6: Install Go dependencies
```powershell
cd rag-service

# Set Go proxy if you're in a restricted network
# For China:
$env:GOPROXY="https://goproxy.cn,direct"

# For other regions:
$env:GOPROXY="https://proxy.golang.org,direct"

go mod download
go mod tidy
```

#### Step 7: Configure environment
```powershell
cd rag-service
cp .env.example .env
# Edit .env if needed
```

#### Step 8: Run the RAG service
```powershell
cd rag-service

# Run directly
go run main.go

# Or build and run
go build -o rag-service.exe
.\rag-service.exe
```

#### Step 9: Verify all services
```powershell
# RAG Service
curl http://localhost:8081/health

# Python Backend
curl http://localhost:8000/docs

# Frontend
# Open http://localhost:3000
```

---

## Running Only the RAG Service

### With Docker
```powershell
cd rag-service

# Build and run standalone
docker build -t rag-service .
docker run -p 8081:8081 `
  -e QDRANT_HOST=localhost `
  -e QDRANT_PORT=6334 `
  -e OLLAMA_URL=http://host.docker.internal:11434 `
  -e EMBEDDING_MODEL=nomic-embed-text `
  -e CHAT_MODEL=llama3 `
  rag-service
```

### Without Docker
```powershell
cd rag-service

# Ensure Qdrant is running on localhost:6334
# Ensure Ollama is running on localhost:11434

go run main.go
```

## Project Structure

```
rag-service/
├── main.go           # Entry point and server setup
├── config.go         # Environment configuration
├── ollama.go         # Ollama client (embeddings + chat)
├── qdrant.go         # Qdrant client (storage + search)
├── parser.go         # File parsers (PDF/TXT/DOCX)
├── chunker.go        # Text chunking logic
├── handler.go        # HTTP handlers (upload, query, health)
├── go.mod            # Go module definition
├── .env.example      # Environment variable template
└── README.md         # This file
```

## Troubleshooting

- **"Failed to connect to Qdrant"**: Ensure Qdrant is running (`docker run -p 6334:6334 qdrant/qdrant`)
- **"Ollama embeddings API returned status: 404"**: Pull the embedding model (`ollama pull nomic-embed-text`)
- **"Collection already exists"**: This is normal on subsequent runs; the service skips creation if the collection exists