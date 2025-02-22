package bridge

import (
    "github.com/sony/gobreaker"
    "bridger/internal/logger"
    "bridger/internal/metrics"
)

func NewCircuitBreaker(cfg BreakerConfig, logger *logger.Logger, metrics *metrics.Metrics) *gobreaker.CircuitBreaker {
    return gobreaker.NewCircuitBreaker(gobreaker.Settings{
        Name:        cfg.Name,
        MaxRequests: cfg.MaxRequests,
        Interval:    cfg.Interval,
        Timeout:     cfg.Timeout,
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= cfg.MaxFailures && failureRatio >= 0.6
        },
        OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
            logger.Info("circuit breaker state changed",
                "from", from.String(),
                "to", to.String())
            
            metrics.RecordBreakerState(to.String())
            
            if to == gobreaker.StateOpen {
                metrics.RecordBreakerTripped()
            }
        },
    })
}

