package bridge

import (
    "context"
    "fmt"
    "sync"
    "time"
    "runtime"

    "bridger/internal/config"
    "bridger/internal/logger"
    "bridger/internal/metrics"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "github.com/sony/gobreaker"
)

// Bridge handles message forwarding between source and destination brokers
type Bridge struct {
    cfg           *config.Config
    logger        *logger.Logger
    metrics       *metrics.Metrics
    sourceConn    mqtt.Client
    destConn      mqtt.Client
    topicMapper   []*TopicMatcher
    workerPool    *WorkerPool
    breaker       *gobreaker.CircuitBreaker
    isConnected   struct {
        source bool
        dest   bool
        sync.RWMutex
    }
    buildVersion string
    buildCommit  string
    buildTime    string
    ctx          context.Context
    cancel       context.CancelFunc
}

// NewBridge creates a new MQTT bridge instance
func NewBridge(cfg *config.Config, log *logger.Logger, metrics *metrics.Metrics, version, commit, buildTime string) (*Bridge, error) {
    ctx, cancel := context.WithCancel(context.Background())

    bridge := &Bridge{
        cfg:          cfg,
        logger:       log,
        metrics:      metrics,
        topicMapper:  make([]*TopicMatcher, 0),
        buildVersion: version,
        buildCommit:  commit,
        buildTime:    buildTime,
        ctx:          ctx,
        cancel:       cancel,
    }

    // Initialize topic mappers
    for _, tm := range cfg.TopicMap {
        mapper := NewTopicMatcher(tm.Source, tm.Destination)
        bridge.topicMapper = append(bridge.topicMapper, mapper)
    }

    // Create worker config
    workerCfg := NewWorkerConfigFromConfig(
        cfg.Performance.WorkerPool.NumWorkers,
        cfg.Performance.WorkerPool.BatchSize,
        cfg.Performance.WorkerPool.BatchTimeoutMs,
        cfg.Performance.WorkerPool.QueueSize,
    )

    // If NumWorkers not specified, use CPU count
    if workerCfg.NumWorkers <= 0 {
        workerCfg.NumWorkers = runtime.NumCPU()
    }

    // Initialize circuit breaker
    breakerCfg := NewBreakerConfig(
        "mqtt-publisher",
        cfg.Performance.CircuitBreaker.MaxFailures,
        cfg.Performance.CircuitBreaker.TimeoutSeconds,
        cfg.Performance.CircuitBreaker.MaxRequests,
        cfg.Performance.CircuitBreaker.IntervalSeconds,
    )
    
    bridge.breaker = NewCircuitBreaker(breakerCfg, log, metrics)

    // Create publisher function for worker pool
    publisher := func(topic string, payload []byte) error {
        token := bridge.destConn.Publish(topic, 0, false, payload)
        token.Wait()
        return token.Error()
    }

    // Initialize worker pool
    bridge.workerPool = NewWorkerPool(
        workerCfg,
        metrics,
        log.With("component", "worker_pool"),
        bridge.breaker,
        publisher,
    )

    // Initialize source connection
    sourceOpts := mqtt.NewClientOptions().
        AddBroker(cfg.Source.Broker).
        SetClientID(cfg.Source.ClientID).
        SetUsername(cfg.Source.Username).
        SetPassword(cfg.Source.Password).
        SetCleanSession(true).
        SetAutoReconnect(true).
        SetMaxReconnectInterval(time.Second * 10).
        SetConnectionLostHandler(bridge.handleSourceConnectionLost).
        SetOnConnectHandler(bridge.handleSourceConnected).
        SetKeepAlive(30 * time.Second)

    // Initialize destination connection
    destOpts := mqtt.NewClientOptions().
        AddBroker(cfg.Destination.Broker).
        SetClientID(cfg.Destination.ClientID).
        SetUsername(cfg.Destination.Username).
        SetPassword(cfg.Destination.Password).
        SetCleanSession(true).
        SetAutoReconnect(true).
        SetMaxReconnectInterval(time.Second * 10).
        SetConnectionLostHandler(bridge.handleDestConnectionLost).
        SetOnConnectHandler(bridge.handleDestConnected).
        SetKeepAlive(30 * time.Second)

    // Configure TLS if enabled for source
    if cfg.Source.TLS.Enable {
        tlsConfig, err := createTLSConfig(cfg.Source.TLS)
        if err != nil {
            return nil, fmt.Errorf("failed to configure source TLS: %w", err)
        }
        sourceOpts.SetTLSConfig(tlsConfig)
    }

    // Configure TLS if enabled for destination
    if cfg.Destination.TLS.Enable {
        tlsConfig, err := createTLSConfig(cfg.Destination.TLS)
        if err != nil {
            return nil, fmt.Errorf("failed to configure destination TLS: %w", err)
        }
        destOpts.SetTLSConfig(tlsConfig)
    }

    // Create MQTT clients
    bridge.sourceConn = mqtt.NewClient(sourceOpts)
    bridge.destConn = mqtt.NewClient(destOpts)

    // Set build info metric
    metrics.SetBuildInfo(version, commit, buildTime)

    // Start system metrics collection
    go bridge.collectSystemMetrics()

    return bridge, nil
}

