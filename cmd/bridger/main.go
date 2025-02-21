package main

import (
    "context"
    "crypto/subtle"
    "encoding/json"
    "flag"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "runtime"
    "syscall"
    "time"

    "bridger/internal/bridge"
    "bridger/internal/config"
    "bridger/internal/logger"
    "bridger/internal/metrics"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Build information. These will be set by the build script
var (
    version   = "dev"
    gitCommit = "none"
    buildTime = "unknown"
)

func main() {
    // Parse command line flags
    configPath := flag.String("config", "config/config.json", "Path to configuration file")
    flag.Parse()

    // Load configuration
    cfg, err := config.LoadConfig(*configPath)
    if err != nil {
        panic(fmt.Errorf("failed to load configuration: %w", err))
    }

    // Initialize logger
    log, err := logger.NewLogger(&cfg.Log)
    if err != nil {
        panic(fmt.Errorf("failed to create logger: %w", err))
    }
    defer log.Sync()

    // Initialize metrics
    reg := prometheus.NewRegistry()
    metrics, err := metrics.NewMetrics(reg)
    if err != nil {
        log.Fatal("failed to create metrics",
            "error", err)
    }

    // Create and start bridge
    bridge, err := bridge.NewBridge(cfg, log, metrics, version, gitCommit, buildTime)
    if err != nil {
        log.Fatal("failed to create bridge",
            "error", err)
    }

    if err := bridge.Start(); err != nil {
        log.Fatal("failed to start bridge",
            "error", err)
    }

    // Start metrics server if enabled
    var metricsServer *http.Server
    if cfg.Metrics.Enabled {
        metricsServer = startMetricsServer(cfg, log, reg, metrics, bridge)
    }

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    // Graceful shutdown
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if metricsServer != nil {
        log.Info("stopping metrics server")
        if err := metricsServer.Shutdown(shutdownCtx); err != nil {
            log.Error("metrics server shutdown error",
                "error", err)
        }
    }

    bridge.Stop()
    log.Info("bridge shutdown complete")
}

func startMetricsServer(cfg *config.Config, log *logger.Logger, reg *prometheus.Registry, metrics *metrics.Metrics, bridge *bridge.Bridge) *http.Server {
    mux := http.NewServeMux()

    // Metrics endpoint with optional auth
    var metricsHandler http.Handler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
    if cfg.Metrics.BasicAuth {
        metricsHandler = basicAuth(metricsHandler, cfg.Metrics.Username, cfg.Metrics.Password)
    }
    mux.Handle(cfg.Metrics.Path, metricsHandler)

    // Status endpoint
    mux.HandleFunc("/status", statusHandler(metrics))

    // Health check endpoint
    mux.HandleFunc("/health", healthHandler(bridge))

    server := &http.Server{
        Addr:         cfg.Metrics.Address,
        Handler:      mux,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
    }

    go func() {
        log.Info("starting metrics server",
            "address", cfg.Metrics.Address,
            "metricsPath", cfg.Metrics.Path)

        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Error("metrics server error",
                "error", err)
        }
    }()

    return server
}

func statusHandler(metrics *metrics.Metrics) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        stats := metrics.GetStats()
        status := map[string]interface{}{
            "version": map[string]string{
                "version":   version,
                "commit":    gitCommit,
                "buildTime": buildTime,
            },
            "metrics": stats,
            "system": map[string]interface{}{
                "goroutines":    runtime.NumGoroutine(),
                "numCPU":        runtime.NumCPU(),
                "goVersion":     runtime.Version(),
            },
        }

        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(status); err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Failed to encode status response",
            })
        }
    }
}

func healthHandler(bridge *bridge.Bridge) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        sourceOk, destOk := bridge.IsConnected()
        health := map[string]interface{}{
            "status": map[string]bool{
                "source":      sourceOk,
                "destination": destOk,
            },
            "timestamp": time.Now().UTC(),
        }

        w.Header().Set("Content-Type", "application/json")
        
        if !sourceOk || !destOk {
            w.WriteHeader(http.StatusServiceUnavailable)
        } else {
            w.WriteHeader(http.StatusOK)
        }

        if err := json.NewEncoder(w).Encode(health); err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Failed to encode health response",
            })
        }
    }
}

func basicAuth(handler http.Handler, username, password string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user, pass, ok := r.BasicAuth()

        if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || 
           subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
            w.Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
            w.WriteHeader(http.StatusUnauthorized)
            w.Write([]byte("Unauthorized\n"))
            return
        }

        handler.ServeHTTP(w, r)
    })
}
