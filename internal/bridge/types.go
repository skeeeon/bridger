package bridge

import (
    "time"
)

// Message represents a single MQTT message with its metadata
type Message struct {
    Topic     string
    Payload   []byte
    Mapper    *TopicMatcher
    Timestamp time.Time
}

// Batch represents a group of messages to be processed together
type Batch struct {
    Messages  []Message
    Size      int
    Topic     string
}

// WorkerConfig defines the configuration for the worker pool
type WorkerConfig struct {
    NumWorkers      int           
    BatchSize       int           
    BatchTimeout    time.Duration 
    QueueSize       int           
}

// BreakerConfig defines the configuration for the circuit breaker
type BreakerConfig struct {
    Name            string
    MaxFailures     uint32
    Timeout         time.Duration
    MaxRequests     uint32
    Interval        time.Duration
}

// FromConfig creates a WorkerConfig from the configuration
func NewWorkerConfigFromConfig(numWorkers, batchSize, batchTimeoutMs, queueSize int) WorkerConfig {
    return WorkerConfig{
        NumWorkers:   numWorkers,
        BatchSize:    batchSize,
        BatchTimeout: time.Duration(batchTimeoutMs) * time.Millisecond,
        QueueSize:    queueSize,
    }
}

// NewBreakerConfig creates a BreakerConfig from the configuration
func NewBreakerConfig(name string, maxFailures int, timeoutSec int, maxRequests int, intervalSec int) BreakerConfig {
    return BreakerConfig{
        Name:        name,
        MaxFailures: uint32(maxFailures),
        Timeout:     time.Duration(timeoutSec) * time.Second,
        MaxRequests: uint32(maxRequests),
        Interval:    time.Duration(intervalSec) * time.Second,
    }
}
