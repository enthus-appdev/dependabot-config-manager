# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Version=$(git describe --tags --always)" -o dependabot-sync ./cmd/dependabot-sync

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S appuser && \
    adduser -u 1000 -S appuser -G appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/dependabot-sync /usr/local/bin/dependabot-sync

# Copy configuration templates
COPY --from=builder /app/configs /app/configs

# Create reports directory
RUN mkdir -p /app/reports && \
    chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Default command
ENTRYPOINT ["dependabot-sync"]
CMD ["--help"]