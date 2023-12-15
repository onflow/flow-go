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
	"github.com/onflow/flow-go/network/p2p/p2plogging"
)

const (
	// DefaultGossipSubInspectorNotificationQueueCacheSize is the default cache size for the gossipsub rpc inspector notification queue.
	DefaultGossipSubInspectorNotificationQueueCacheSize = 10_000
	// defaultGossipSubInspectorNotificationQueueWorkerCount is the default number of workers that will process the gossipsub rpc inspector notifications.
	defaultGossipSubInspectorNotificationQueueWorkerCount = 1
)

var _ p2p.GossipSubInspectorNotifDistributor = (*GossipSubInspectorNotifDistributor)(nil)

// GossipSubInspectorNotifDistributor is a component that distributes gossipsub rpc inspector notifications to
// registered consumers in a non-blocking manner and asynchronously. It is thread-safe and can be used concurrently from
// multiple goroutines. The distribution is done by a worker pool. The worker pool is configured with a queue that has a
// fixed size. If the queue is full, the notification is discarded. The queue size and the number of workers can be
// configured.
type GossipSubInspectorNotifDistributor struct {
	component.Component
	cm     *component.ComponentManager
	logger zerolog.Logger

	workerPool   *worker.Pool[*p2p.InvCtrlMsgNotif]
	consumerLock sync.RWMutex // protects the consumer field from concurrent updates
	consumers    []p2p.GossipSubInvCtrlMsgNotifConsumer
}

// DefaultGossipSubInspectorNotificationDistributor returns a new GossipSubInspectorNotifDistributor component with the default configuration.
func DefaultGossipSubInspectorNotificationDistributor(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *GossipSubInspectorNotifDistributor {
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

// NewGossipSubInspectorNotificationDistributor returns a new GossipSubInspectorNotifDistributor component.
// It takes a message store to store the notifications in memory and process them asynchronously.
func NewGossipSubInspectorNotificationDistributor(log zerolog.Logger, store engine.MessageStore) *GossipSubInspectorNotifDistributor {
	lg := log.With().Str("component", "gossipsub_rpc_inspector_distributor").Logger()

	d := &GossipSubInspectorNotifDistributor{
		logger: lg,
	}

	pool := worker.NewWorkerPoolBuilder[*p2p.InvCtrlMsgNotif](lg, store, d.distribute).Build()
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
// The distribution is done asynchronously and non-blocking. The notification is added to a queue and processed by a worker pool.
// DistributeEvent in this implementation does not return an error, but it logs a warning if the queue is full.
func (g *GossipSubInspectorNotifDistributor) Distribute(notification *p2p.InvCtrlMsgNotif) error {
	lg := g.logger.With().Str("peer_id", p2plogging.PeerId(notification.PeerID)).Logger()
	if ok := g.workerPool.Submit(notification); !ok {
		// we use a queue with a fixed size, so this can happen when queue is full or when the notification is duplicate.
		lg.Warn().Msg("gossipsub rpc inspector notification queue is full or notification is duplicate, discarding notification")
	}
	lg.Trace().Msg("gossipsub rpc inspector notification submitted to the queue")
	return nil
}

// AddConsumer adds a consumer to the distributor. The consumer will be called when distributor distributes a new event.
// AddConsumer must be concurrency safe. Once a consumer is added, it must be called for all future events.
// There is no guarantee that the consumer will be called for events that were already received by the distributor.
func (g *GossipSubInspectorNotifDistributor) AddConsumer(consumer p2p.GossipSubInvCtrlMsgNotifConsumer) {
	g.consumerLock.Lock()
	defer g.consumerLock.Unlock()

	g.consumers = append(g.consumers, consumer)
}

// distribute calls the ConsumeEvent method of all registered consumers. It is called by the workers of the worker pool.
// It is concurrency safe and can be called concurrently by multiple workers. However, the consumers may be blocking
// on the ConsumeEvent method.
func (g *GossipSubInspectorNotifDistributor) distribute(notification *p2p.InvCtrlMsgNotif) error {
	g.consumerLock.RLock()
	defer g.consumerLock.RUnlock()

	g.logger.Trace().Msg("distributing gossipsub rpc inspector notification")
	for _, consumer := range g.consumers {
		consumer.OnInvalidControlMessageNotification(notification)
	}
	g.logger.Trace().Msg("gossipsub rpc inspector notification distributed")

	return nil
}
