package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// DashboardServer represents the HTTP server for the dashboard
type DashboardServer struct {
	config   DashboardConfig
	dbLogger *DBLogger
	server   *http.Server
	wg       sync.WaitGroup
}

// DashboardConfig holds configuration for the dashboard server
type DashboardConfig struct {
	Port            int
	DBLogPath       string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	MaxHeaderBytes  int
	ShutdownTimeout int
	Debug           bool
	LogLevel        zerolog.Level
}

// LoadDashboardConfigFromEnv loads configuration for the dashboard from environment variables
func LoadDashboardConfigFromEnv() DashboardConfig {
	// Default configuration
	config := DashboardConfig{
		Port:            getEnvAsInt("DASHBOARD_PORT", 8081),
		DBLogPath:       getEnv("DB_LOG_PATH", ""),
		ReadTimeout:     time.Duration(getEnvAsInt("DASHBOARD_READ_TIMEOUT", 30)) * time.Second,
		WriteTimeout:    time.Duration(getEnvAsInt("DASHBOARD_WRITE_TIMEOUT", 30)) * time.Second,
		IdleTimeout:     time.Duration(getEnvAsInt("DASHBOARD_IDLE_TIMEOUT", 60)) * time.Second,
		MaxHeaderBytes:  getEnvAsInt("DASHBOARD_MAX_HEADER_BYTES", 1<<20), // 1MB
		ShutdownTimeout: getEnvAsInt("DASHBOARD_SHUTDOWN_TIMEOUT", 30),
		LogLevel:        getLogLevel(getEnv("DASHBOARD_LOG_LEVEL", "info")),
	}

	// Support DEBUG environment variable
	if getEnvAsBool("DASHBOARD_DEBUG", false) {
		config.Debug = true
		config.LogLevel = zerolog.DebugLevel
	}

	return config
}

// NewDashboardServer creates a new dashboard server
func NewDashboardServer(config DashboardConfig, dbLogger *DBLogger) *DashboardServer {
	dashboard := &DashboardServer{
		config:   config,
		dbLogger: dbLogger,
	}

	// Create HTTP server with proper timeouts
	mux := http.NewServeMux()

	// Register dashboard routes
	mux.HandleFunc("/dashboard", dashboard.handleDashboard)
	mux.HandleFunc("/dashboard/logs", dashboard.handleLogs)
	mux.HandleFunc("/dashboard/stats", dashboard.handleStats)
	mux.HandleFunc("/dashboard/api/logs", dashboard.handleLogsAPI)
	mux.HandleFunc("/dashboard/api/stats", dashboard.handleStatsAPI)

	dashboard.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", config.Port),
		Handler:        mux,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		IdleTimeout:    config.IdleTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}

	return dashboard
}

// Start starts the HTTP server
func (ds *DashboardServer) Start() error {
	log.Info().Int("port", ds.config.Port).Msg("Starting Dashboard server")
	return ds.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (ds *DashboardServer) Shutdown(ctx context.Context) error {
	return ds.server.Shutdown(ctx)
}

// handleDashboard handles the main dashboard page
func (ds *DashboardServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	renderDashboardTemplate(w)
}

// handleLogs handles requests to view logs in HTML format
func (ds *DashboardServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	if ds.dbLogger == nil || !ds.dbLogger.enabled {
		http.Error(w, "Logging not enabled", http.StatusNotFound)
		return
	}
	renderLogsTemplate(w)
}

// handleStats handles requests to view stats in HTML format
func (ds *DashboardServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if ds.dbLogger == nil || !ds.dbLogger.enabled {
		http.Error(w, "Logging not enabled", http.StatusNotFound)
		return
	}
	renderStatsTemplate(w)
}

// handleLogsAPI handles API requests for log data
func (ds *DashboardServer) handleLogsAPI(w http.ResponseWriter, r *http.Request) {
	if ds.dbLogger == nil || !ds.dbLogger.enabled {
		http.Error(w, "Logging not enabled", http.StatusNotFound)
		return
	}

	handleLogsAPIRequest(w, r, ds.dbLogger)
}

// handleStatsAPI provides statistics about logged requests
func (ds *DashboardServer) handleStatsAPI(w http.ResponseWriter, r *http.Request) {
	if ds.dbLogger == nil || !ds.dbLogger.enabled {
		http.Error(w, "Logging not enabled", http.StatusNotFound)
		return
	}

	handleStatsAPIRequest(w, r, ds.dbLogger)
}
