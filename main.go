package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Define command line flags
	var proxyOnly bool
	var dashboardOnly bool

	flag.BoolVar(&proxyOnly, "proxy-only", false, "Run only the proxy server")
	flag.BoolVar(&dashboardOnly, "dashboard-only", false, "Run only the dashboard server")
	flag.Parse()

	// Load configuration
	proxyConfig := LoadConfigFromEnv()
	dashboardConfig := LoadDashboardConfigFromEnv()

	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Determine log level - use highest verbosity level from either config
	logLevel := proxyConfig.LogLevel
	if dashboardConfig.LogLevel < logLevel {
		logLevel = dashboardConfig.LogLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Use pretty logging for development
	if proxyConfig.Debug || dashboardConfig.Debug || logLevel == zerolog.DebugLevel || logLevel == zerolog.TraceLevel {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	// Initialize DB logger if enabled
	var dbLogger *DBLogger
	var err error

	dbLogPath := proxyConfig.DBLogPath
	dbMaxEntries := proxyConfig.DBMaxEntries

	// If dashboard only, use dashboard config values
	if dashboardOnly && !proxyOnly {
		dbLogPath = dashboardConfig.DBLogPath
	}

	if dbLogPath != "" && dbMaxEntries > 0 {
		dbLogger, err = NewDBLogger(dbLogPath, dbMaxEntries)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize DB logger")
		} else {
			log.Info().Str("path", dbLogPath).Int("maxEntries", dbMaxEntries).
				Msg("DB logger initialized successfully")
			defer dbLogger.Close()
		}
	} else {
		log.Info().Msg("DB logging disabled")
	}

	// Start the appropriate server(s) based on command line flags
	if dashboardOnly && proxyOnly {
		log.Fatal().Msg("Cannot specify both -proxy-only and -dashboard-only")
	}

	// Create a WaitGroup to manage server shutdown
	var wg sync.WaitGroup
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	// Start appropriate servers based on flags
	if dashboardOnly {
		// Dashboard-only mode
		log.Info().
			Int("dashboardPort", dashboardConfig.Port).
			Str("dbLogPath", dbLogPath).
			Bool("debug", dashboardConfig.Debug).
			Str("logLevel", dashboardConfig.LogLevel.String()).
			Msg("Starting Dashboard server only")

		runDashboardOnly(dashboardConfig, dbLogger, shutdownCh)
	} else if proxyOnly {
		// Proxy-only mode
		log.Info().
			Str("redisAddr", proxyConfig.RedisAddr).
			Int("redisPoolSize", proxyConfig.RedisPoolSize).
			Int("proxyPort", proxyConfig.Port).
			Str("fixedTopic", proxyConfig.FixedTopic).
			Int("respondImmediately", proxyConfig.RespondImmediatelyStatus).
			Int("responseTimeout", proxyConfig.ResponseTimeout).
			Bool("debug", proxyConfig.Debug).
			Str("logLevel", proxyConfig.LogLevel.String()).
			Msg("Starting Redis proxy server only")

		runProxyOnly(proxyConfig, dbLogger, shutdownCh, &wg)
	} else {
		// Default: run both servers
		log.Info().
			Str("redisAddr", proxyConfig.RedisAddr).
			Int("redisPoolSize", proxyConfig.RedisPoolSize).
			Int("proxyPort", proxyConfig.Port).
			Int("dashboardPort", dashboardConfig.Port).
			Str("fixedTopic", proxyConfig.FixedTopic).
			Int("respondImmediately", proxyConfig.RespondImmediatelyStatus).
			Int("responseTimeout", proxyConfig.ResponseTimeout).
			Bool("debug", proxyConfig.Debug || dashboardConfig.Debug).
			Str("logLevel", logLevel.String()).
			Str("dbLogPath", dbLogPath).
			Int("dbMaxEntries", dbMaxEntries).
			Msg("Starting Redis proxy and Dashboard servers")

		runBothServers(proxyConfig, dashboardConfig, dbLogger, shutdownCh, &wg)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	log.Info().Msg("All servers stopped. Exiting.")
}

// runDashboardOnly runs just the dashboard server
func runDashboardOnly(config DashboardConfig, dbLogger *DBLogger, shutdownCh chan os.Signal) {
	// Create and start the dashboard server
	dashboardServer := NewDashboardServer(config, dbLogger)

	// Start in a goroutine for signal handling
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- dashboardServer.Start()
	}()

	// Wait for a shutdown signal or server error
	select {
	case err := <-serverErrCh:
		if err != nil && err.Error() != "http: Server closed" {
			log.Error().Err(err).Msg("Dashboard server error")
		}
	case <-shutdownCh:
		log.Info().Msg("Shutdown signal received, stopping dashboard server...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.ShutdownTimeout)*time.Second)
		defer cancel()

		if err := dashboardServer.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Dashboard server shutdown error")
		}
		log.Info().Msg("Dashboard server gracefully stopped")
	}
}

