# Simple Makefile for Redis HTTP Proxy
# Run different configurations with "go run ."

# Default run - synchronous mode with path-based topics
run:
	go run .

# Run in synchronous mode with path-based topics
run-sync:
	PORT=8080 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	RESPONSE_TIMEOUT=30 \
	go run .

run-sync-debug:
	PORT=8080 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	RESPONSE_TIMEOUT=30 \
	DEBUG=true \
	go run .

# Run in asynchronous mode with path-based topics
run-async:
	PORT=8080 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	RESPOND_IMMEDIATELY_STATUS_CODE=201 \
	go run .

# Run in synchronous mode with fixed topic
run-sync-fixed:
	PORT=8080 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	FIXED_TOPIC=incoming-messages \
	RESPONSE_TIMEOUT=30 \
	go run .

# Run in synchronous mode with fixed topic and debug logging
run-sync-fixed-debug:
	PORT=8080 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	FIXED_TOPIC=incoming-messages \
	RESPONSE_TIMEOUT=30 \
	DEBUG=true \
	go run .

# Run in asynchronous mode with fixed topic
run-async-fixed:
	PORT=8080 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	FIXED_TOPIC=incoming-messages \
	RESPOND_IMMEDIATELY_STATUS_CODE=201 \
	go run .

# Run with debug logging
run-debug:
	PORT=8080 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	RESPONSE_TIMEOUT=30 \
	DEBUG=true \
	go run .

# Run on a different port
run-alt-port:
	PORT=9090 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	go run .

# Display help information
help:
	@echo "Redis HTTP Proxy - Simple Makefile"
	@echo ""
	@echo "Available commands:"
	@echo "  make run                  Run in default mode (synchronous with path-based topics)"
	@echo "  make run-sync             Run in synchronous mode with path-based topics"
	@echo "  make run-async            Run in asynchronous mode with path-based topics"
	@echo "  make run-sync-fixed       Run in synchronous mode with fixed topic"
	@echo "  make run-sync-fixed-debug Run in synchronous mode with fixed topic and debug logging"
	@echo "  make run-async-fixed      Run in asynchronous mode with fixed topic"
	@echo "  make run-debug            Run with debug logging enabled"
	@echo "  make run-alt-port         Run on alternate port (9090)"
	@echo "  make help                 Display this help information"

.PHONY: run run-sync run-async run-sync-fixed run-sync-fixed-debug run-async-fixed run-debug run-alt-port help