package bridge

import (
    "context"
    "sync"
    "time"

    "bridger/internal/logger"
    "bridger/internal/metrics"
    "github.com/sony/gobreaker"
)

type WorkerPool struct {
    config     WorkerConfig
    msgQueue   chan Message
    batches    chan Batch
    metrics    *metrics.Metrics
    logger     *logger.Logger
    breaker    *gobreaker.CircuitBreaker
    publisher  func(string, []byte) error
    wg         sync.WaitGroup
    ctx        context.Context
    cancel     context.CancelFunc
}

func NewWorkerPool(cfg WorkerConfig, metrics *metrics.Metrics, logger *logger.Logger, breaker *gobreaker.CircuitBreaker, publisher func(string, []byte) error) *WorkerPool {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &WorkerPool{
        config:    cfg,
        msgQueue:  make(chan Message, cfg.QueueSize),
        batches:   make(chan Batch, cfg.NumWorkers),
        metrics:   metrics,
        logger:    logger,
        breaker:   breaker,
        publisher: publisher,
        ctx:       ctx,
        cancel:    cancel,
    }
}

func (wp *WorkerPool) Start() {
    // Start batch collector
    wp.wg.Add(1)
    go wp.batchCollector()

    // Start workers
    for i := 0; i < wp.config.NumWorkers; i++ {
        wp.wg.Add(1)
        go wp.worker(i)
    }

    wp.logger.Info("worker pool started",
        "workers", wp.config.NumWorkers,
        "batchSize", wp.config.BatchSize,
        "queueSize", wp.config.QueueSize)
}

func (wp *WorkerPool) Stop() {
    wp.cancel()
    wp.wg.Wait()
    wp.logger.Info("worker pool stopped")
}

func (wp *WorkerPool) Submit(msg Message) bool {
    select {
    case wp.msgQueue <- msg:
        return true
    default:
        wp.metrics.RecordMessageDropped()
        return false
    }
}

func (wp *WorkerPool) batchCollector() {
    defer wp.wg.Done()

    batches := make(map[string]*Batch)
    timer := time.NewTicker(wp.config.BatchTimeout)
    defer timer.Stop()

    for {
        select {
        case <-wp.ctx.Done():
            wp.flushBatches(batches)
            return

        case msg := <-wp.msgQueue:
            destTopic := msg.Mapper.Transform(msg.Topic)
            
            if batch, exists := batches[destTopic]; exists {
                batch.Messages = append(batch.Messages, msg)
                batch.Size++
                
                if batch.Size >= wp.config.BatchSize {
                    wp.sendBatch(batch)
                    delete(batches, destTopic)
                }
            } else {
                batches[destTopic] = &Batch{
                    Messages: []Message{msg},
                    Size:    1,
                    Topic:   destTopic,
                }
            }

        case <-timer.C:
            wp.flushBatches(batches)
        }
    }
}

func (wp *WorkerPool) flushBatches(batches map[string]*Batch) {
    for topic, batch := range batches {
        if batch.Size > 0 {
            wp.sendBatch(batch)
            delete(batches, topic)
        }
    }
}

func (wp *WorkerPool) sendBatch(batch *Batch) {
    select {
    case wp.batches <- *batch:
        wp.metrics.RecordBatchProcessed(batch.Size, 0)
    default:
        wp.metrics.RecordBatchDropped(batch.Size)
        wp.logger.Warn("batch queue full, dropping batch",
            "topic", batch.Topic,
            "size", batch.Size)
    }
}

func (wp *WorkerPool) worker(id int) {
    defer wp.wg.Done()
    logger := wp.logger.With("worker", id)

    for {
        select {
        case <-wp.ctx.Done():
            return

        case batch := <-wp.batches:
            start := time.Now()
            
            _, err := wp.breaker.Execute(func() (interface{}, error) {
                // Combine payloads
                combined := wp.combineBatchPayloads(batch)
                return nil, wp.publisher(batch.Topic, combined)
            })

            if err != nil {
                logger.Error("failed to publish batch",
                    "topic", batch.Topic,
                    "size", batch.Size,
                    "error", err)
                wp.metrics.RecordError()
                continue
            }

            wp.metrics.RecordBatchProcessed(batch.Size, time.Since(start))
            logger.Debug("batch processed",
                "topic", batch.Topic,
                "size", batch.Size,
                "duration", time.Since(start))
        }
    }
}

func (wp *WorkerPool) combineBatchPayloads(batch Batch) []byte {
    totalSize := 0
    for _, msg := range batch.Messages {
        totalSize += len(msg.Payload)
    }

    combined := make([]byte, 0, totalSize)
    for _, msg := range batch.Messages {
        combined = append(combined, msg.Payload...)
    }
    
    return combined
}
