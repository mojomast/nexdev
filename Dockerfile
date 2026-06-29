# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /app/bin/geoffrussy ./cmd/geoffrussy

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite-libs git

# Create non-root user
RUN addgroup -g 1000 geoffrey && \
    adduser -D -u 1000 -G geoffrey geoffrey

# Set working directory
WORKDIR /home/geoffrey

# Copy binary from builder
COPY --from=builder /app/bin/geoffrussy /usr/local/bin/geoffrussy

# Change ownership
RUN chown -R geoffrey:geoffrey /home/geoffrey

# Switch to non-root user
USER geoffrey

# Set entrypoint
ENTRYPOINT ["geoffrussy"]
CMD ["--help"]
