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
    BatchesProcessed   uint64
    BatchesDropped     uint64
    MessagesDropped    uint64
    BreakerTrips       uint64
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

    // Batch metrics
    batchesProcessed  prometheus.Counter
    batchesDropped    prometheus.Counter
    batchSize        prometheus.Histogram
    batchLatency     prometheus.Histogram
    
    // Queue metrics
    queueSize        prometheus.Gauge
    messagesDropped  prometheus.Counter
    
    // Circuit breaker metrics
    breakerState     prometheus.Gauge
    breakerTrips     prometheus.Counter
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

        batchesProcessed: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_batches_processed_total",
            Help: "Total number of batches processed",
        }),

        batchesDropped: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_batches_dropped_total",
            Help: "Total number of batches dropped due to queue full",
        }),

        batchSize: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name:    "bridge_batch_size",
            Help:    "Distribution of batch sizes",
            Buckets: []float64{10, 25, 50, 75, 100, 150, 200},
        }),

        batchLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name:    "bridge_batch_latency_seconds",
            Help:    "Distribution of batch processing latencies",
            Buckets: prometheus.DefBuckets,
        }),

        queueSize: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bridge_queue_size",
            Help: "Current size of the message queue",
        }),

        messagesDropped: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_messages_dropped_total",
            Help: "Total number of messages dropped due to queue full",
        }),

        breakerState: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bridge_circuit_breaker_state",
            Help: "Current state of the circuit breaker (0=Closed, 1=Half-Open, 2=Open)",
        }),

        breakerTrips: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bridge_circuit_breaker_trips_total",
            Help: "Total number of times the circuit breaker has tripped",
        }),
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
        m.batchesProcessed,
        m.batchesDropped,
        m.batchSize,
        m.batchLatency,
        m.queueSize,
        m.messagesDropped,
        m.breakerState,
        m.breakerTrips,
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
    atomic.AddUint64(&m.stats.MessagesReceived, 1)
    atomic.AddUint64(&m.stats.MessagesForwarded, 1)
    atomic.AddUint64(&m.stats.BytesReceived, uint64(bytesReceived))
    atomic.AddUint64(&m.stats.BytesForwarded, uint64(bytesForwarded))
    atomic.StoreInt64(&m.stats.LastProcessingTime, int64(processingTime))

    current := atomic.LoadUint64(&m.stats.AverageLatency)
    newLatency := uint64(processingTime.Microseconds())
    if current == 0 {
        atomic.StoreUint64(&m.stats.AverageLatency, newLatency)
    } else {
        atomic.StoreUint64(&m.stats.AverageLatency, (current+newLatency)/2)
    }

    m.messagesReceived.Inc()
    m.messagesForwarded.Inc()
    m.bytesReceived.Add(float64(bytesReceived))
    m.bytesForwarded.Add(float64(bytesForwarded))
}

// RecordBatchProcessed records metrics for a processed batch
func (m *Metrics) RecordBatchProcessed(size int, duration time.Duration) {
    atomic.AddUint64(&m.stats.BatchesProcessed, 1)
    m.batchesProcessed.Inc()
    m.batchSize.Observe(float64(size))
    if duration > 0 {
        m.batchLatency.Observe(duration.Seconds())
    }
}

// RecordBatchDropped records metrics for a dropped batch
func (m *Metrics) RecordBatchDropped(size int) {
    atomic.AddUint64(&m.stats.BatchesDropped, 1)
    m.batchesDropped.Inc()
}

// RecordMessageDropped records metrics for a dropped message
func (m *Metrics) RecordMessageDropped() {
    atomic.AddUint64(&m.stats.MessagesDropped, 1)
    m.messagesDropped.Inc()
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

// RecordBreakerState updates the circuit breaker state metric
func (m *Metrics) RecordBreakerState(state string) {
    var stateValue float64
    switch state {
    case "closed":
        stateValue = 0
    case "half-open":
        stateValue = 1
    case "open":
        stateValue = 2
    }
    m.breakerState.Set(stateValue)
}

// RecordBreakerTripped increments the circuit breaker trip counter
func (m *Metrics) RecordBreakerTripped() {
    atomic.AddUint64(&m.stats.BreakerTrips, 1)
    m.breakerTrips.Inc()
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
        "messages_dropped":    atomic.LoadUint64(&m.stats.MessagesDropped),
        "bytes_received":      atomic.LoadUint64(&m.stats.BytesReceived),
        "bytes_forwarded":     atomic.LoadUint64(&m.stats.BytesForwarded),
        "processing_errors":   atomic.LoadUint64(&m.stats.ProcessingErrors),
        "source_reconnects":   atomic.LoadUint64(&m.stats.SourceReconnects),
        "dest_reconnects":     atomic.LoadUint64(&m.stats.DestReconnects),
        "average_latency_us":  atomic.LoadUint64(&m.stats.AverageLatency),
        "messages_per_second": float64(received) / uptime.Seconds(),
        "batches_processed":   atomic.LoadUint64(&m.stats.BatchesProcessed),
        "batches_dropped":     atomic.LoadUint64(&m.stats.BatchesDropped),
        "breaker_trips":       atomic.LoadUint64(&m.stats.BreakerTrips),
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
