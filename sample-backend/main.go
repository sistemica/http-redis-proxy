package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

// Message represents the structure of messages received from Redis
type Message struct {
	Header map[string]interface{} `json:"header"`
	Body   json.RawMessage        `json:"body"`
}

// Response is the structure of the echo response
type Response struct {
	Body interface{} `json:"body"`
}

func main() {
	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Get Redis connection details from environment variables or use defaults
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisTopicsStr := getEnv("REDIS_TOPICS", "incoming-messages")

	// Parse topics (comma separated list)
	redisTopics := strings.Split(redisTopicsStr, ",")
	for i, topic := range redisTopics {
		redisTopics[i] = strings.TrimSpace(topic)
	}

	// Check if we should use a pattern subscription
	usePattern := getEnvAsBool("USE_PATTERN", false)
	if usePattern {
		// Convert topics to patterns if needed
		for i, topic := range redisTopics {
			if !strings.HasSuffix(topic, "*") {
				redisTopics[i] = topic + "*"
			}
		}
	}

	// Get optional response delay
	responseDelay := getEnvAsInt("RESPONSE_DELAY_MS", 0)

	// Get debug flag
	debug := getEnvAsBool("DEBUG", false)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})

	// Create context for Redis operations
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test Redis connection
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis at %s: %v", redisAddr, err)
	}
	log.Printf("Connected to Redis at %s", redisAddr)

	// Create a pubsub client
	var pubsub *redis.PubSub
	if usePattern {
		log.Printf("Listening on patterns: %s", strings.Join(redisTopics, ", "))
		pubsub = client.PSubscribe(ctx, redisTopics...)
	} else {
		log.Printf("Listening on topics: %s", strings.Join(redisTopics, ", "))
		pubsub = client.Subscribe(ctx, redisTopics...)
	}
	defer pubsub.Close()

	// Make sure subscription is established
	if _, err := pubsub.Receive(ctx); err != nil {
		log.Fatalf("Failed to establish subscription: %v", err)
	}

	// Channel to receive messages
	ch := pubsub.Channel()

	// Graceful shutdown handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Echo server ready, response delay: %dms, debug: %v", responseDelay, debug)

	// Process messages
	for {
		select {
		case msg := <-ch:
			if debug {
				log.Printf("[DEBUG] Received message on channel: %s", msg.Channel)
			}
			go handleMessage(ctx, client, msg.Channel, msg.Payload, responseDelay, debug)
		case sig := <-sigCh:
			log.Printf("Received signal: %v, shutting down...", sig)
			return
		}
	}
}

func handleMessage(ctx context.Context, client *redis.Client, channel, payload string, delayMs int, debug bool) {
	if debug {
		log.Printf("[DEBUG] Processing message: %s", truncate(payload, 100))
	}

	// Parse the incoming message
	var incomingMsg Message
	if err := json.Unmarshal([]byte(payload), &incomingMsg); err != nil {
		log.Printf("Error parsing message: %v", err)
		log.Printf("Raw message: %s", payload)
		return
	}

	// Get response topic
	var responseTopic string
	if rt, ok := incomingMsg.Header["response_topic"].(string); ok {
		responseTopic = rt
	} else {
		log.Printf("No response_topic in header or wrong format, skipping message")
		return
	}

	if debug {
		log.Printf("[DEBUG] Response topic: %s", responseTopic)
	}

	// Parse body data
	var bodyData interface{}
	if len(incomingMsg.Body) > 0 {
		if err := json.Unmarshal(incomingMsg.Body, &bodyData); err != nil {
			log.Printf("Error parsing body JSON: %v", err)
			// Use raw body as string
			bodyData = string(incomingMsg.Body)
		}
	}

	// Add artificial delay if configured
	if delayMs > 0 {
		if debug {
			log.Printf("[DEBUG] Delaying response for %dms", delayMs)
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	// Create echo response
	response := Response{
		Body: map[string]interface{}{
			"status":           "success",
			"message":          "Echo response",
			"original_channel": channel,
			"original_header":  incomingMsg.Header,
			"original_body":    bodyData,
			"timestamp":        time.Now().Format(time.RFC3339),
		},
	}

	// Serialize the response
	responseJSON, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error creating response: %v", err)
		return
	}

	if debug {
		log.Printf("[DEBUG] Sending response to %s: %s", responseTopic, truncate(string(responseJSON), 100))
	}

	// Publish to the response topic
	if err := client.Publish(ctx, responseTopic, responseJSON).Err(); err != nil {
		log.Printf("Error publishing to response topic %s: %v", responseTopic, err)
		return
	}

	log.Printf("Echoed message to %s", responseTopic)
}

// Helper function to truncate string for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// getEnv returns the value of the environment variable or a default value if not set
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt returns the value of the environment variable as int or a default value if not set
func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// atoi converts string to int, with error handling
func atoi(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// getEnvAsBool returns the value of the environment variable as bool or a default value if not set
func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return strings.ToLower(value) == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
