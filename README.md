# MQTT Bridge

MQTT Bridge is a high-performance message forwarding service that connects two MQTT brokers and forwards messages between them based on configurable topic mappings. It supports wildcard topics, connection recovery, metrics, and structured logging.

## Features

- Forward messages between two MQTT brokers
- Support for MQTT topic wildcards (`+` and `#`)
- Flexible topic mapping and transformation
- High-performance message processing:
  - Parallel processing with configurable worker pool
  - Message batching by topic
  - Circuit breaker for failure protection
- TLS/SSL support for secure connections
- Automatic connection recovery
- Topic resubscription on reconnection
- Structured JSON logging
- Comprehensive Prometheus metrics
- Health and status monitoring
- Build version tracking
- Clean shutdown handling

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/mqtt-bridge.git
cd mqtt-bridge

# Build the binary
go build -o mqtt-bridge ./cmd/mqtt-bridge
```

## Configuration

Configuration is provided via a JSON file. Here's an example:

```json
{
    "source": {
        "broker": "tcp://source-broker:1883",
        "client_id": "mqtt-bridge-source",
        "username": "source_user",
        "password": "source_pass",
        "tls": {
            "enable": false
        }
    },
    "destination": {
        "broker": "tcp://dest-broker:1883",
        "client_id": "mqtt-bridge-dest",
        "username": "dest_user",
        "password": "dest_pass",
        "tls": {
            "enable": false
        }
    },
    "topic_map": [
        {
            "source": "sensors/+/temperature",
            "destination": "forwarded/+/temp"
        },
        {
            "source": "devices/#",
            "destination": "forwarded/devices/#"
        }
    ],
    "performance": {
        "worker_pool": {
            "num_workers": 8,         // Number of worker goroutines (defaults to CPU count if 0)
            "batch_size": 100,        // Maximum messages per batch
            "batch_timeout_ms": 100,  // Maximum time to wait before processing a partial batch
            "queue_size": 1000        // Size of the message queue
        },
        "circuit_breaker": {
            "max_failures": 5,         // Number of failures before opening
            "timeout_seconds": 60,     // How long to wait before half-open
            "max_requests": 2,         // Maximum requests in half-open state
            "interval_seconds": 60     // Time window for failure counting
        }
    },
    "log": {
        "level": "info",
        "encoding": "json",
        "output_path": "bridge.log"
    },
    "metrics": {
        "enabled": true,
        "address": ":8080",
        "path": "/metrics",
        "basic_auth": true,
        "username": "metrics",
        "password": "secret"
    }
}
```

## Performance Features

### Worker Pool
- Parallel message processing scaled to available CPU cores or configured count
- Message queuing with configurable size for backpressure handling
- Graceful shutdown with remaining message processing

### Message Batching
- Groups messages by destination topic
- Configurable batch size and timeout
- Reduces broker connection overhead
- Optimized payload combining

### Circuit Breaker
- Protects against destination broker failures
- Configurable failure thresholds and recovery periods
- Three states: Closed (normal), Half-Open (testing), Open (failing)
- Automatic state management based on failure rates

## Metrics and Monitoring

The bridge provides comprehensive metrics through several endpoints:

### Prometheus Metrics

Available at `/metrics` (configurable path), includes:

- **Message Metrics**
  - `bridge_messages_received_total`: Total messages received
  - `bridge_messages_forwarded_total`: Total messages forwarded
  - `bridge_messages_dropped_total`: Messages dropped due to queue full
  - `bridge_bytes_received_total`: Total bytes received
  - `bridge_bytes_forwarded_total`: Total bytes forwarded
  - `bridge_processing_errors_total`: Total processing errors

- **Batch Metrics**
  - `bridge_batches_processed_total`: Total batches processed
  - `bridge_batches_dropped_total`: Batches dropped due to queue full
  - `bridge_batch_size`: Distribution of batch sizes
  - `bridge_batch_latency_seconds`: Batch processing latency

- **Connection Metrics**
  - `bridge_source_connected`: Source broker connection status (0/1)
  - `bridge_destination_connected`: Destination broker connection status (0/1)
  - `bridge_source_reconnects_total`: Source broker reconnection count
  - `bridge_dest_reconnects_total`: Destination broker reconnection count

- **System Metrics**
  - `bridge_goroutines`: Current number of goroutines
  - `bridge_memory_bytes`: Current memory usage
  - `bridge_build_info`: Build information (version, commit, build time)

Example:
```bash
# Get Prometheus metrics
curl -u metrics:secret http://localhost:8080/metrics
```

### Status Endpoint

Available at `/status`, provides operational status:

```bash
curl http://localhost:8080/status
```

Response includes:
```json
{
    "version": {
        "version": "1.0.0",
        "commit": "abc123",
        "buildTime": "2024-02-20T10:00:00Z"
    },
    "metrics": {
        "uptime_seconds": 3600,
        "messages_received": 1000,
        "messages_forwarded": 1000,
        "messages_dropped": 0,
        "processing_errors": 0,
        "average_latency_us": 1234,
        "batches_processed": 100,
        "batches_dropped": 0,
        "breaker_trips": 0
    },
    "system": {
        "goroutines": 10,
        "numCPU": 4,
        "goVersion": "go1.21"
    }
}
```

### Health Check

Available at `/health`, indicates operational status:

```bash
curl -i http://localhost:8080/health
```

Response includes:
```json
{
    "status": {
        "source": true,
        "destination": true
    },
    "timestamp": "2024-02-20T10:00:00Z"
}
```

HTTP Status Codes:
- 200: Both brokers connected
- 503: One or both brokers disconnected

## Project Structure

```
mqtt-bridge/
├── cmd/
│   └── mqtt-bridge/        # Main application
├── internal/
│   ├── bridge/            # Bridge implementation
│   │   ├── breaker.go     # Circuit breaker
│   │   ├── worker.go      # Worker pool and batching
│   │   ├── bridge.go      # Core bridge logic
│   │   ├── types.go       # Common types
│   │   └── topic_matcher.go # Topic matching logic
│   ├── config/            # Configuration handling
│   ├── logger/            # Logging package
│   └── metrics/           # Metrics collection
├── config/                # Configuration files
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
