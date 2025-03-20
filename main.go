package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Config holds all configuration options
type Config struct {
	// Redis connection details
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// HTTP server settings
	Port int

	// Proxy behavior settings
	FixedTopic               string // If set, all messages go to this topic
	RespondImmediatelyStatus int    // If set, respond immediately with this status code
	ResponseTimeout          int    // Timeout in seconds for waiting for a response

	// Debug mode
	Debug bool
}

// Message represents the format of messages sent to Redis
type Message struct {
	Header map[string]interface{} `json:"header"`
	Body   interface{}            `json:"body"`
}

// Response represents the expected response format from Redis
type Response struct {
	Body interface{} `json:"body"`
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() Config {
	config := Config{
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		Port:            getEnvAsInt("PORT", 8080),
		ResponseTimeout: getEnvAsInt("RESPONSE_TIMEOUT", 30),
		Debug:           getEnvAsBool("DEBUG", false),
	}

	// Parse Redis DB
	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err == nil {
		config.RedisDB = redisDB
	}

	// Optional fixed topic
	config.FixedTopic = getEnv("FIXED_TOPIC", "")

	// Optional immediate response status code
	respondStatus := getEnv("RESPOND_IMMEDIATELY_STATUS_CODE", "")
	if respondStatus != "" {
		statusCode, err := strconv.Atoi(respondStatus)
		if err == nil {
			config.RespondImmediatelyStatus = statusCode
		}
	}

	return config
}

// Helper function to get environment variable with a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Helper function to get environment variable as int with a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// Helper function to get environment variable as bool with a default value
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// createTopicFromPath converts an HTTP path to a Redis topic by replacing slashes
func createTopicFromPath(path string) string {
	// Remove leading slash if present
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	// Replace slashes with Redis separator (typically ":")
	return strings.ReplaceAll(path, "/", ":")
}

// debugLog logs messages when in debug mode
func debugLog(config Config, format string, v ...interface{}) {
	if config.Debug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Helper function to truncate long strings for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractResponseBody attempts to extract a response body from various formats
func extractResponseBody(payload string) (interface{}, error) {
	// First, try standard Response format
	var response Response
	if err := json.Unmarshal([]byte(payload), &response); err == nil {
		if response.Body != nil {
			return response.Body, nil
		}
	}

	// Try parsing as direct JSON object
	var directJSON interface{}
	if err := json.Unmarshal([]byte(payload), &directJSON); err == nil {
		return directJSON, nil
	}

	// If we can't parse it as JSON at all, return the raw string
	return payload, nil
}

// handleRequest processes incoming HTTP requests
func handleRequest(w http.ResponseWriter, r *http.Request, config Config, redisClient *redis.Client) {
	ctx := context.Background()
	requestID := uuid.New().String()

	debugLog(config, "[%s] Received request: %s %s", requestID, r.Method, r.URL.Path)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[%s] Error reading request body: %v", requestID, err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	debugLog(config, "[%s] Request body length: %d bytes", requestID, len(body))

	// Prepare headers map
	headers := make(map[string]interface{})
	for key, values := range r.Header {
		if len(values) == 1 {
			headers[key] = values[0]
		} else {
			headers[key] = values
		}
	}

	// Add query parameters to headers
	queryParams := r.URL.Query()
	for key, values := range queryParams {
		if len(values) == 1 {
			headers["query_"+key] = values[0]
		} else {
			headers["query_"+key] = values
		}
	}

	// Add path and method to headers
	headers["path"] = r.URL.Path
	headers["method"] = r.Method
	headers["request_id"] = requestID

	// Create message
	var bodyData interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &bodyData); err != nil {
			// If not valid JSON, use as string
			bodyData = string(body)
			debugLog(config, "[%s] Body is not valid JSON, using as string", requestID)
		} else {
			debugLog(config, "[%s] Body parsed as JSON", requestID)
		}
	}

	message := Message{
		Header: headers,
		Body:   bodyData,
	}

	// Determine the topic to publish to
	var topic string
	if config.FixedTopic != "" {
		topic = config.FixedTopic
		debugLog(config, "[%s] Using fixed topic: %s", requestID, topic)
	} else {
		// Use the path but replace slashes with Redis separator
		topic = createTopicFromPath(r.URL.Path)
		debugLog(config, "[%s] Using path-based topic: %s", requestID, topic)
	}

	// Generate a unique response topic
	responseID := uuid.New().String()
	responseTopic := fmt.Sprintf("%s:response:%s", topic, responseID)

	debugLog(config, "[%s] Response topic: %s", requestID, responseTopic)

	// Add the response topic to the message headers
	message.Header["response_topic"] = responseTopic

	// Marshal the message to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		log.Printf("[%s] Error creating message: %v", requestID, err)
		http.Error(w, "Error creating message", http.StatusInternalServerError)
		return
	}

	// If configured to respond immediately, do so and return
	if config.RespondImmediatelyStatus > 0 {
		debugLog(config, "[%s] Publishing message to topic: %s", requestID, topic)

		// Publish the message to Redis
		err = redisClient.Publish(ctx, topic, messageJSON).Err()
		if err != nil {
			log.Printf("[%s] Error publishing to Redis: %v", requestID, err)
			http.Error(w, "Error publishing to Redis", http.StatusInternalServerError)
			return
		}

		debugLog(config, "[%s] Message published successfully", requestID)
		debugLog(config, "[%s] Responding immediately with status code: %d", requestID, config.RespondImmediatelyStatus)
		w.WriteHeader(config.RespondImmediatelyStatus)
		return
	}

	// At this point, we know we need to wait for a response
	debugLog(config, "[%s] Setting up response handler for topic: %s", requestID, responseTopic)

	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(config.ResponseTimeout)*time.Second)
	defer cancel()

	// Subscribe to the response topic BEFORE publishing the message
	pubsub := redisClient.Subscribe(ctx, responseTopic)
	defer pubsub.Close()

	// Make sure subscription is established before waiting for messages
	if _, err := pubsub.Receive(ctx); err != nil {
		log.Printf("[%s] Error establishing subscription: %v", requestID, err)
		http.Error(w, "Error connecting to response channel", http.StatusInternalServerError)
		return
	}

	debugLog(config, "[%s] Subscription to response topic established", requestID)

	// Set up channels for message handling
	msgChan := make(chan *redis.Message)
	errChan := make(chan error)

	// Start the goroutine to listen for messages BEFORE publishing
	go func() {
		ch := pubsub.Channel()
		debugLog(config, "[%s] Started goroutine to listen for messages", requestID)

		select {
		case msg := <-ch:
			debugLog(config, "[%s] Received message from channel: %s", requestID, msg.Channel)
			msgChan <- msg
		case <-timeoutCtx.Done():
			errChan <- timeoutCtx.Err()
		}
	}()

	// NOW publish the message to Redis after the listener is set up
	debugLog(config, "[%s] Publishing message to topic: %s", requestID, topic)

	err = redisClient.Publish(ctx, topic, messageJSON).Err()
	if err != nil {
		log.Printf("[%s] Error publishing to Redis: %v", requestID, err)
		http.Error(w, "Error publishing to Redis", http.StatusInternalServerError)
		return
	}

	debugLog(config, "[%s] Message published successfully", requestID)
	debugLog(config, "[%s] Waiting for response on topic: %s with timeout: %d seconds",
		requestID, responseTopic, config.ResponseTimeout)

	// Wait for either a message, an error, or a timeout
	select {
	case msg := <-msgChan:
		debugLog(config, "[%s] Processing received message from topic: %s", requestID, msg.Channel)
		debugLog(config, "[%s] Message payload: %s", requestID, msg.Payload)

		// Extract response body with more flexible handling
		responseBody, err := extractResponseBody(msg.Payload)
		if err != nil {
			log.Printf("[%s] Error parsing response: %v", requestID, err)
			log.Printf("[%s] Raw response: %s", requestID, msg.Payload)
			http.Error(w, "Error parsing response", http.StatusInternalServerError)
			return
		}

		debugLog(config, "[%s] Response processed successfully", requestID)

		// Send the response body back to the client
		w.Header().Set("Content-Type", "application/json")
		responseJSON, err := json.Marshal(responseBody)
		if err != nil {
			log.Printf("[%s] Error formatting response: %v", requestID, err)
			http.Error(w, "Error formatting response", http.StatusInternalServerError)
			return
		}

		debugLog(config, "[%s] Writing response to client", requestID)
		w.Write(responseJSON)
		debugLog(config, "[%s] Response sent to client successfully", requestID)

	case err := <-errChan:
		log.Printf("[%s] Error receiving message: %v", requestID, err)
		http.Error(w, "Error receiving response", http.StatusInternalServerError)
		return

	case <-timeoutCtx.Done():
		// Timeout occurred
		log.Printf("[%s] Response timeout after %d seconds", requestID, config.ResponseTimeout)
		http.Error(w, "Response timeout", http.StatusGatewayTimeout)
		return
	}
}

func main() {
	// Load configuration
	config := LoadConfigFromEnv()

	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if config.Debug {
		log.Println("[DEBUG] Debug mode enabled")
	}

	// Set up Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Verify Redis connection
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	debugLog(config, "Successfully connected to Redis at %s", config.RedisAddr)

	// Set up HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, config, redisClient)
	})

	// Start server
	serverAddr := fmt.Sprintf(":%d", config.Port)
	log.Printf("Starting HTTP server on %s", serverAddr)
	log.Printf("Redis proxy configuration: RedisAddr=%s, FixedTopic=%s, RespondImmediately=%d, Timeout=%ds, Debug=%v",
		config.RedisAddr, config.FixedTopic, config.RespondImmediatelyStatus, config.ResponseTimeout, config.Debug)

	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
