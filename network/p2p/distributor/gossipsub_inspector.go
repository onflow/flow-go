package distributor

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/handler"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
)

const (
	// DefaultGossipSubInspectorNotificationQueueCacheSize is the default cache size for the gossipsub rpc inspector notification queue.
	DefaultGossipSubInspectorNotificationQueueCacheSize = 10_000
	// defaultGossipSubInspectorNotificationQueueWorkerCount is the default number of workers that will process the gossipsub rpc inspector notifications.
	defaultGossipSubInspectorNotificationQueueWorkerCount = 1
)

// GossipSubInspectorNotification is a component that distributes gossipsub rpc inspector notifications to
// registered consumers.
type GossipSubInspectorNotification struct {
	component.Component
	cm     *component.ComponentManager
	logger zerolog.Logger

	// handler is the async event handler that will process the notifications asynchronously.
	handler *handler.AsyncEventDistributor[p2p.InvalidControlMessageNotification]
}

// GossipSubInspectorNotificationProcessor is an adapter that allows the GossipSubInspectorNotification to be used as an
// EventConsumer.
type GossipSubInspectorNotificationProcessor struct {
	consumer p2p.GossipSubRpcInspectorConsumer
}

// ProcessEvent processes the gossipsub rpc inspector notification. It will be called by the async event handler.
// It will call the consumer's OnInvalidControlMessage method.
func (g *GossipSubInspectorNotificationProcessor) ConsumeEvent(msg p2p.InvalidControlMessageNotification) {
	g.consumer.OnInvalidControlMessage(msg)
}

var _ p2p.GossipSubRpcInspectorConsumer = (*GossipSubInspectorNotification)(nil)

// DefaultGossipSubInspectorNotification returns a new GossipSubInspectorNotification component with the default configuration.
func DefaultGossipSubInspectorNotification(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *GossipSubInspectorNotification {
	cfg := &queue.HeroStoreConfig{
		SizeLimit: DefaultGossipSubInspectorNotificationQueueCacheSize,
		Collector: metrics.NewNoopCollector(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	store := queue.NewHeroStore(cfg.SizeLimit, logger, cfg.Collector)
	return NewGossipSubInspectorNotification(logger, store)
}

// NewGossipSubInspectorNotification returns a new GossipSubInspectorNotification component.
// It takes a message store to store the notifications in memory and process them asynchronously.
func NewGossipSubInspectorNotification(log zerolog.Logger, store engine.MessageStore) *GossipSubInspectorNotification {
	lg := log.With().Str("component", "gossipsub_rpc_inspector_distributor").Logger()

	h := handler.NewAsyncEventDistributor[p2p.InvalidControlMessageNotification](
		log,
		store,
		defaultGossipSubInspectorNotificationQueueWorkerCount)

	g := &GossipSubInspectorNotification{
		logger:  lg,
		handler: h,
	}

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()

		h.Start(ctx)
		<-h.Ready()
		g.logger.Info().Msg("node disallow list notification distributor started")

		<-ctx.Done()
		g.logger.Debug().Msg("node disallow list distributor shutting down")
	})

	g.cm = cm.Build()
	g.Component = g.cm

	return g
}

// OnInvalidControlMessage is called when a control message is received that is invalid according to the
// Flow protocol specifications.
// Prerequisites:
// Implementation must be concurrency safe and non-blocking.
func (g *GossipSubInspectorNotification) OnInvalidControlMessage(notification p2p.InvalidControlMessageNotification) {
	// we submit notification event to the handler to be processed by the worker.
	// the origin id is set to flow.ZeroID because the notification update event is not associated with a specific node.
	// the distributor discards the origin id upon processing it.
	err := g.handler.Submit(notification)
	if err != nil {
		g.logger.Fatal().Err(err).Msg("failed to submit invalid control message event to handler")
	}
}

// AddConsumer adds a consumer to the GossipSubInspectorNotification. The consumer will be called when a new
// notification is received. The consumer must be concurrency safe.
func (g *GossipSubInspectorNotification) AddConsumer(consumer p2p.GossipSubRpcInspectorConsumer) {
	g.handler.RegisterConsumer(&GossipSubInspectorNotificationProcessor{consumer: consumer})
}
