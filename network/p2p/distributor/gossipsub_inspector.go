package distributor

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/module/component"
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

	workerPool   *worker.Pool[p2p.InvalidControlMessageNotification]
	consumerLock sync.RWMutex // protects the consumer field from concurrent updates
	consumers    []p2p.GossipSubRpcInspectorConsumer
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

	d := &GossipSubInspectorNotificationDistributor{
		logger: lg,
	}

	pool := worker.NewWorkerPoolBuilder[p2p.InvalidControlMessageNotification](lg, store, d.Distribute).Build()
	d.workerPool = pool

	cm := component.NewComponentManagerBuilder()

	for i := 0; i < defaultGossipSubInspectorNotificationQueueWorkerCount; i++ {
		cm.AddWorker(pool.WorkerLogic())
	}

	d.cm = cm.Build()
	d.Component = d.cm

	return d
}

// Distribute distributes the gossipsub rpc inspector notification to all registered consumers.
// It will be called by the workers of the worker pool.
// It will call the consumer's OnInvalidControlMessage method.
// It does not return an error.
func (g *GossipSubInspectorNotificationDistributor) Distribute(msg p2p.InvalidControlMessageNotification) error {
	g.consumerLock.RLock()
	defer g.consumerLock.RUnlock()

	for _, consumer := range g.consumers {
		consumer.OnInvalidControlMessage(msg)
	}

	return nil
}

// OnInvalidControlMessage is called when a control message is received that is invalid according to the
// Flow protocol specifications.
// Prerequisites:
// Implementation must be concurrency safe and non-blocking.
func (g *GossipSubInspectorNotificationDistributor) OnInvalidControlMessage(notification p2p.InvalidControlMessageNotification) {
	ok := g.workerPool.Submit(notification)
	if !ok {
		g.logger.Fatal().Msg("failed to submit invalid control message event to distributor")
	}
}

// AddConsumer adds a consumer to the GossipSubInspectorNotificationDistributor. The consumer will be called when a new
// notification is received. The consumer must be concurrency safe.
func (g *GossipSubInspectorNotificationDistributor) AddConsumer(consumer p2p.GossipSubRpcInspectorConsumer) {
	g.consumerLock.Lock()
	defer g.consumerLock.Unlock()

	g.consumers = append(g.consumers, consumer)
}
