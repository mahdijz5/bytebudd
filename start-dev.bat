@echo off
echo ============================================
echo   ByteBudd Local Development Setup
echo ============================================
echo.

REM Check if Docker is running
docker info >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Docker is not running. Please start Docker Desktop first.
    pause
    exit /b 1
)
echo [OK] Docker is running

REM Start infrastructure containers
echo.
echo Starting infrastructure services (PostgreSQL, Qdrant, pgadmin)...
docker-compose -f docker-compose-dev.yml up -d

echo.
echo Waiting for services to be ready...
timeout /t 10 /nobreak >nul

REM Check if PostgreSQL is ready
echo Checking PostgreSQL...
docker exec bytebudd-postgres pg_isready -U bytebudd >nul 2>&1
if errorlevel 1 (
    echo [WARNING] PostgreSQL may not be ready yet. Waiting...
    timeout /t 10 /nobreak >nul
)

echo.
echo ============================================
echo   Infrastructure Running!
echo ============================================
echo.
echo   - PostgreSQL:   localhost:5432
echo   - Qdrant:       localhost:6333, localhost:6334
echo   - pgadmin:      localhost:5050
echo.
echo   Next steps (open NEW terminals for each):
echo.
echo   1. RAG Service (Go) - OPTIONAL, skip if not using RAG:
echo      cd rag-service
echo      go mod download
echo      go run .
echo.
echo   2. Backend (Python):
echo      cd backend
echo      copy .env.dev .env
echo      pip install -r requirements.txt
echo      uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
echo.
echo   3. Frontend (Node.js):
echo      cd frontend
echo      npm install
echo      npm run dev
echo.
echo   4. Access the application:
echo      - Frontend:      http://localhost:3000
echo      - Backend API:   http://localhost:8000
echo      - RAG Service:   http://localhost:8081
echo      - pgadmin:       http://localhost:5050
echo.
echo ============================================
echo.
echo To stop infrastructure, run: stop-dev.bat
echo.
pause