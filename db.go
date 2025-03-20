package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

// RequestLogEntry represents a log entry for requests and responses
type RequestLogEntry struct {
	ID            int64     `json:"id"`
	RequestID     string    `json:"request_id"`
	Method        string    `json:"method"`
	Path          string    `json:"path"`
	Topic         string    `json:"topic"`
	RequestBody   string    `json:"request_body"`
	ResponseBody  string    `json:"response_body"`
	StatusCode    int       `json:"status_code"`
	ResponseTime  int64     `json:"response_time_ms"` // in milliseconds
	Timestamp     time.Time `json:"timestamp"`
	ResponseTopic string    `json:"response_topic"`
	Error         string    `json:"error,omitempty"`
}

// DBLogger handles logging of requests and responses to SQLite
type DBLogger struct {
	db          *sql.DB
	maxEntries  int
	initialized bool
	enabled     bool
	mutex       sync.Mutex
	queue       chan *RequestLogEntry
	wg          sync.WaitGroup
}

// NewDBLogger creates a new database logger
func NewDBLogger(dbPath string, maxEntries int) (*DBLogger, error) {
	if dbPath == "" || maxEntries <= 0 {
		// Return a disabled logger if no path or max entries <= 0
		return &DBLogger{
			enabled: false,
		}, nil
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	logger := &DBLogger{
		db:         db,
		maxEntries: maxEntries,
		enabled:    true,
		queue:      make(chan *RequestLogEntry, 100), // Buffer size for queued log entries
	}

	// Initialize the database schema
	if err := logger.initDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Start background worker
	logger.startWorker()

	return logger, nil
}

// initDB initializes the database schema
func (l *DBLogger) initDB() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.initialized {
		return nil
	}

	// Create requests table
	_, err := l.db.Exec(`
		CREATE TABLE IF NOT EXISTS request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			topic TEXT NOT NULL,
			request_body TEXT,
			response_body TEXT,
			status_code INTEGER,
			response_time INTEGER,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			response_topic TEXT,
			error TEXT
		)
	`)
	if err != nil {
		return err
	}

	// Create index on request_id for faster lookups
	_, err = l.db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_id ON request_logs(request_id)`)
	if err != nil {
		return err
	}

	// Create index on timestamp for cleanup
	_, err = l.db.Exec(`CREATE INDEX IF NOT EXISTS idx_timestamp ON request_logs(timestamp)`)
	if err != nil {
		return err
	}

	l.initialized = true
	return nil
}

// startWorker starts a background worker to process log entries
func (l *DBLogger) startWorker() {
	if !l.enabled {
		return
	}

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()

		log.Info().Int("maxEntries", l.maxEntries).Msg("Starting DB logger worker")

		for entry := range l.queue {
			// Insert the log entry
			if err := l.insertLogEntry(entry); err != nil {
				log.Error().Err(err).Str("requestID", entry.RequestID).Msg("Failed to insert log entry")
			}

			// Cleanup old entries if needed
			if err := l.cleanupOldEntries(); err != nil {
				log.Error().Err(err).Msg("Failed to cleanup old log entries")
			}
		}
	}()
}

// LogRequest logs a request to the database
func (l *DBLogger) LogRequest(ctx context.Context, requestID, method, path, topic, responseTopic string, requestBody interface{}) {
	if !l.enabled {
		return
	}

	var bodyStr string
	if requestBody != nil {
		bodyBytes, err := json.Marshal(requestBody)
		if err == nil {
			bodyStr = string(bodyBytes)
		} else {
			bodyStr = fmt.Sprintf("Error marshaling body: %v", err)
		}
	}

	entry := &RequestLogEntry{
		RequestID:     requestID,
		Method:        method,
		Path:          path,
		Topic:         topic,
		RequestBody:   bodyStr,
		Timestamp:     time.Now(),
		ResponseTopic: responseTopic,
	}

	// Add to processing queue
	select {
	case l.queue <- entry:
		// Entry queued successfully
	default:
		// Queue full, log and drop
		log.Warn().Str("requestID", requestID).Msg("DB logger queue full, dropping log entry")
	}
}

// LogResponse updates the request log with response information
func (l *DBLogger) LogResponse(requestID string, statusCode int, responseBody interface{}, responseTime time.Duration, err error) {
	if !l.enabled {
		return
	}

	var bodyStr string
	if responseBody != nil {
		bodyBytes, marshalErr := json.Marshal(responseBody)
		if marshalErr == nil {
			bodyStr = string(bodyBytes)
		} else {
			bodyStr = fmt.Sprintf("Error marshaling response: %v", marshalErr)
		}
	}

	var errStr string
	if err != nil {
		errStr = err.Error()
	}

	entry := &RequestLogEntry{
		RequestID:    requestID,
		ResponseBody: bodyStr,
		StatusCode:   statusCode,
		ResponseTime: responseTime.Milliseconds(),
		Error:        errStr,
	}

	// Add to processing queue
	select {
	case l.queue <- entry:
		// Entry queued successfully
	default:
		// Queue full, log and drop
		log.Warn().Str("requestID", requestID).Msg("DB logger queue full, dropping response log entry")
	}
}

// insertLogEntry inserts or updates a log entry in the database
func (l *DBLogger) insertLogEntry(entry *RequestLogEntry) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if entry.RequestID == "" {
		return fmt.Errorf("request ID is required")
	}

	// Check if this is a request or response entry
	if entry.Method != "" {
		// This is a new request entry
		_, err := l.db.Exec(`
			INSERT INTO request_logs 
			(request_id, method, path, topic, request_body, timestamp, response_topic) 
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, entry.RequestID, entry.Method, entry.Path, entry.Topic, entry.RequestBody, entry.Timestamp, entry.ResponseTopic)
		return err
	} else {
		// This is a response update
		_, err := l.db.Exec(`
			UPDATE request_logs 
			SET response_body = ?, status_code = ?, response_time = ?, error = ?
			WHERE request_id = ?
		`, entry.ResponseBody, entry.StatusCode, entry.ResponseTime, entry.Error, entry.RequestID)
		return err
	}
}

