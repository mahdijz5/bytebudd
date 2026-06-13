# Local Development Setup

This guide explains how to run ByteBudd with local backend and frontend services, while keeping database services in Docker for faster development iteration.

## Prerequisites

- **Docker Desktop** - for running PostgreSQL, Qdrant
- **Python 3.8+** - for running the FastAPI backend
- **Node.js 20+** - for running the Next.js frontend
- **Ollama** (optional) - if you want to use RAG/chat features

## Quick Start

### 1. Start Infrastructure

```batch
start-dev.bat
```

This starts:
- **PostgreSQL** on `localhost:5432`
- **Qdrant** on `localhost:6333` (REST) and `localhost:6334` (gRPC)
- **pgadmin** on `localhost:5050` (optional database UI)

### 1. Start RAG Service (Go) - **OPTIONAL**

If you need RAG features (document search, chat with documents):

```batch
cd rag-service
go mod download
go run .
```

The RAG service will be available at `http://localhost:8081`

**Note:** Make sure you have Go installed (1.21+) and Ollama models pulled:
```batch
ollama pull qwen3:4b
ollama pull embeddinggemma
```

### 2. Start Backend (New Terminal)

```batch
cd backend
copy .env.dev .env
pip install -r requirements.txt
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

The backend will be available at `http://localhost:8000`

### 3. Start Frontend (New Terminal)

```batch
cd frontend
npm install
npm run dev
```

The frontend will be available at `http://localhost:3000`

## Access Points

| Service | URL | Description |
|---------|-----|-------------|
| Frontend | http://localhost:3000 | Next.js application |
| Backend API | http://localhost:8000 | FastAPI backend |
| API Docs | http://localhost:8000/docs | Swagger UI |
| RAG Service | http://localhost:8081 | Go RAG service (if running) |
| pgadmin | http://localhost:5050 | Database management |
| Qdrant Dashboard | http://localhost:6333/dashboard | Vector DB dashboard |

## Stopping Services

### Stop Infrastructure Only

```batch
stop-dev.bat
```

### Stop Everything

1. Press `Ctrl+C` in the backend terminal
2. Press `Ctrl+C` in the frontend terminal
3. Run `stop-dev.bat` to stop Docker services

## Ollama Configuration

If Ollama is installed **locally** on Windows:
```
OLLAMA_BASE_URL=http://localhost:11434
```

If Ollama runs in **Docker**:
```
OLLAMA_BASE_URL=http://host.docker.internal:11434
```

Update `backend/.env` accordingly.

## Troubleshooting

### PostgreSQL Connection Refused
- Make sure Docker is running
- Run `docker-compose -f docker-compose-dev.yml ps` to check containers
- Run `start-dev.bat` to restart infrastructure

### Backend Import Errors
- Ensure you're in the `backend` directory
- Run `pip install -r requirements.txt`
- Check that `.env` file exists (copy from `.env.dev`)

### Frontend Module Not Found
- Ensure you're in the `frontend` directory
- Run `npm install` to install dependencies
- Check that `.env.local` exists

### Port Already in Use
- Check if another service is using port 8000, 3000, or 5432
- Use `netstat -ano | findstr :PORT` to find the process
- Kill the process or change the port in config

## File Structure

```
bytebudd/
├── docker-compose-dev.yml    # Docker infrastructure only
├── start-dev.bat             # Start script
├── stop-dev.bat              # Stop script
├── backend/
│   ├── .env.dev              # Backend dev environment
│   ├── .env                  # Active environment (copy from .env.dev)
│   └── ...
└── frontend/
    ├── .env.local            # Frontend dev environment
    └── ...