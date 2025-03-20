# Redis HTTP Proxy with Dashboard
# This Makefile always enables the dashboard on port 8081

# Default configuration - run both proxy and dashboard
run:
	PORT=8080 \
	DASHBOARD_PORT=8081 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	RESPONSE_TIMEOUT=30 \
	DB_LOG_PATH=./proxy-logs.db \
	DB_MAX_ENTRIES=1000 \
	go run .

# Run with debug logging enabled
run-debug:
	PORT=8080 \
	DASHBOARD_PORT=8081 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	RESPONSE_TIMEOUT=30 \
	DB_LOG_PATH=./proxy-logs.db \
	DB_MAX_ENTRIES=1000 \
	DEBUG=true \
	DASHBOARD_DEBUG=true \
	go run .

# Run with fixed topic
run-fixed-topic:
	PORT=8080 \
	DASHBOARD_PORT=8081 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	FIXED_TOPIC=incoming-messages \
	RESPONSE_TIMEOUT=30 \
	DB_LOG_PATH=./proxy-logs.db \
	DB_MAX_ENTRIES=1000 \
	go run .

# Run in asynchronous mode (fire-and-forget)
run-async:
	PORT=8080 \
	DASHBOARD_PORT=8081 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	RESPOND_IMMEDIATELY_STATUS_CODE=201 \
	DB_LOG_PATH=./proxy-logs.db \
	DB_MAX_ENTRIES=1000 \
	go run .

# Run on different ports
run-alt-ports:
	PORT=9090 \
	DASHBOARD_PORT=9091 \
	REDIS_ADDR=localhost:6379 \
	REDIS_PASSWORD= \
	REDIS_DB=0 \
	RESPONSE_TIMEOUT=30 \
	DB_LOG_PATH=./proxy-logs.db \
	DB_MAX_ENTRIES=1000 \
	go run .

# Build the application
build:
	go build -o redis-proxy .

# Clean build artifacts
clean:
	rm -f redis-proxy proxy-logs.db

# Display help information
help:
	@echo "Redis HTTP Proxy with Dashboard - Makefile"
	@echo ""
	@echo "Available commands:"
	@echo "  make run              Run proxy (port 8080) and dashboard (port 8081)"
	@echo "  make run-debug        Run with debug logging enabled"
	@echo "  make run-fixed-topic  Run with fixed topic 'incoming-messages'"
	@echo "  make run-async        Run in asynchronous mode (fire-and-forget)"
	@echo "  make run-alt-ports    Run on ports 9090 (proxy) and 9091 (dashboard)"
	@echo "  make build            Build the application"
	@echo "  make clean            Clean build artifacts"
	@echo "  make help             Display this help information"

.PHONY: run run-debug run-fixed-topic run-async run-alt-ports build clean help