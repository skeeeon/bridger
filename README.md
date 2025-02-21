# MQTT Bridge

MQTT Bridge is a high-performance message forwarding service that connects two MQTT brokers and forwards messages between them based on configurable topic mappings. It supports wildcard topics, connection recovery, metrics, and structured logging.

## Features

- Forward messages between two MQTT brokers
- Support for MQTT topic wildcards (`+` and `#`)
- Flexible topic mapping and transformation
- TLS/SSL support for secure connections
- Automatic connection recovery
- Topic resubscription on reconnection
- Structured JSON logging
- Prometheus metrics
- Health and status monitoring
- Build version tracking
- Docker support
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

## Metrics and Monitoring

The bridge provides comprehensive metrics and monitoring capabilities through several endpoints:

### Prometheus Metrics

Available at `/metrics` (configurable path), includes:

- **Message Metrics**
  - `bridge_messages_received_total`: Total messages received
  - `bridge_messages_forwarded_total`: Total messages forwarded
  - `bridge_bytes_received_total`: Total bytes received
  - `bridge_bytes_forwarded_total`: Total bytes forwarded
  - `bridge_processing_errors_total`: Total processing errors

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

Available at `/status`, provides detailed operational status:

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
        "processing_errors": 0,
        "average_latency_us": 1234
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

## Usage

1. Create your configuration file:
```bash
cp config/config.example.json config/config.json
# Edit config.json with your broker details
```

2. Run the bridge:
```bash
./mqtt-bridge -config config/config.json
```

## Development

### Project Structure

```
mqtt-bridge/
├── cmd/
│   └── mqtt-bridge/        # Main application
├── internal/
│   ├── bridge/            # Bridge implementation
│   ├── config/            # Configuration handling
│   ├── logger/            # Logging package
│   └── metrics/           # Metrics collection
├── config/                # Configuration files
```

### Building

```bash
# Build binary
go build -o mqtt-bridge ./cmd/mqtt-bridge
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
