# Redis HTTP Proxy with Dashboard

A comprehensive HTTP-to-Redis communication system with integrated monitoring dashboard and flexible backend configuration options. This system decouples HTTP requests from backend processing using Redis pub/sub, enabling more flexible and scalable architectures.

## System Architecture

This system consists of the following core components:

1. **HTTP Redis Proxy** - Serves as an HTTP bridge to Redis pub/sub messaging
2. **Dashboard Interface** - Provides real-time monitoring, logs, and statistics
3. **Redis Echo Server** - Test backend service (for development and testing only)
4. **Redis** - Message broker that connects all components

```
┌───────────┐      HTTP      ┌───────────────┐
│  Client   │◄──────────────►│  Redis HTTP   │
│  Browser  │                │  Proxy        │◄─────┐
└───────────┘                └───────────────┘      │
                                     │               │
                                     │ Redis Pub/Sub │
                                     ▼               │
                              ┌──────────────┐       │ HTTP
                              │  Redis       │       │ Dashboard
                              │  Broker      │       │ Interface
                              └──────────────┘       │
                                     │               │
                                     │ Redis Pub/Sub │
                                     ▼               │
                             ┌───────────────┐       │
                             │ Real Backend  │       │
                             │ or Echo Server│◄──────┘
                             └───────────────┘
```

## Dashboard Screenshots

<div style="display: flex; justify-content: space-between; gap: 10px;">
  <div style="flex: 1; text-align: center;">
    <img src="./docs/screen-1.png" alt="Dashboard Overview" width="100%">
    <p><strong>Dashboard Overview</strong></p>
  </div>
  <div style="flex: 1; text-align: center;">
    <img src="./docs/screen-2.png" alt="Statistics View" width="100%">
    <p><strong>Statistics View</strong></p>
  </div>
  <div style="flex: 1; text-align: center;">
    <img src="./docs/screen-3.png" alt="Logs View" width="100%">
    <p><strong>Logs View</strong></p>
  </div>
</div>


## Features

### HTTP Redis Proxy
- **Flexible Routing**:
  - Path-based topic routing (converts URL paths to Redis topics)
  - Fixed topic support (all requests go to a single topic)
- **Multiple Operation Modes**:
  - Synchronous (waits for response)
  - Asynchronous/Fire-and-forget (immediate response)
- **Robust Error Handling**:
  - Race-condition safe subscription management
  - Configurable timeouts
  - Detailed error reporting
- **Performance Optimizations**:
  - Connection pooling
  - Efficient subscription handling

### Dashboard Interface
- **Real-time Monitoring**:
  - Overview dashboard with key metrics
  - Auto-refreshing statistics
  - Time period filtering (hour, day, week, month, all-time)
- **Detailed Log Viewing**:
  - Full request/response inspection
  - Request body visualization
  - Response timing analysis
  - Error highlighting
- **Statistical Analysis**:
  - Request distribution by status code
  - Topic popularity metrics
  - Response time analysis
  - Success/failure rate tracking

## Benefits of HTTP-to-Redis Decoupling

This proxy was built to decouple HTTP requests from backend processing through Redis, providing several important benefits:

### Advantages

1. **High Availability and Horizontal Scaling**:
   - Backend instances can be added or removed without affecting the client-facing interface
   - No need to reconfigure load balancers when scaling backend services
   - Backends can be distributed across different regions or data centers

2. **Workload Distribution**:
   - Enables effective work distribution across multiple backend workers
   - Supports topic-based routing for specialized processing
   - Each worker can specialize in certain types of requests

3. **Resilience and Fault Tolerance**:
   - Backend failures don't directly impact clients
   - Requests can be persisted in Redis if backends are temporarily unavailable
   - Easier recovery from backend failures

4. **Request Throttling and Buffering**:
   - Handles traffic spikes by buffering requests
   - Prevents backend overload during high traffic situations
   - Provides consistent response times to clients

5. **Simplified Backend Development**:
   - Backend services only need to handle Redis pub/sub, not HTTP
   - Consistent message format across all services
   - Backends can be implemented in any language with Redis support

