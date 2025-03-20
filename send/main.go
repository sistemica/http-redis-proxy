package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Message structure matching the echo server's expected format
type Message struct {
	Header Header          `json:"header"`
	Body   json.RawMessage `json:"body"`
}

// Header structure
type Header struct {
	ResponseTopic string `json:"response_topic"`
	System        string `json:"system,omitempty"`
}

func main() {
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	ctx := context.Background()

	// Test connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully")

	// Define topics
	incomingTopic := "incoming-messages"
	responseTopic := "response-topic"

	// Create a message to send
	testMessage := Message{
		Header: Header{
			ResponseTopic: responseTopic,
		},
		Body: json.RawMessage(`{"message":"Hello, Echo Server!", "timestamp":"` + time.Now().Format(time.RFC3339) + `"}`),
	}

	// Convert to JSON
	messageJSON, err := json.Marshal(testMessage)
	if err != nil {
		log.Fatalf("Failed to marshal message: %v", err)
	}

	// Subscribe to the response topic before sending the message
	pubsub := client.Subscribe(ctx, responseTopic)
	defer pubsub.Close()

	// Setup response channel
	responseCh := pubsub.Channel()

	// Send the message
	log.Printf("Sending test message to topic '%s'...", incomingTopic)
	log.Printf("Message: %s", string(messageJSON))
	err = client.Publish(ctx, incomingTopic, messageJSON).Err()
	if err != nil {
		log.Fatalf("Failed to publish message: %v", err)
	}
	log.Println("Message sent successfully")

	// Wait for response with timeout
	log.Printf("Waiting for response on topic '%s'...", responseTopic)
	timeout := time.After(5 * time.Second)
	select {
	case msg := <-responseCh:
		log.Println("Received response:")
		log.Println(msg.Payload)

		// Parse the response to pretty print it
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(msg.Payload), &response); err == nil {
			prettyJSON, _ := json.MarshalIndent(response, "", "  ")
			fmt.Println("\nPretty Response:")
			fmt.Println(string(prettyJSON))
		}

	case <-timeout:
		log.Println("Timeout: No response received within 5 seconds")
	}
}
