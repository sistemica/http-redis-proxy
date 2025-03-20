# Redis HTTP Async Proxy System

A complete HTTP-to-Redis communication system with a flexible proxy and echo server implementation.

## System Components

1. **HTTP Redis Proxy** - Converts HTTP requests to Redis messages and waits for responses
2. **Redis Echo Server** - Backend service that listens on Redis topics and responds to messages
3. **Redis** - Message broker that connects the components

## Features

### HTTP Redis Proxy
- Converts HTTP requests to Redis pub/sub messages
- Supports both fixed and path-based topic routing
- Works in synchronous or asynchronous (fire-and-forget) mode
- Comprehensive debug logging
- Race-condition safe subscription handling

### Redis Echo Server
- Listens on single or multiple Redis topics
- Supports pattern-based subscriptions (wildcards)
- Configurable response delays for testing
- Debug logging
- Flexible response format

## Running the System

### Using Docker Compose

The easiest way to run the complete system is with Docker Compose:

```bash
docker-compose up -d
```

This starts:
- Redis instance
- HTTP Proxy (configured in docker-compose.yml)
- Echo Server (configured in docker-compose.yml)

### Running Components Individually

#### HTTP Proxy

```bash
# Sync mode with path-based topics
make run-sync

# Sync mode with fixed topic
make run-sync-fixed

# Async mode
make run-async

# With debug logging
make run-sync-debug
make run-sync-fixed-debug
```

#### Echo Server

```bash
cd sample-backend

# Listen on a fixed topic
make run-fixed

# Listen on multiple topics
make run-multi

# Listen on all topics (wildcard)
make run-all

# Listen on all API topics
make run-api

# With debug logging
make run-debug

# With response delay
make run-delay
```

## Testing the System

### Using curl

```bash
# Test with path-based routing
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"test":"data"}' \
  http://localhost:8080/api/users

# Test with query parameters
curl -X GET \
  "http://localhost:8080/api/search?term=test&limit=10"

# Test with custom headers
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: abc123" \
  -d '{"name":"test"}' \
  http://localhost:8080/api/items
```

### Using the Test Client

The repository includes a test client in the `send` directory:

```bash
cd send
go run main.go
```

## Configuration

### HTTP Redis Proxy

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `REDIS_ADDR` | Redis server address | localhost:6379 |
| `REDIS_PASSWORD` | Redis password | "" |
| `REDIS_DB` | Redis database number | 0 |
| `PORT` | HTTP server port | 8080 |
| `FIXED_TOPIC` | If set, uses this topic for all messages | "" |
| `RESPOND_IMMEDIATELY_STATUS_CODE` | If set, returns immediately with this status code | "" |
| `RESPONSE_TIMEOUT` | Timeout in seconds to wait for a response | 30 |
| `DEBUG` | Enable detailed debug logging | false |

### Echo Server

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `REDIS_ADDR` | Redis server address | localhost:6379 |
| `REDIS_PASSWORD` | Redis password | "" |
| `REDIS_TOPICS` | Comma-separated list of topics to listen on | incoming-messages |
| `USE_PATTERN` | Enable pattern matching for topics | false |
| `RESPONSE_DELAY_MS` | Artificial delay before responding | 0 |
| `DEBUG` | Enable detailed debug logging | false |

## Message Format

### Published to Redis:
```json
{
  "header": {
    "Content-Type": "application/json",
    "path": "/api/resource",
    "query_param1": "value1",
    "response_topic": "api:resource:response:uuid"
  },
  "body": { 
    // Original HTTP request body
  }
}
```

### Echo Server Response:
```json
{
  "body": {
    "status": "success",
    "message": "Echo response",
    "original_channel": "api:resource",
    "original_header": {
      // Original request headers
    },
    "original_body": {
      // Original request body
    },
    "timestamp": "2025-03-20T16:30:00Z"
  }
}
```

## Architecture

```
┌─────────────┐      HTTP      ┌──────────────┐
│  HTTP       │◄──────────────►│  Redis HTTP  │
│  Client     │                │  Proxy       │
└─────────────┘                └──────────────┘
                                      │
                                      │ Redis Pub/Sub
                                      ▼
                               ┌──────────────┐
                               │  Redis       │
                               │  Broker      │
                               └──────────────┘
                                      │
                                      │ Redis Pub/Sub
                                      ▼
                               ┌──────────────┐
                               │  Echo Server │
                               │  Backend     │
                               └──────────────┘
```

## Operation Modes

### Synchronous Mode
If `RESPOND_IMMEDIATELY_STATUS_CODE` is not set, the proxy:
1. Receives an HTTP request
2. Subscribes to a unique response topic
3. Converts the request to a Redis message
4. Publishes the message to the appropriate topic
5. Waits for a response on the unique topic
6. Returns the response body to the HTTP client

### Asynchronous Mode (Fire-and-Forget)
If `RESPOND_IMMEDIATELY_STATUS_CODE` is set (e.g., to 201), the proxy:
1. Receives an HTTP request
2. Converts it to a Redis message
3. Publishes the message to the appropriate topic
4. Immediately responds with the configured status code
5. Does not wait for any response from Redis