### Challenges and Considerations

1. **Increased Complexity**:
   - Adds an additional component (Redis) as a dependency
   - Requires monitoring and management of the Redis instance
   - More complex debugging across the entire system

2. **Potential Latency**:
   - Additional network hops can increase overall response time
   - Synchronous mode requires maintaining subscriptions for responses

3. **Message Format Constraints**:
   - All services must adhere to the same message format
   - Data must be serializable (typically as JSON)

4. **State Management**:
   - Stateful operations require additional consideration
   - Session management must be handled separately

## n8n Integration Context

This proxy was specifically designed with [n8n](https://n8n.io/) workflows in mind, enabling:

1. **High-Availability n8n Deployments**:
   - Multiple n8n worker instances can run behind the proxy
   - Redis serves as the communication layer between instances
   - Enables true horizontal scaling of n8n worker nodes

2. **HTTP-to-Redis Workflow Conversion**:
   - Eliminates need for direct HTTP-to-Redis workflows in n8n
   - Provides synchronous behavior over Redis (which n8n didn't natively support)
   - Simplifies workflow development by handling the pub/sub pattern invisibly

3. **Decoupled Execution**:
   - n8n instances can process requests from any frontend without direct connection
   - Worker nodes can be added or removed without frontend reconfiguration
   - Enables specialized n8n instances for specific workflow types

## Getting Started

### Prerequisites

- Go 1.19 or higher
- Redis 5.0 or higher
- SQLite (for logging)

### Installation

#### Using Docker Compose (Recommended)

The easiest way to run the complete system is with Docker Compose:

```bash
# Build and start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

#### Manual Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/yourusername/redis-http-proxy.git
   cd redis-http-proxy
   ```

2. **Build the proxy and dashboard**:
   ```bash
   go build -o redis-proxy .
   ```

3. **Build the echo server** (for testing only):
   ```bash
   cd sample-backend
   go build -o redis-echo-server .
   cd ..
   ```

4. **Run Redis**:
   ```bash
   # If not already running
   redis-server --daemonize yes
   ```

## Running the System

### Configuration Options

#### HTTP Redis Proxy

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `PORT` | HTTP server port | 8080 |
| `DASHBOARD_PORT` | Dashboard interface port | 8081 |
| `REDIS_ADDR` | Redis server address | localhost:6379 |
| `REDIS_PASSWORD` | Redis password | "" |
| `REDIS_DB` | Redis database number | 0 |
| `REDIS_POOL_SIZE` | Connection pool size | 10 |
| `FIXED_TOPIC` | If set, uses this topic for all messages | "" |
| `RESPOND_IMMEDIATELY_STATUS_CODE` | Enables async mode with this status code | "" |
| `RESPONSE_TIMEOUT` | Timeout in seconds to wait for a response | 30 |
| `DEBUG` | Enable detailed debug logging | false |
| `DASHBOARD_DEBUG` | Enable debug logging for dashboard | false |
| `DB_LOG_PATH` | Path to SQLite database for logging | "" |
| `DB_MAX_ENTRIES` | Maximum number of log entries to keep | 1000 |

#### Echo Server (for testing)

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `REDIS_ADDR` | Redis server address | localhost:6379 |
| `REDIS_PASSWORD` | Redis password | "" |
| `REDIS_TOPICS` | Comma-separated list of topics to listen on | incoming-messages |
| `USE_PATTERN` | Enable pattern matching for topics | false |
| `RESPONSE_DELAY_MS` | Artificial delay before responding | 0 |
| `DEBUG` | Enable detailed debug logging | false |

### Using the Makefile

For convenience, several Makefile targets are provided:

```bash
# Run proxy with dashboard in default configuration
make run

# Run with debug logging
make run-debug

# Run with fixed topic
make run-fixed-topic

# Run in asynchronous mode
make run-async

# Run on different ports (9090/9091)
make run-alt-ports

# Display help
make help
```

## Testing the System

The system includes an Echo Server that serves as a testing backend for development and validation. This component simply echoes back the received messages, making it useful for confirming the system is working properly.

### Using curl

```bash
# Test with path-based routing (topic = api:users)
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"name":"test","value":123}' \
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

# Test the async mode
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"test":"async"}' \
  http://localhost:8080/api/async-test
```

### Using the Test Client

The repository includes a test client for sending and receiving test messages:

```bash
cd send
go run main.go
```

### Running Integration Tests

A comprehensive integration test script is included:

```bash
# Make the script executable
chmod +x integration-test.sh

# Run tests
./integration-test.sh
```

## Accessing the Dashboard

Once the system is running, access the dashboard at:

```
http://localhost:8081/dashboard
```

The dashboard provides:

1. **Overview** - High-level system metrics
2. **Statistics** - Detailed performance analysis
3. **Logs** - Complete request/response inspection

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

## Dashboard Architecture

The dashboard server runs alongside the proxy on a separate port and provides:

### 1. Main Dashboard View
- Overview of system metrics and health
- Quick access to logs and statistics

### 2. Statistics View
- Request volume metrics
- Success/failure rates
- Response time analysis
- Status code distribution
- Topic popularity charts

### 3. Logs View
- Complete request/response inspection
- Syntax-highlighted JSON formatting
- Error highlighting
- Filterable by count

### 4. API Endpoints
- `/dashboard/api/logs` - Retrieve log entries
- `/dashboard/api/stats` - Retrieve system statistics

## Implementing Real Backend Services

While the echo server is included for testing, in production you'll want to implement actual backend services. These services will:

1. Subscribe to specific Redis topics
2. Process incoming messages according to business logic
3. Publish responses back to the response topic specified in the message header

### Example Backend (Python)

```python
import redis
import json
import time
import threading

# Connect to Redis
r = redis.Redis(host='localhost', port=6379)
p = r.pubsub()

# Subscribe to a specific topic
p.subscribe('api:users')

# Process messages
for message in p.listen():
    if message['type'] == 'message':
        data = json.loads(message['data'])
        
        # Get response topic from header
        response_topic = data['header']['response_topic']
        
        # Process the request (this is your actual business logic)
        result = process_user_request(data['body'])
        
        # Send back response
        response = {
            "body": {
                "status": "success",
                "result": result,
                "timestamp": time.time()
            }
        }
        
        r.publish(response_topic, json.dumps(response))
```

### n8n Backend Integration Example

For n8n integration, you can set up n8n workers to listen to specific topics:

```javascript
// n8n workflow that listens for messages on a Redis topic
// and processes them automatically

// Redis Trigger node
{
  "parameters": {
    "channels": ["api:workflows"],
    "options": {
      "readExistingData": true
    }
  }
}

// Then connect to your workflow processing nodes
// ...

// Finally, publish result back to Redis
{
  "parameters": {
    "channel": "={{$node['Redis Trigger'].json.header.response_topic}}",
    "value": "={ \"body\": { \"status\": \"success\", \"result\": $node['Process Data'].json } }"
  }
}
```

## Performance Considerations

For high-volume deployments:

1. Increase Redis connection pool size via `REDIS_POOL_SIZE`
2. Use asynchronous mode when immediate responses aren't required
3. Set appropriate timeouts for your workload via `RESPONSE_TIMEOUT`
4. Consider using Redis Cluster for high availability and throughput
5. Monitor using the dashboard and adjust configuration as needed

## Troubleshooting

### Common Issues

1. **Timeouts on Responses**:
   - Check if backend service is running and subscribed to correct topics
   - Verify Redis connectivity for both proxy and backend service
   - Increase `RESPONSE_TIMEOUT` if your backend processing is slow

2. **Database Write Errors**:
   - Ensure the SQLite database is writable by the application
   - Check disk space for the database file
   - Reduce `DB_MAX_ENTRIES` if database is growing too large

3. **Missing Response Data**:
   - Ensure backend is formatting responses correctly
   - Check that response is being published to the correct response topic
   - Verify serialization of complex data types in the response

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the GNU GPL 3 - see the LICENSE file for details.