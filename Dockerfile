# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install required dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY *.go ./

# Build the application with SQLite support
RUN CGO_ENABLED=1 GOOS=linux go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o redis-proxy .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS and SQLite libraries
RUN apk --no-cache add ca-certificates sqlite-libs

# Create directory for storing database
RUN mkdir -p /app/data

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/redis-proxy .

# Expose proxy and dashboard ports
EXPOSE 8080 8081

# Set up entrypoint
ENTRYPOINT ["./redis-proxy"]