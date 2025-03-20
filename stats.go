package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// RequestStats represents statistics about requests
type RequestStats struct {
	Timestamp            time.Time      `json:"timestamp"`
	TotalRequests        int            `json:"total_requests"`
	SuccessfulRequests   int            `json:"successful_requests"`
	FailedRequests       int            `json:"failed_requests"`
	AverageResponseTime  float64        `json:"average_response_time_ms"`
	MinResponseTime      int64          `json:"min_response_time_ms"`
	MaxResponseTime      int64          `json:"max_response_time_ms"`
	TimeoutRequests      int            `json:"timeout_requests"`
	RequestsByStatusCode map[int]int    `json:"requests_by_status_code"`
	RequestsByTopic      map[string]int `json:"requests_by_topic"`
	Period               string         `json:"period"`
	WindowSize           int            `json:"window_size"`
}

// getRequestStats retrieves statistics about requests from the database
func (l *DBLogger) getRequestStats(ctx context.Context, period string, windowSize int) (*RequestStats, error) {
	if !l.enabled {
		return nil, nil
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Determine time window based on period
	var timeWindow time.Time
	now := time.Now()

	switch period {
	case "hour":
		timeWindow = now.Add(-1 * time.Hour)
	case "day":
		timeWindow = now.AddDate(0, 0, -1)
	case "week":
		timeWindow = now.AddDate(0, 0, -7)
	case "month":
		timeWindow = now.AddDate(0, -1, 0)
	case "all":
		// No time window filter
	default:
		// Default to last hour
		period = "hour"
		timeWindow = now.Add(-1 * time.Hour)
	}

	var whereClause string
	var args []interface{}
	if period != "all" {
		whereClause = "WHERE timestamp >= datetime(?)"
		args = append(args, timeWindow.Format("2006-01-02 15:04:05"))
	}

	// Get total requests
	var totalRequests int
	query := "SELECT COUNT(*) FROM request_logs " + whereClause
	err := l.db.QueryRowContext(ctx, query, args...).Scan(&totalRequests)
	if err != nil {
		return nil, err
	}

	// Get successful requests (status code 2xx)
	var successfulRequests int
	query = "SELECT COUNT(*) FROM request_logs WHERE status_code >= 200 AND status_code < 300 "
	if whereClause != "" {
		query += "AND timestamp >= datetime(?)"
	}
	err = l.db.QueryRowContext(ctx, query, args...).Scan(&successfulRequests)
	if err != nil {
		return nil, err
	}

	// Get failed requests (status code >= 400 or error not null)
	var failedRequests int
	query = "SELECT COUNT(*) FROM request_logs WHERE (status_code >= 400 OR error IS NOT NULL) "
	if whereClause != "" {
		query += "AND timestamp >= datetime(?)"
	}
	err = l.db.QueryRowContext(ctx, query, args...).Scan(&failedRequests)
	if err != nil {
		return nil, err
	}

	// Get timeout requests (status code 504 or error message containing "timeout")
	var timeoutRequests int
	query = "SELECT COUNT(*) FROM request_logs WHERE status_code = 504 OR error LIKE '%timeout%' "
	if whereClause != "" {
		query += "AND timestamp >= datetime(?)"
	}
	err = l.db.QueryRowContext(ctx, query, args...).Scan(&timeoutRequests)
	if err != nil {
		return nil, err
	}

	// Get average, min, max response time
	var avgResponseTime float64
	var minResponseTime sql.NullInt64
	var maxResponseTime sql.NullInt64
	query = "SELECT AVG(response_time), MIN(response_time), MAX(response_time) FROM request_logs WHERE response_time > 0 "
	if whereClause != "" {
		query += "AND timestamp >= datetime(?)"
	}
	err = l.db.QueryRowContext(ctx, query, args...).Scan(&avgResponseTime, &minResponseTime, &maxResponseTime)
	if err != nil {
		return nil, err
	}

	// Get requests by status code
	requestsByStatusCode := make(map[int]int)
	query = "SELECT status_code, COUNT(*) FROM request_logs WHERE status_code IS NOT NULL "
	if whereClause != "" {
		query += "AND timestamp >= datetime(?)"
	}
	query += " GROUP BY status_code"

	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var statusCode int
		var count int
		if err := rows.Scan(&statusCode, &count); err != nil {
			return nil, err
		}
		requestsByStatusCode[statusCode] = count
	}

	// Get requests by topic
	requestsByTopic := make(map[string]int)
	query = "SELECT topic, COUNT(*) FROM request_logs "
	if whereClause != "" {
		query += "WHERE timestamp >= datetime(?)"
	}
	query += " GROUP BY topic ORDER BY COUNT(*) DESC LIMIT ?"

	args = append(args, windowSize)
	rows, err = l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var topic string
		var count int
		if err := rows.Scan(&topic, &count); err != nil {
			return nil, err
		}
		requestsByTopic[topic] = count
	}

	return &RequestStats{
		Timestamp:            now,
		TotalRequests:        totalRequests,
		SuccessfulRequests:   successfulRequests,
		FailedRequests:       failedRequests,
		AverageResponseTime:  avgResponseTime,
		MinResponseTime:      minResponseTime.Int64,
		MaxResponseTime:      maxResponseTime.Int64,
		TimeoutRequests:      timeoutRequests,
		RequestsByStatusCode: requestsByStatusCode,
		RequestsByTopic:      requestsByTopic,
		Period:               period,
		WindowSize:           windowSize,
	}, nil
}

// statsHandler provides statistics about logged requests
func (ps *ProxyServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	if ps.dbLogger == nil || !ps.dbLogger.enabled {
		http.Error(w, "Logging not enabled", http.StatusNotFound)
		return
	}

	// Parse query parameters
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "hour" // Default to last hour
	}

	windowSizeStr := r.URL.Query().Get("window_size")
	windowSize := 10 // Default to top 10
	if windowSizeStr != "" {
		if parsed, err := strconv.Atoi(windowSizeStr); err == nil && parsed > 0 {
			windowSize = parsed
		}
	}

	// Get statistics from database
	stats, err := ps.dbLogger.getRequestStats(r.Context(), period, windowSize)
	if err != nil {
		log.Error().Err(err).Str("period", period).Msg("Error retrieving request statistics")
		http.Error(w, "Error retrieving statistics", http.StatusInternalServerError)
		return
	}

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Error().Err(err).Msg("Error encoding stats")
		http.Error(w, "Error encoding stats", http.StatusInternalServerError)
	}
}
