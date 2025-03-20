package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Config holds all configuration options
type Config struct {
	// Redis connection details
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisPoolSize int

	// HTTP server settings
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	MaxHeaderBytes  int
	ShutdownTimeout int

	// Proxy behavior settings
	FixedTopic               string // If set, all messages go to this topic
	RespondImmediatelyStatus int    // If set, respond immediately with this status code
	ResponseTimeout          int    // Timeout in seconds for waiting for a response

	// Debug mode
	Debug    bool
	LogLevel zerolog.Level

	// Database logging
	DBLogPath    string // Path to SQLite database for request/response logging
	DBMaxEntries int    // Maximum number of entries to keep in the database
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

// Result represents the result of a request processing
type Result struct {
	StatusCode int
	Body       []byte
	Error      error
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() Config {
	// Default configuration
	config := Config{
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		RedisPoolSize:   getEnvAsInt("REDIS_POOL_SIZE", 10),
		Port:            getEnvAsInt("PORT", 8080),
		ReadTimeout:     time.Duration(getEnvAsInt("HTTP_READ_TIMEOUT", 30)) * time.Second,
		WriteTimeout:    time.Duration(getEnvAsInt("HTTP_WRITE_TIMEOUT", 30)) * time.Second,
		IdleTimeout:     time.Duration(getEnvAsInt("HTTP_IDLE_TIMEOUT", 60)) * time.Second,
		MaxHeaderBytes:  getEnvAsInt("HTTP_MAX_HEADER_BYTES", 1<<20), // 1MB
		ShutdownTimeout: getEnvAsInt("SHUTDOWN_TIMEOUT", 30),
		ResponseTimeout: getEnvAsInt("RESPONSE_TIMEOUT", 30),
		LogLevel:        getLogLevel(getEnv("LOG_LEVEL", "info")),
		DBLogPath:       getEnv("DB_LOG_PATH", ""),
		DBMaxEntries:    getEnvAsInt("DB_MAX_ENTRIES", 0),
	}

	// Support DEBUG environment variable for backward compatibility
	if getEnvAsBool("DEBUG", false) {
		config.Debug = true
		config.LogLevel = zerolog.DebugLevel
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

// getLogLevel converts a string log level to zerolog.Level
func getLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
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