// cleanupOldEntries removes old entries to maintain the maximum number of entries
func (l *DBLogger) cleanupOldEntries() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Count total entries
	var count int
	err := l.db.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&count)
	if err != nil {
		return err
	}

	// If we're below the limit, no need to clean up
	if count <= l.maxEntries {
		return nil
	}

	// Delete oldest entries
	deleteCount := count - l.maxEntries
	_, err = l.db.Exec(`
		DELETE FROM request_logs 
		WHERE id IN (
			SELECT id FROM request_logs ORDER BY timestamp ASC LIMIT ?
		)
	`, deleteCount)

	return err
}

// GetEntries retrieves the latest log entries
func (l *DBLogger) GetEntries(limit int) ([]RequestLogEntry, error) {
	if !l.enabled {
		return nil, nil
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	if limit <= 0 || limit > l.maxEntries {
		limit = l.maxEntries
	}

	rows, err := l.db.Query(`
		SELECT id, request_id, method, path, topic, request_body, response_body, 
		       status_code, response_time, timestamp, response_topic, error
		FROM request_logs 
		ORDER BY timestamp DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []RequestLogEntry
	for rows.Next() {
		var entry RequestLogEntry
		var timestamp string

		err := rows.Scan(
			&entry.ID, &entry.RequestID, &entry.Method, &entry.Path, &entry.Topic,
			&entry.RequestBody, &entry.ResponseBody, &entry.StatusCode, &entry.ResponseTime,
			&timestamp, &entry.ResponseTopic, &entry.Error,
		)
		if err != nil {
			return nil, err
		}

		// Parse timestamp
		entry.Timestamp, _ = time.Parse("2006-01-02 15:04:05", timestamp)
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetEntry retrieves a specific log entry by request ID
func (l *DBLogger) GetEntry(requestID string) (*RequestLogEntry, error) {
	if !l.enabled {
		return nil, nil
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	var entry RequestLogEntry
	var timestamp string

	err := l.db.QueryRow(`
		SELECT id, request_id, method, path, topic, request_body, response_body, 
		       status_code, response_time, timestamp, response_topic, error
		FROM request_logs 
		WHERE request_id = ?
	`, requestID).Scan(
		&entry.ID, &entry.RequestID, &entry.Method, &entry.Path, &entry.Topic,
		&entry.RequestBody, &entry.ResponseBody, &entry.StatusCode, &entry.ResponseTime,
		&timestamp, &entry.ResponseTopic, &entry.Error,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// Parse timestamp
	entry.Timestamp, _ = time.Parse("2006-01-02 15:04:05", timestamp)

	return &entry, nil
}

// Close closes the database connection and worker
func (l *DBLogger) Close() error {
	if !l.enabled {
		return nil
	}

	log.Info().Msg("Closing DB logger")
	close(l.queue)
	l.wg.Wait()

	return l.db.Close()
}