// Start begins bridging messages between brokers
func (b *Bridge) Start() error {
    // Connect to source broker
    if token := b.sourceConn.Connect(); token.Wait() && token.Error() != nil {
        return fmt.Errorf("failed to connect to source broker: %w", token.Error())
    }

    // Connect to destination broker
    if token := b.destConn.Connect(); token.Wait() && token.Error() != nil {
        return fmt.Errorf("failed to connect to destination broker: %w", token.Error())
    }

    // Start worker pool
    b.workerPool.Start()

    b.logger.Info("bridge started successfully",
        "mappings", len(b.topicMapper),
        "workers", b.workerPool.config.NumWorkers,
        "batchSize", b.workerPool.config.BatchSize,
        "batchTimeout", b.workerPool.config.BatchTimeout,
        "queueSize", b.workerPool.config.QueueSize,
        "version", b.buildVersion,
        "commit", b.buildCommit)

    return nil
}

func (b *Bridge) collectSystemMetrics() {
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-b.ctx.Done():
            return
        case <-ticker.C:
            b.metrics.UpdateSystemMetrics()
        }
    }
}

func (b *Bridge) handleSourceConnected(_ mqtt.Client) {
    b.logger.Info("connected to source broker", "broker", b.cfg.Source.Broker)
    
    b.isConnected.Lock()
    b.isConnected.source = true
    b.isConnected.Unlock()

    b.metrics.SetConnectionStatus(true, true)

    if err := b.subscribeToTopics(); err != nil {
        b.logger.Error("failed to subscribe to topics after connection",
            "error", err)
        b.metrics.RecordError()
        return
    }
}

func (b *Bridge) handleDestConnected(_ mqtt.Client) {
    b.logger.Info("connected to destination broker", "broker", b.cfg.Destination.Broker)
    
    b.isConnected.Lock()
    b.isConnected.dest = true
    b.isConnected.Unlock()

    b.metrics.SetConnectionStatus(false, true)
}

func (b *Bridge) handleSourceConnectionLost(_ mqtt.Client, err error) {
    b.logger.Error("lost connection to source broker",
        "error", err,
        "broker", b.cfg.Source.Broker)

    b.isConnected.Lock()
    b.isConnected.source = false
    b.isConnected.Unlock()

    b.metrics.SetConnectionStatus(true, false)
    b.metrics.RecordReconnect(true)
}

func (b *Bridge) handleDestConnectionLost(_ mqtt.Client, err error) {
    b.logger.Error("lost connection to destination broker",
        "error", err,
        "broker", b.cfg.Destination.Broker)

    b.isConnected.Lock()
    b.isConnected.dest = false
    b.isConnected.Unlock()

    b.metrics.SetConnectionStatus(false, false)
    b.metrics.RecordReconnect(false)
}

func (b *Bridge) subscribeToTopics() error {
    for _, mapper := range b.topicMapper {
        handler := func(mapper *TopicMatcher) mqtt.MessageHandler {
            return func(client mqtt.Client, msg mqtt.Message) {
                b.handleMessage(msg.Topic(), msg.Payload(), mapper)
            }
        }(mapper)

        if token := b.sourceConn.Subscribe(mapper.SourcePattern, 0, handler); token.Wait() && token.Error() != nil {
            b.logger.Error("failed to subscribe to topic",
                "topic", mapper.SourcePattern,
                "error", token.Error())
            return fmt.Errorf("failed to subscribe to topic %s: %w", mapper.SourcePattern, token.Error())
        }

        b.logger.Info("subscribed to topic",
            "sourceTopic", mapper.SourcePattern,
            "destPattern", mapper.DestinationPattern)
    }

    return nil
}

func (b *Bridge) handleMessage(sourceTopic string, payload []byte, mapper *TopicMatcher) {
    msg := Message{
        Topic:     sourceTopic,
        Payload:   payload,
        Mapper:    mapper,
        Timestamp: time.Now(),
    }

    if !b.workerPool.Submit(msg) {
        b.logger.Error("failed to submit message - queue full",
            "topic", sourceTopic)
        b.metrics.RecordMessageDropped()
    }
}

func (b *Bridge) IsConnected() (source, dest bool) {
    b.isConnected.RLock()
    defer b.isConnected.RUnlock()
    return b.isConnected.source, b.isConnected.dest
}

func (b *Bridge) Stop() {
    b.logger.Info("stopping bridge")
    
    // Stop worker pool first to process remaining messages
    b.workerPool.Stop()

    // Cancel context for metric collection
    b.cancel()

    // Disconnect from brokers with a timeout
    b.sourceConn.Disconnect(250)
    b.destConn.Disconnect(250)

    b.logger.Info("bridge stopped")
}
