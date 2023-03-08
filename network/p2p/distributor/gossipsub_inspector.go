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
	consumers    []p2p.EventConsumer[p2p.InvalidControlMessageNotification]
}

// DistributeEvent distributes the gossipsub rpc inspector notification to all registered consumers.
// The distribution is done asynchronously and non-blocking. The notification is added to a queue and processed by a worker pool.
// DistributeEvent in this implementation does not return an error, but it logs a warning if the queue is full.
func (g *GossipSubInspectorNotificationDistributor) DistributeEvent(msg p2p.InvalidControlMessageNotification) error {
	if ok := g.workerPool.Submit(msg); !ok {
		// we use a queue with a fixed size, so this can happen when queue is full.
		g.logger.Warn().Msg("gossipsub rpc inspector notification queue is full, discarding notification")
	}
	return nil
}

// AddConsumer adds a consumer to the distributor. The consumer will be called when distributor receives a new event.
// AddConsumer must be concurrency safe. Once a consumer is added, it must be called for all future events.
// There is no guarantee that the consumer will be called for events that were already received by the distributor.
func (g *GossipSubInspectorNotificationDistributor) AddConsumer(consumer p2p.EventConsumer[p2p.InvalidControlMessageNotification]) {
	g.consumerLock.Lock()
	defer g.consumerLock.Unlock()

	g.consumers = append(g.consumers, consumer)
}

var _ p2p.EventDistributor[p2p.InvalidControlMessageNotification] = (*GossipSubInspectorNotificationDistributor)(nil)

// DefaultGossipSubInspectorNotificationDistributor returns a new GossipSubInspectorNotificationDistributor component with the default configuration.
func DefaultGossipSubInspectorNotificationDistributor(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *GossipSubInspectorNotificationDistributor {
	cfg := &queue.HeroStoreConfig{
		SizeLimit: DefaultGossipSubInspectorNotificationQueueCacheSize,
		Collector: metrics.NewNoopCollector(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	store := queue.NewHeroStore(cfg.SizeLimit, logger, cfg.Collector)
	return NewGossipSubInspectorNotificationDistributor(logger, store)
}

// NewGossipSubInspectorNotificationDistributor returns a new GossipSubInspectorNotificationDistributor component.
// It takes a message store to store the notifications in memory and process them asynchronously.
func NewGossipSubInspectorNotificationDistributor(log zerolog.Logger, store engine.MessageStore) *GossipSubInspectorNotificationDistributor {
	lg := log.With().Str("component", "gossipsub_rpc_inspector_distributor").Logger()

	d := &GossipSubInspectorNotificationDistributor{
		logger: lg,
	}

	pool := worker.NewWorkerPoolBuilder[p2p.InvalidControlMessageNotification](lg, store, d.distribute).Build()
	d.workerPool = pool

	cm := component.NewComponentManagerBuilder()

	for i := 0; i < defaultGossipSubInspectorNotificationQueueWorkerCount; i++ {
		cm.AddWorker(pool.WorkerLogic())
	}

	d.cm = cm.Build()
	d.Component = d.cm

	return d
}

// distribute calls the ConsumeEvent method of all registered consumers. It is called by the workers of the worker pool.
// It is concurrency safe and can be called concurrently by multiple workers. However, the consumers may be blocking
// on the ConsumeEvent method.
func (g *GossipSubInspectorNotificationDistributor) distribute(msg p2p.InvalidControlMessageNotification) error {
	g.consumerLock.RLock()
	defer g.consumerLock.RUnlock()

	g.logger.Trace().Msg("distributing gossipsub rpc inspector notification")
	for _, consumer := range g.consumers {
		consumer.ConsumeEvent(msg)
	}
	g.logger.Trace().Msg("gossipsub rpc inspector notification distributed")

	return nil
}
