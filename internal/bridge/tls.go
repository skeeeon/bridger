package bridge

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "os"

    "bridger/internal/config"
)

// createTLSConfig creates a TLS configuration from the provided config
func createTLSConfig(cfg config.TLSConfig) (*tls.Config, error) {
    // Create TLS config
    tlsConfig := &tls.Config{
        MinVersion: tls.VersionTLS12,
    }

    // Load client certificate if provided
    if cfg.CertFile != "" && cfg.KeyFile != "" {
        cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
        if err != nil {
            return nil, fmt.Errorf("failed to load client certificate: %w", err)
        }
        tlsConfig.Certificates = []tls.Certificate{cert}
    }

    // Load CA certificate if provided
    if cfg.CAFile != "" {
        caCert, err := os.ReadFile(cfg.CAFile)
        if err != nil {
            return nil, fmt.Errorf("failed to read CA certificate: %w", err)
        }

        caCertPool := x509.NewCertPool()
        if !caCertPool.AppendCertsFromPEM(caCert) {
            return nil, fmt.Errorf("failed to parse CA certificate")
        }
        tlsConfig.RootCAs = caCertPool
    }

    return tlsConfig, nil
}
