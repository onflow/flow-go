package distributor

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/distributor"
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

// GossipSubInspectorNotificationDistributor is a component that distributes gossipsub rpc inspector notifications to
// registered consumers in a non-blocking manner.
type GossipSubInspectorNotificationDistributor struct {
	component.Component
	cm     *component.ComponentManager
	logger zerolog.Logger

	// AsyncEventDistributor that will distribute the notifications asynchronously.
	asyncDistributor *distributor.AsyncEventDistributor[p2p.InvalidControlMessageNotification]
}

// GossipSubInspectorNotificationConsumer is an adapter that allows the GossipSubInspectorNotificationDistributor to be used as an
// EventConsumer.
type GossipSubInspectorNotificationConsumer struct {
	consumer p2p.GossipSubRpcInspectorConsumer
}

// ConsumeEvent processes the gossipsub rpc inspector notification. It will be called by the AsyncEventDistributor.
// It will call the consumer's OnInvalidControlMessage method.
func (g *GossipSubInspectorNotificationConsumer) ConsumeEvent(msg p2p.InvalidControlMessageNotification) {
	g.consumer.OnInvalidControlMessage(msg)
}

var _ p2p.GossipSubRpcInspectorConsumer = (*GossipSubInspectorNotificationDistributor)(nil)

// DefaultGossipSubInspectorNotification returns a new GossipSubInspectorNotificationDistributor component with the default configuration.
func DefaultGossipSubInspectorNotification(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *GossipSubInspectorNotificationDistributor {
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

// NewGossipSubInspectorNotification returns a new GossipSubInspectorNotificationDistributor component.
// It takes a message store to store the notifications in memory and process them asynchronously.
func NewGossipSubInspectorNotification(log zerolog.Logger, store engine.MessageStore) *GossipSubInspectorNotificationDistributor {
	lg := log.With().Str("component", "gossipsub_rpc_inspector_distributor").Logger()

	h := distributor.NewAsyncEventDistributor[p2p.InvalidControlMessageNotification](
		log,
		store,
		defaultGossipSubInspectorNotificationQueueWorkerCount)

	g := &GossipSubInspectorNotificationDistributor{
		logger:           lg,
		asyncDistributor: h,
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
func (g *GossipSubInspectorNotificationDistributor) OnInvalidControlMessage(notification p2p.InvalidControlMessageNotification) {
	err := g.asyncDistributor.Submit(notification)
	if err != nil {
		g.logger.Fatal().Err(err).Msg("failed to submit invalid control message event to distributor")
	}
}

// AddConsumer adds a consumer to the GossipSubInspectorNotificationDistributor. The consumer will be called when a new
// notification is received. The consumer must be concurrency safe.
func (g *GossipSubInspectorNotificationDistributor) AddConsumer(consumer p2p.GossipSubRpcInspectorConsumer) {
	g.asyncDistributor.RegisterConsumer(&GossipSubInspectorNotificationConsumer{consumer: consumer})
}
