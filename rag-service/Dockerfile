# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

# 1. Copy BOTH mod and sum files
COPY go.mod go.sum* ./

# 2. Download dependencies directly (no tidy needed)
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o rag-service .

# Runtime stage
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/rag-service .

EXPOSE 8081

CMD ["./rag-service"]