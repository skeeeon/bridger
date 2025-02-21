package metrics

import (
    "runtime"
    "sync/atomic"
    "time"

    "github.com/prometheus/client_golang/prometheus"
)

// Stats holds internal performance metrics
type Stats struct {
    MessagesReceived   uint64
    MessagesForwarded  uint64
    BytesReceived      uint64
    BytesForwarded     uint64
    ProcessingErrors   uint64
    SourceReconnects   uint64
    DestReconnects     uint64
    LastProcessingTime int64  // nanoseconds
    AverageLatency     uint64 // microseconds
}

// Metrics manages performance metrics collection
type Metrics struct {
    stats     Stats
    startTime time.Time

    // Message metrics
    messagesReceived   prometheus.Counter
    messagesForwarded  prometheus.Counter
    bytesReceived      prometheus.Counter
    bytesForwarded     prometheus.Counter
    processingErrors   prometheus.Counter
    sourceReconnects   prometheus.Counter
    destReconnects     prometheus.Counter

    // Connection status
    sourceConnected    prometheus.Gauge
    destinationConnected prometheus.Gauge

    // System metrics
    goroutineCount    prometheus.Gauge
    memoryUsage       prometheus.Gauge

    // Build information
    buildInfo         *prometheus.GaugeVec
}

// NewMetrics creates a new metrics collector
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
    m := &Metrics{
        startTime: time.Now(),

        messagesReceived: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_messages_received_total",
            Help: "Total number of messages received",
        }),

        messagesForwarded: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_messages_forwarded_total",
            Help: "Total number of messages forwarded",
        }),

        bytesReceived: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_bytes_received_total",
            Help: "Total bytes received",
        }),

        bytesForwarded: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_bytes_forwarded_total",
            Help: "Total bytes forwarded",
        }),

        processingErrors: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_processing_errors_total",
            Help: "Total number of processing errors",
        }),

        sourceReconnects: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_source_reconnects_total",
            Help: "Total number of source broker reconnections",
        }),

        destReconnects: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_dest_reconnects_total",
            Help: "Total number of destination broker reconnections",
        }),

        sourceConnected: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bridge_source_connected",
            Help: "Source broker connection status (0/1)",
        }),

        destinationConnected: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bridge_destination_connected",
            Help: "Destination broker connection status (0/1)",
        }),

        goroutineCount: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bridge_goroutines",
            Help: "Current number of goroutines",
        }),

        memoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bridge_memory_bytes",
            Help: "Current memory usage in bytes",
        }),

        buildInfo: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "bridge_build_info",
                Help: "Build information",
            },
            []string{"version", "commit", "build_time"},
        ),
    }

    // Register all metrics
    metrics := []prometheus.Collector{
        m.messagesReceived,
        m.messagesForwarded,
        m.bytesReceived,
        m.bytesForwarded,
        m.processingErrors,
        m.sourceReconnects,
        m.destReconnects,
        m.sourceConnected,
        m.destinationConnected,
        m.goroutineCount,
        m.memoryUsage,
        m.buildInfo,
    }

    for _, metric := range metrics {
        if err := reg.Register(metric); err != nil {
            return nil, err
        }
    }

    return m, nil
}

// RecordMessage records metrics for a processed message
func (m *Metrics) RecordMessage(bytesReceived, bytesForwarded int, processingTime time.Duration) {
    // Update internal stats
    atomic.AddUint64(&m.stats.MessagesReceived, 1)
    atomic.AddUint64(&m.stats.MessagesForwarded, 1)
    atomic.AddUint64(&m.stats.BytesReceived, uint64(bytesReceived))
    atomic.AddUint64(&m.stats.BytesForwarded, uint64(bytesForwarded))
    atomic.StoreInt64(&m.stats.LastProcessingTime, int64(processingTime))

    // Update average latency
    current := atomic.LoadUint64(&m.stats.AverageLatency)
    newLatency := uint64(processingTime.Microseconds())
    if current == 0 {
        atomic.StoreUint64(&m.stats.AverageLatency, newLatency)
    } else {
        atomic.StoreUint64(&m.stats.AverageLatency, (current+newLatency)/2)
    }

    // Update Prometheus metrics
    m.messagesReceived.Inc()
    m.messagesForwarded.Inc()
    m.bytesReceived.Add(float64(bytesReceived))
    m.bytesForwarded.Add(float64(bytesForwarded))
}

// RecordError increments the error counter
func (m *Metrics) RecordError() {
    atomic.AddUint64(&m.stats.ProcessingErrors, 1)
    m.processingErrors.Inc()
}

// RecordReconnect increments the reconnection counters
func (m *Metrics) RecordReconnect(isSource bool) {
    if isSource {
        atomic.AddUint64(&m.stats.SourceReconnects, 1)
        m.sourceReconnects.Inc()
    } else {
        atomic.AddUint64(&m.stats.DestReconnects, 1)
        m.destReconnects.Inc()
    }
}

// SetConnectionStatus updates the connection status metrics
func (m *Metrics) SetConnectionStatus(isSource bool, connected bool) {
    if isSource {
        m.sourceConnected.Set(boolToFloat64(connected))
    } else {
        m.destinationConnected.Set(boolToFloat64(connected))
    }
}

// UpdateSystemMetrics updates system-level metrics
func (m *Metrics) UpdateSystemMetrics() {
    m.goroutineCount.Set(float64(runtime.NumGoroutine()))
    
    var mem runtime.MemStats
    runtime.ReadMemStats(&mem)
    m.memoryUsage.Set(float64(mem.Alloc))
}

// SetBuildInfo sets the build information metric
func (m *Metrics) SetBuildInfo(version, commit, buildTime string) {
    m.buildInfo.WithLabelValues(version, commit, buildTime).Set(1)
}

// GetStats returns the current metrics
func (m *Metrics) GetStats() map[string]interface{} {
    uptime := time.Since(m.startTime)
    received := atomic.LoadUint64(&m.stats.MessagesReceived)

    var mem runtime.MemStats
    runtime.ReadMemStats(&mem)

    return map[string]interface{}{
        "uptime_seconds":      uptime.Seconds(),
        "messages_received":   received,
        "messages_forwarded":  atomic.LoadUint64(&m.stats.MessagesForwarded),
        "bytes_received":      atomic.LoadUint64(&m.stats.BytesReceived),
        "bytes_forwarded":     atomic.LoadUint64(&m.stats.BytesForwarded),
        "processing_errors":   atomic.LoadUint64(&m.stats.ProcessingErrors),
        "source_reconnects":   atomic.LoadUint64(&m.stats.SourceReconnects),
        "dest_reconnects":     atomic.LoadUint64(&m.stats.DestReconnects),
        "average_latency_us":  atomic.LoadUint64(&m.stats.AverageLatency),
        "messages_per_second": float64(received) / uptime.Seconds(),
        "system": map[string]interface{}{
            "goroutines":    runtime.NumGoroutine(),
            "memory_bytes":  mem.Alloc,
        },
    }
}

func boolToFloat64(b bool) float64 {
    if b {
        return 1
    }
    return 0
}
