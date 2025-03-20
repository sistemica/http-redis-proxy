package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

// handleLogsAPIRequest processes API requests for log data
func handleLogsAPIRequest(w http.ResponseWriter, r *http.Request, dbLogger *DBLogger) {
	// Parse limit parameter
	limitStr := r.URL.Query().Get("limit")
	limit := 1000 // Default to 1000 entries
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Parse specific request ID
	requestID := r.URL.Query().Get("request_id")

	var (
		entries interface{}
		err     error
	)

	if requestID != "" {
		// Get specific log entry
		entries, err = dbLogger.GetEntry(requestID)
	} else {
		// Get latest entries
		entries, err = dbLogger.GetEntries(limit)
	}

	if err != nil {
		log.Error().Err(err).Msg("Error retrieving log entries")
		http.Error(w, "Error retrieving logs", http.StatusInternalServerError)
		return
	}

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		log.Error().Err(err).Msg("Error encoding log entries")
		http.Error(w, "Error encoding logs", http.StatusInternalServerError)
	}
}

// handleStatsAPIRequest processes API requests for statistics data
func handleStatsAPIRequest(w http.ResponseWriter, r *http.Request, dbLogger *DBLogger) {
	// Parse period parameter
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "day" // Default to last 24 hours
	}

	// Parse window size parameter
	windowSizeStr := r.URL.Query().Get("window_size")
	windowSize := 10 // Default to top 10
	if windowSizeStr != "" {
		if parsed, err := strconv.Atoi(windowSizeStr); err == nil && parsed > 0 {
			windowSize = parsed
		}
	}

	// Get statistics from database
	stats, err := dbLogger.getRequestStats(r.Context(), period, windowSize)
	if err != nil {
		log.Error().Err(err).Str("period", period).Msg("Error retrieving request statistics")
		http.Error(w, "Error retrieving statistics", http.StatusInternalServerError)
		return
	}

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Error().Err(err).Msg("Error encoding statistics")
		http.Error(w, "Error encoding statistics", http.StatusInternalServerError)
	}
}