// runProxyOnly runs just the proxy server
func runProxyOnly(config Config, dbLogger *DBLogger, shutdownCh chan os.Signal, wg *sync.WaitGroup) {
	// Set up Redis manager
	redisManager, err := NewRedisManager(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisManager.Close()

	// Create and start the proxy server
	proxyServer := NewProxyServer(config, redisManager, wg, dbLogger)

	// Start in a goroutine for signal handling
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- proxyServer.Start()
	}()

	// Wait for a shutdown signal or server error
	select {
	case err := <-serverErrCh:
		if err != nil && err.Error() != "http: Server closed" {
			log.Error().Err(err).Msg("Proxy server error")
		}
	case <-shutdownCh:
		log.Info().Msg("Shutdown signal received, stopping proxy server...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.ShutdownTimeout)*time.Second)
		defer cancel()

		if err := proxyServer.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Proxy server shutdown error")
		}
		log.Info().Msg("Proxy server gracefully stopped")
	}
}

// runBothServers runs both the proxy and dashboard servers
func runBothServers(proxyConfig Config, dashboardConfig DashboardConfig, dbLogger *DBLogger, shutdownCh chan os.Signal, wg *sync.WaitGroup) {
	// Set up Redis manager
	redisManager, err := NewRedisManager(proxyConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisManager.Close()

	// Create servers
	proxyServer := NewProxyServer(proxyConfig, redisManager, wg, dbLogger)
	dashboardServer := NewDashboardServer(dashboardConfig, dbLogger)

	// Start servers in separate goroutines
	proxyErrCh := make(chan error, 1)
	dashboardErrCh := make(chan error, 1)

	go func() {
		log.Info().Int("port", proxyConfig.Port).Msg("Starting HTTP proxy server")
		proxyErrCh <- proxyServer.Start()
	}()

	go func() {
		log.Info().Int("port", dashboardConfig.Port).Msg("Starting Dashboard server")
		dashboardErrCh <- dashboardServer.Start()
	}()

	// Wait for shutdown signal or server errors
	select {
	case err := <-proxyErrCh:
		if err != nil && err.Error() != "http: Server closed" {
			log.Error().Err(err).Msg("Proxy server error, shutting down both servers")
		}
	case err := <-dashboardErrCh:
		if err != nil && err.Error() != "http: Server closed" {
			log.Error().Err(err).Msg("Dashboard server error, shutting down both servers")
		}
	case <-shutdownCh:
		log.Info().Msg("Shutdown signal received, stopping all servers...")
	}

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(proxyConfig.ShutdownTimeout)*time.Second)
	defer cancel()

	// Shutdown both servers
	if err := proxyServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Proxy server shutdown error")
	}

	if err := dashboardServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Dashboard server shutdown error")
	}

	log.Info().Msg("All servers gracefully stopped")
}
