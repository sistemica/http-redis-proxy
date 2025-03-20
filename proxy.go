package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ProxyServer represents the HTTP server for Redis proxy
type ProxyServer struct {
	config       Config
	redisManager *RedisManager
	server       *http.Server
	wg           *sync.WaitGroup
	dbLogger     *DBLogger // Optional DB logger for request/response tracking
}

// NewProxyServer creates a new proxy server
func NewProxyServer(config Config, redisManager *RedisManager, wg *sync.WaitGroup, dbLogger *DBLogger) *ProxyServer {
	proxy := &ProxyServer{
		config:       config,
		redisManager: redisManager,
		wg:           wg,
		dbLogger:     dbLogger,
	}

	// Create HTTP server with proper timeouts
	mux := http.NewServeMux()

	// Main request handler
	mux.HandleFunc("/", proxy.handleRequest)

	// The /logs and /stats endpoints are moved to the dashboard server

	proxy.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", config.Port),
		Handler:        mux,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		IdleTimeout:    config.IdleTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}

	return proxy
}

// Start starts the HTTP server
func (ps *ProxyServer) Start() error {
	log.Info().Int("port", ps.config.Port).Msg("Starting HTTP proxy server")
	return ps.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (ps *ProxyServer) Shutdown(ctx context.Context) error {
	return ps.server.Shutdown(ctx)
}

// handleRequest processes incoming HTTP requests
func (ps *ProxyServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Increment wait group counter for graceful shutdown
	ps.wg.Add(1)
	defer ps.wg.Done()

	ctx := context.Background()
	requestID := uuid.New().String()
	logger := log.With().Str("requestID", requestID).Str("path", r.URL.Path).Str("method", r.Method).Logger()

	startTime := time.Now()
	logger.Debug().Msg("Received request")

	// Read request body with size limit
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		logger.Error().Err(err).Msg("Error reading request body")
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	logger.Debug().Int("bodyLength", len(body)).Msg("Request body read")

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
			logger.Debug().Msg("Body is not valid JSON, using as string")
		} else {
			logger.Debug().Msg("Body parsed as JSON")
		}
	}

	message := Message{
		Header: headers,
		Body:   bodyData,
	}

	// Determine the topic to publish to
	var topic string
	if ps.config.FixedTopic != "" {
		topic = ps.config.FixedTopic
		logger.Debug().Str("fixedTopic", topic).Msg("Using fixed topic")
	} else {
		// Use the path but replace slashes with Redis separator
		topic = createTopicFromPath(r.URL.Path)
		logger.Debug().Str("pathBasedTopic", topic).Msg("Using path-based topic")
	}

	// Generate a unique response topic
	responseID := uuid.New().String()
	responseTopic := fmt.Sprintf("%s:response:%s", topic, responseID)

	logger.Debug().Str("responseTopic", responseTopic).Msg("Created response topic")

	// Add the response topic to the message headers
	message.Header["response_topic"] = responseTopic

	// Marshal the message to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		logger.Error().Err(err).Msg("Error creating message")
		http.Error(w, "Error creating message", http.StatusInternalServerError)
		return
	}

	// Log request to database if enabled
	if ps.dbLogger != nil && ps.dbLogger.enabled {
		ps.dbLogger.LogRequest(ctx, requestID, r.Method, r.URL.Path, topic, responseTopic, bodyData)
	}

	// If configured to respond immediately, do so and return
	if ps.config.RespondImmediatelyStatus > 0 {
		logger.Debug().Str("topic", topic).Msg("Publishing message")

		// Publish the message to Redis
		err = ps.redisManager.Publish(ctx, topic, messageJSON)
		if err != nil {
			logger.Error().Err(err).Str("topic", topic).Msg("Error publishing to Redis")

			// Log error response
			if ps.dbLogger != nil && ps.dbLogger.enabled {
				ps.dbLogger.LogResponse(requestID, http.StatusInternalServerError, nil, time.Since(startTime), err)
			}

			http.Error(w, "Error publishing to Redis", http.StatusInternalServerError)
			return
		}

		logger.Debug().Int("statusCode", ps.config.RespondImmediatelyStatus).Msg("Responding immediately")

		// Log success response
		if ps.dbLogger != nil && ps.dbLogger.enabled {
			ps.dbLogger.LogResponse(requestID, ps.config.RespondImmediatelyStatus, nil, time.Since(startTime), nil)
		}

		w.WriteHeader(ps.config.RespondImmediatelyStatus)
		return
	}

	// At this point, we know we need to wait for a response
	logger.Debug().Str("responseTopic", responseTopic).Msg("Setting up response handler")

	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(ps.config.ResponseTimeout)*time.Second)
	defer cancel()

	// Subscribe to the response topic BEFORE publishing the message
	pubsub := ps.redisManager.Subscribe(ctx, responseTopic)
	defer pubsub.Close()

	// Make sure subscription is established before waiting for messages
	if _, err := pubsub.Receive(ctx); err != nil {
		logger.Error().Err(err).Str("responseTopic", responseTopic).Msg("Error establishing subscription")

		// Log error response
		if ps.dbLogger != nil && ps.dbLogger.enabled {
			ps.dbLogger.LogResponse(requestID, http.StatusInternalServerError, nil, time.Since(startTime), err)
		}

		http.Error(w, "Error connecting to response channel", http.StatusInternalServerError)
		return
	}

	logger.Debug().Str("responseTopic", responseTopic).Msg("Subscription established")

	// Set up channels for message handling
	msgChan := make(chan *json.RawMessage)
	errChan := make(chan error)

	// Start the goroutine to listen for messages BEFORE publishing
	go func() {
		ch := pubsub.Channel()
		logger.Debug().Msg("Started goroutine to listen for messages")

		select {
		case msg := <-ch:
			logger.Debug().Str("channel", msg.Channel).Msg("Received message from channel")
			var rawMsg json.RawMessage = []byte(msg.Payload)
			msgChan <- &rawMsg
		case <-timeoutCtx.Done():
			errChan <- timeoutCtx.Err()
		}
	}()

	// NOW publish the message to Redis after the listener is set up
	logger.Debug().Str("topic", topic).Msg("Publishing message")
	err = ps.redisManager.Publish(ctx, topic, messageJSON)
	if err != nil {
		logger.Error().Err(err).Str("topic", topic).Msg("Error publishing to Redis")

		// Log error response
		if ps.dbLogger != nil && ps.dbLogger.enabled {
			ps.dbLogger.LogResponse(requestID, http.StatusInternalServerError, nil, time.Since(startTime), err)
		}

		http.Error(w, "Error publishing to Redis", http.StatusInternalServerError)
		return
	}

	logger.Debug().Msg("Message published successfully")
	logger.Debug().Str("responseTopic", responseTopic).Int("timeout", ps.config.ResponseTimeout).
		Msg("Waiting for response")

	// Wait for either a message, an error, or a timeout
	var responseBody interface{}
	var statusCode int
	var responseErr error

	select {
	case msg := <-msgChan:
		logger.Debug().Str("payload", truncateString(string(*msg), 200)).
			Msg("Processing received message")

		// Extract response body with more flexible handling
		responseBody, err = extractResponseBody(string(*msg))
		if err != nil {
			logger.Error().Err(err).Str("payload", truncateString(string(*msg), 500)).
				Msg("Error parsing response")

			statusCode = http.StatusInternalServerError
			responseErr = fmt.Errorf("error parsing response: %w", err)
		} else {
			logger.Debug().Msg("Response processed successfully")
			statusCode = http.StatusOK
		}

	case err := <-errChan:
		logger.Error().Err(err).Msg("Error receiving message")
		statusCode = http.StatusInternalServerError
		responseErr = fmt.Errorf("error receiving response: %w", err)

	case <-timeoutCtx.Done():
		// Timeout occurred
		logger.Error().Int("timeout", ps.config.ResponseTimeout).Msg("Response timeout")
		statusCode = http.StatusGatewayTimeout
		responseErr = fmt.Errorf("response timeout after %d seconds", ps.config.ResponseTimeout)
	}

	// Log response to database if enabled
	if ps.dbLogger != nil && ps.dbLogger.enabled {
		ps.dbLogger.LogResponse(requestID, statusCode, responseBody, time.Since(startTime), responseErr)
	}

	// Handle error cases
	if responseErr != nil {
		http.Error(w, responseErr.Error(), statusCode)
		return
	}

	// Send the response body back to the client
	w.Header().Set("Content-Type", "application/json")
	responseJSON, err := json.Marshal(responseBody)
	if err != nil {
		logger.Error().Err(err).Msg("Error formatting response")

		// Update response log with error
		if ps.dbLogger != nil && ps.dbLogger.enabled {
			ps.dbLogger.LogResponse(requestID, http.StatusInternalServerError, nil, time.Since(startTime), err)
		}

		http.Error(w, "Error formatting response", http.StatusInternalServerError)
		return
	}

	logger.Debug().Msg("Writing response to client")
	w.Write(responseJSON)
	logger.Debug().Msg("Response sent to client successfully")
}
