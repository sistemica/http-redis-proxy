#!/bin/bash
# integration-test.sh - Test the HTTP Redis Proxy with Echo Server

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Redis HTTP Proxy Integration Test${NC}"
echo "=============================="
echo ""

# Function to check if redis is running
check_redis() {
  echo -n "Checking Redis... "
  if redis-cli ping > /dev/null 2>&1; then
    echo -e "${GREEN}Running${NC}"
    return 0
  else
    echo -e "${RED}Not running${NC}"
    echo "Starting Redis..."
    redis-server --daemonize yes
    sleep 1
    if redis-cli ping > /dev/null 2>&1; then
      echo -e "${GREEN}Redis started successfully${NC}"
      return 0
    else
      echo -e "${RED}Failed to start Redis${NC}"
      return 1
    fi
  fi
}

# Function to start the echo server
start_echo_server() {
  echo -n "Starting Echo Server... "
  cd sample-backend
  REDIS_ADDR=localhost:6379 REDIS_TOPICS=* USE_PATTERN=true DEBUG=true go run . > ../echo-server.log 2>&1 &
  ECHO_PID=$!
  cd ..
  sleep 2
  if ps -p $ECHO_PID > /dev/null; then
    echo -e "${GREEN}Started (PID: $ECHO_PID)${NC}"
    return 0
  else
    echo -e "${RED}Failed${NC}"
    return 1
  fi
}

# Function to start the HTTP proxy
start_proxy() {
  echo -n "Starting HTTP Proxy... "
  PORT=8080 REDIS_ADDR=localhost:6379 RESPONSE_TIMEOUT=10 DEBUG=true go run . > proxy.log 2>&1 &
  PROXY_PID=$!
  sleep 2
  if ps -p $PROXY_PID > /dev/null; then
    echo -e "${GREEN}Started (PID: $PROXY_PID)${NC}"
    return 0
  else
    echo -e "${RED}Failed${NC}"
    return 1
  fi
}

# Function to test the proxy with path-based routing
test_path_based() {
  echo -e "\n${YELLOW}Testing Path-Based Routing:${NC}"
  
  echo -n "Sending request to /api/users... "
  RESPONSE=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d '{"name":"test","value":123}' http://localhost:8080/api/users)
  STATUS=$(echo "$RESPONSE" | tail -n1)
  BODY=$(echo "$RESPONSE" | sed '$d')
  
  if [ "$STATUS" = "200" ]; then
    echo -e "${GREEN}Success (Status: $STATUS)${NC}"
    echo "Response: $BODY"
    
    # Verify it contains expected values
    if echo "$BODY" | grep -q "original_channel"; then
      echo -e "${GREEN}✓ Response contains original_channel${NC}"
    else
      echo -e "${RED}✗ Response missing original_channel${NC}"
    fi
    
    if echo "$BODY" | grep -q "api:users"; then
      echo -e "${GREEN}✓ Topic path correctly created${NC}"
    else
      echo -e "${RED}✗ Topic path not created correctly${NC}"
    fi
  else
    echo -e "${RED}Failed (Status: $STATUS)${NC}"
    echo "Response: $BODY"
  fi
}

