# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o redis-echo-service .

# Final stage
FROM alpine:3.18

# Install certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/redis-echo-service .

# Set environment variables (these can be overridden at runtime)
ENV REDIS_ADDR=localhost:6379 \
    REDIS_PASSWORD="" \
    REDIS_TOPIC=incoming-messages

# Expose any necessary ports (if applicable)
# EXPOSE 8080

# Run the application
CMD ["./redis-echo-service"]