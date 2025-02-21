package logger

import (
    "fmt"
    "bridger/internal/config"

    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger to provide application-specific logging
type Logger struct {
    *zap.SugaredLogger
}

// NewLogger creates a new logger instance
func NewLogger(cfg *config.LogConfig) (*Logger, error) {
    if cfg == nil {
        return nil, fmt.Errorf("logger config is nil")
    }

    // Parse log level
    var level zapcore.Level
    switch cfg.Level {
    case "debug":
        level = zap.DebugLevel
    case "info":
        level = zap.InfoLevel
    case "warn":
        level = zap.WarnLevel
    case "error":
        level = zap.ErrorLevel
    default:
        level = zap.InfoLevel
    }

    // Create zap config
    zapCfg := zap.Config{
        Level:       zap.NewAtomicLevelAt(level),
        Development: false,
        Sampling: &zap.SamplingConfig{
            Initial:    100,
            Thereafter: 100,
        },
        Encoding:         cfg.Encoding,
        EncoderConfig:    zap.NewProductionEncoderConfig(),
        OutputPaths:      []string{cfg.OutputPath},
        ErrorOutputPaths: []string{cfg.OutputPath},
    }

    // Customize encoder config
    zapCfg.EncoderConfig.TimeKey = "timestamp"
    zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    zapCfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
    zapCfg.EncoderConfig.StacktraceKey = "stacktrace"

    logger, err := zapCfg.Build(
        zap.AddCallerSkip(1),
        zap.AddStacktrace(zapcore.ErrorLevel),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create logger: %w", err)
    }

    return &Logger{logger.Sugar()}, nil
}

// Debug logs a message at Debug level
func (l *Logger) Debug(msg string, args ...interface{}) {
    l.SugaredLogger.Debugw(msg, args...)
}

// Info logs a message at Info level
func (l *Logger) Info(msg string, args ...interface{}) {
    l.SugaredLogger.Infow(msg, args...)
}

// Warn logs a message at Warn level
func (l *Logger) Warn(msg string, args ...interface{}) {
    l.SugaredLogger.Warnw(msg, args...)
}

// Error logs a message at Error level
func (l *Logger) Error(msg string, args ...interface{}) {
    l.SugaredLogger.Errorw(msg, args...)
}

// Fatal logs a message at Fatal level and exits
func (l *Logger) Fatal(msg string, args ...interface{}) {
    l.SugaredLogger.Fatalw(msg, args...)
}

// With returns a logger with additional structured context
func (l *Logger) With(args ...interface{}) *Logger {
    return &Logger{l.SugaredLogger.With(args...)}
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
    return l.SugaredLogger.Sync()
}