# Function to test the proxy with fixed topic
test_fixed_topic() {
  echo -e "\n${YELLOW}Testing Fixed Topic:${NC}"
  
  # Stop the previous proxy
  if [ -n "$PROXY_PID" ]; then
    kill $PROXY_PID
    wait $PROXY_PID 2>/dev/null
    echo "Previous proxy stopped"
  fi
  
  # Start new proxy with fixed topic
  echo -n "Starting HTTP Proxy with fixed topic... "
  PORT=8080 REDIS_ADDR=localhost:6379 RESPONSE_TIMEOUT=10 FIXED_TOPIC=incoming-messages DEBUG=true go run . > proxy-fixed.log 2>&1 &
  PROXY_PID=$!
  sleep 2
  
  if ps -p $PROXY_PID > /dev/null; then
    echo -e "${GREEN}Started (PID: $PROXY_PID)${NC}"
  else
    echo -e "${RED}Failed${NC}"
    return 1
  fi
  
  echo -n "Sending request to /any/path... "
  RESPONSE=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d '{"test":"fixed-topic"}' http://localhost:8080/any/path)
  STATUS=$(echo "$RESPONSE" | tail -n1)
  BODY=$(echo "$RESPONSE" | sed '$d')
  
  if [ "$STATUS" = "200" ]; then
    echo -e "${GREEN}Success (Status: $STATUS)${NC}"
    echo "Response: $BODY"
    
    # Verify it contains expected values
    if echo "$BODY" | grep -q "incoming-messages"; then
      echo -e "${GREEN}✓ Fixed topic correctly used${NC}"
    else
      echo -e "${RED}✗ Fixed topic not used correctly${NC}"
    fi
  else
    echo -e "${RED}Failed (Status: $STATUS)${NC}"
    echo "Response: $BODY"
  fi
}

# Function to test the async mode
test_async_mode() {
  echo -e "\n${YELLOW}Testing Async Mode:${NC}"
  
  # Stop the previous proxy
  if [ -n "$PROXY_PID" ]; then
    kill $PROXY_PID
    wait $PROXY_PID 2>/dev/null
    echo "Previous proxy stopped"
  fi
  
  # Start new proxy with async mode
  echo -n "Starting HTTP Proxy in async mode... "
  PORT=8080 REDIS_ADDR=localhost:6379 RESPOND_IMMEDIATELY_STATUS_CODE=201 DEBUG=true go run . > proxy-async.log 2>&1 &
  PROXY_PID=$!
  sleep 2
  
  if ps -p $PROXY_PID > /dev/null; then
    echo -e "${GREEN}Started (PID: $PROXY_PID)${NC}"
  else
    echo -e "${RED}Failed${NC}"
    return 1
  fi
  
  echo -n "Sending request to /api/async-test... "
  RESPONSE=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d '{"test":"async"}' http://localhost:8080/api/async-test)
  STATUS=$(echo "$RESPONSE" | tail -n1)
  BODY=$(echo "$RESPONSE" | sed '$d')
  
  if [ "$STATUS" = "201" ]; then
    echo -e "${GREEN}Success (Status: $STATUS)${NC}"
    
    # Check echo server logs to verify message was received
    sleep 1
    if grep -q "api:async-test" echo-server.log; then
      echo -e "${GREEN}✓ Echo server received the async message${NC}"
    else
      echo -e "${RED}✗ Echo server did not receive the message${NC}"
    fi
  else
    echo -e "${RED}Failed (Status: $STATUS)${NC}"
    echo "Response: $BODY"
  fi
}

# Function to clean up
cleanup() {
  echo -e "\n${YELLOW}Cleaning up:${NC}"
  
  if [ -n "$PROXY_PID" ]; then
    echo -n "Stopping HTTP Proxy... "
    kill $PROXY_PID
    wait $PROXY_PID 2>/dev/null
    echo -e "${GREEN}Done${NC}"
  fi
  
  if [ -n "$ECHO_PID" ]; then
    echo -n "Stopping Echo Server... "
    kill $ECHO_PID
    wait $ECHO_PID 2>/dev/null
    echo -e "${GREEN}Done${NC}"
  fi
  
  echo -e "\n${GREEN}Integration tests completed${NC}"
  
  # Keep logs for inspection
  echo "Log files:"
  echo "  - proxy.log: HTTP Proxy (path-based mode)"
  echo "  - proxy-fixed.log: HTTP Proxy (fixed topic mode)"
  echo "  - proxy-async.log: HTTP Proxy (async mode)"
  echo "  - echo-server.log: Echo Server"
}

# Set up trap to clean up on exit
trap cleanup EXIT

# Main test sequence
check_redis || exit 1
start_echo_server || exit 1
start_proxy || exit 1

# Run tests
test_path_based
test_fixed_topic
test_async_mode

echo -e "\n${GREEN}All tests completed successfully!${NC}"