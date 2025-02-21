package config

import (
    "encoding/json"
    "fmt"
    "os"
)

// Config holds the complete bridge configuration
type Config struct {
    Source      BrokerConfig  `json:"source"`
    Destination BrokerConfig  `json:"destination"`
    TopicMap    []TopicMap    `json:"topic_map"`
    Log         LogConfig     `json:"log"`
    Metrics     MetricsConfig `json:"metrics"`
}

// MetricsConfig holds metrics server configuration
type MetricsConfig struct {
    Enabled    bool   `json:"enabled"`
    Address    string `json:"address"`
    Path       string `json:"path"`
    BasicAuth  bool   `json:"basic_auth"`
    Username   string `json:"username,omitempty"`
    Password   string `json:"password,omitempty"`
}

// BrokerConfig holds MQTT broker connection settings
type BrokerConfig struct {
    Broker   string     `json:"broker"`
    ClientID string     `json:"client_id"`
    Username string     `json:"username"`
    Password string     `json:"password"`
    TLS      TLSConfig `json:"tls"`
}

// TLSConfig holds TLS/SSL configuration
type TLSConfig struct {
    Enable   bool   `json:"enable"`
    CertFile string `json:"cert_file"`
    KeyFile  string `json:"key_file"`
    CAFile   string `json:"ca_file"`
}

// TopicMap defines a mapping between source and destination topics
type TopicMap struct {
    Source      string `json:"source"`
    Destination string `json:"destination"`
}

// LogConfig holds logging configuration
type LogConfig struct {
    Level      string `json:"level"`      // debug, info, warn, error
    Encoding   string `json:"encoding"`   // json or console
    OutputPath string `json:"output_path"` // file path or "stdout"
}

// LoadConfig loads and validates the configuration from a file
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }

    if err := validateConfig(&cfg); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }

    // Set defaults
    setDefaults(&cfg)

    return &cfg, nil
}

// validateConfig performs configuration validation
func validateConfig(cfg *Config) error {
    if cfg.Source.Broker == "" {
        return fmt.Errorf("source broker address is required")
    }
    if cfg.Destination.Broker == "" {
        return fmt.Errorf("destination broker address is required")
    }
    if len(cfg.TopicMap) == 0 {
        return fmt.Errorf("at least one topic mapping is required")
    }

    // Validate metrics config
    if cfg.Metrics.Enabled {
        if cfg.Metrics.Address == "" {
            return fmt.Errorf("metrics address is required when metrics are enabled")
        }
        if cfg.Metrics.BasicAuth && (cfg.Metrics.Username == "" || cfg.Metrics.Password == "") {
            return fmt.Errorf("username and password are required when basic auth is enabled")
        }
    }

    // Validate log configuration
    if cfg.Log.Level != "" && !isValidLogLevel(cfg.Log.Level) {
        return fmt.Errorf("invalid log level: %s", cfg.Log.Level)
    }
    if cfg.Log.Encoding != "" && !isValidLogEncoding(cfg.Log.Encoding) {
        return fmt.Errorf("invalid log encoding: %s", cfg.Log.Encoding)
    }

    // Validate topic mappings
    for i, tm := range cfg.TopicMap {
        if tm.Source == "" {
            return fmt.Errorf("source topic is required for mapping %d", i)
        }
        if tm.Destination == "" {
            return fmt.Errorf("destination topic is required for mapping %d", i)
        }
    }

    return nil
}

// setDefaults sets default values for optional configuration
func setDefaults(cfg *Config) {
    // Log defaults
    if cfg.Log.Level == "" {
        cfg.Log.Level = "info"
    }
    if cfg.Log.Encoding == "" {
        cfg.Log.Encoding = "json"
    }
    if cfg.Log.OutputPath == "" {
        cfg.Log.OutputPath = "stdout"
    }

    // Metrics defaults
    if cfg.Metrics.Enabled && cfg.Metrics.Path == "" {
        cfg.Metrics.Path = "/metrics"
    }
}

// isValidLogLevel checks if the log level is valid
func isValidLogLevel(level string) bool {
    switch level {
    case "debug", "info", "warn", "error":
        return true
    default:
        return false
    }
}

// isValidLogEncoding checks if the log encoding is valid
func isValidLogEncoding(encoding string) bool {
    switch encoding {
    case "json", "console":
        return true
    default:
        return false
    }
}
