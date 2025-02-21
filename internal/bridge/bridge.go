package bridge

import (
    "context"
    "fmt"
    "sync"
    "time"

    "bridger/internal/config"
    "bridger/internal/logger"
    "bridger/internal/metrics"
    mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Bridge handles message forwarding between source and destination brokers
type Bridge struct {
    cfg           *config.Config
    logger        *logger.Logger
    metrics       *metrics.Metrics
    sourceConn    mqtt.Client
    destConn      mqtt.Client
    topicMapper   []*TopicMatcher
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

    b.logger.Info("bridge started successfully",
        "mappings", len(b.topicMapper),
        "version", b.buildVersion,
        "commit", b.buildCommit)

    return nil
}

// collectSystemMetrics periodically updates system metrics
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

// handleSourceConnected handles successful source broker connections
func (b *Bridge) handleSourceConnected(_ mqtt.Client) {
    b.logger.Info("connected to source broker", "broker", b.cfg.Source.Broker)
    
    b.isConnected.Lock()
    b.isConnected.source = true
    b.isConnected.Unlock()

    b.metrics.SetConnectionStatus(true, true)

    // Subscribe to all topics
    if err := b.subscribeToTopics(); err != nil {
        b.logger.Error("failed to subscribe to topics after connection",
            "error", err)
        b.metrics.RecordError()
        return
    }
}

// handleDestConnected handles successful destination broker connections
func (b *Bridge) handleDestConnected(_ mqtt.Client) {
    b.logger.Info("connected to destination broker", "broker", b.cfg.Destination.Broker)
    
    b.isConnected.Lock()
    b.isConnected.dest = true
    b.isConnected.Unlock()

    b.metrics.SetConnectionStatus(false, true)
}

// handleSourceConnectionLost handles source broker connection loss
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

// handleDestConnectionLost handles destination broker connection loss
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

// subscribeToTopics subscribes to all configured source topics
func (b *Bridge) subscribeToTopics() error {
    for _, mapper := range b.topicMapper {
        // Create closure to capture mapper
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

// handleMessage processes and forwards messages
func (b *Bridge) handleMessage(sourceTopic string, payload []byte, mapper *TopicMatcher) {
    start := time.Now()
    payloadSize := len(payload)

    // Check destination connection before attempting to publish
    b.isConnected.RLock()
    destConnected := b.isConnected.dest
    b.isConnected.RUnlock()

    if !destConnected {
        b.logger.Error("cannot forward message - destination broker not connected",
            "sourceTopic", sourceTopic)
        b.metrics.RecordError()
        return
    }

    // Generate destination topic
    destTopic := mapper.Transform(sourceTopic)

    b.logger.Debug("forwarding message",
        "sourceTopic", sourceTopic,
        "destTopic", destTopic,
        "payloadSize", payloadSize)

    // Publish to destination broker
    if token := b.destConn.Publish(destTopic, 0, false, payload); token.Wait() && token.Error() != nil {
        b.logger.Error("failed to forward message",
            "error", token.Error(),
            "sourceTopic", sourceTopic,
            "destTopic", destTopic)
        b.metrics.RecordError()
        return
    }

    // Record metrics
    processingTime := time.Since(start)
    b.metrics.RecordMessage(payloadSize, payloadSize, processingTime)

    b.logger.Debug("message forwarded successfully",
        "sourceTopic", sourceTopic,
        "destTopic", destTopic,
        "processingTime", processingTime)
}

// IsConnected returns the connection status of both brokers
func (b *Bridge) IsConnected() (source, dest bool) {
    b.isConnected.RLock()
    defer b.isConnected.RUnlock()
    return b.isConnected.source, b.isConnected.dest
}

// Stop gracefully shuts down the bridge
func (b *Bridge) Stop() {
    b.logger.Info("stopping bridge")
    b.cancel()

    // Disconnect from brokers with a timeout
    b.sourceConn.Disconnect(250)
    b.destConn.Disconnect(250)

    b.logger.Info("bridge stopped")
}
