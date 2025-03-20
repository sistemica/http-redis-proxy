# Makefile for Redis Echo Server

# Default run - listen on a single topic
run:
	go run .

# Run with fixed topic
run-fixed:
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_TOPICS=incoming-messages \
	go run .

# Run with multiple specific topics
run-multi:
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_TOPICS=incoming-messages,api:users,api:orders \
	go run .

# Run with wildcard pattern (listen to all topics)
run-all:
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_TOPICS=* \
	USE_PATTERN=true \
	go run .

# Run with wildcard pattern for all api: topics
run-api:
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_TOPICS=api: \
	USE_PATTERN=true \
	go run .

# Run with debug logging
run-debug:
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_TOPICS=incoming-messages \
	DEBUG=true \
	go run .

# Run with response delay (simulating slow service)
run-delay:
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_TOPICS=incoming-messages \
	RESPONSE_DELAY_MS=500 \
	go run .

# Run with all debug options (slow, verbose)
run-debug-all:
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_TOPICS=* \
	USE_PATTERN=true \
	RESPONSE_DELAY_MS=1000 \
	DEBUG=true \
	go run .

# Build the echo server
build:
	go build -o redis-echo-server

# Clean build files
clean:
	rm -f redis-echo-server

# Help target
help:
	@echo "Redis Echo Server - Makefile"
	@echo ""
	@echo "Available commands:"
	@echo "  make run             Run with default settings (single topic)"
	@echo "  make run-fixed       Run with fixed topic 'incoming-messages'"
	@echo "  make run-multi       Run with multiple specific topics"
	@echo "  make run-all         Run listening to all Redis topics (wildcard)"
	@echo "  make run-api         Run listening to all api: prefixed topics"
	@echo "  make run-debug       Run with debug logging"
	@echo "  make run-delay       Run with 500ms response delay"
	@echo "  make run-debug-all   Run with all debug options (full wildcard, slow, verbose)"
	@echo "  make build           Build the echo server"
	@echo "  make clean           Clean build files"
	@echo "  make help            Display this help message"

.PHONY: run run-fixed run-multi run-all run-api run-debug run-delay run-debug-all build clean help