package distributor

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
)

const (
	// DefaultDisallowListNotificationQueueCacheSize is the default size of the disallow list notification queue.
	DefaultDisallowListNotificationQueueCacheSize = 100
)

// DisallowListNotificationDistributor is a component that distributes disallow list updates to registered consumers in an
// asynchronous, fan-out manner. It is thread-safe and can be used concurrently from multiple goroutines.
type DisallowListNotificationDistributor struct {
	component.Component
	cm     *component.ComponentManager
	logger zerolog.Logger

	consumerLock sync.RWMutex // protects the consumer field from concurrent updates
	consumers    []network.DisallowListNotificationConsumer
	workerPool   *worker.Pool[*p2p.RemoteNodesAllowListingUpdate]
}

var _ p2p.DisallowListNotificationDistributor = (*DisallowListNotificationDistributor)(nil)

// DefaultDisallowListNotificationDistributor creates a new disallow list notification distributor with default configuration.
func DefaultDisallowListNotificationDistributor(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *DisallowListNotificationDistributor {
	cfg := &queue.HeroStoreConfig{
		SizeLimit: DefaultDisallowListNotificationQueueCacheSize,
		Collector: metrics.NewNoopCollector(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	store := queue.NewHeroStore(cfg.SizeLimit, logger, cfg.Collector)
	return NewDisallowListConsumer(logger, store)
}

// NewDisallowListConsumer creates a new disallow list notification distributor.
// It takes a message store as a parameter, which is used to store the events that are distributed to the consumers.
// The message store is used to ensure that DistributeBlockListNotification is non-blocking.
func NewDisallowListConsumer(logger zerolog.Logger, store engine.MessageStore) *DisallowListNotificationDistributor {
	lg := logger.With().Str("component", "node_disallow_distributor").Logger()

	d := &DisallowListNotificationDistributor{
		logger: lg,
	}

	pool := worker.NewWorkerPoolBuilder[*p2p.RemoteNodesAllowListingUpdate](
		lg,
		store,
		d.distribute).Build()

	d.workerPool = pool

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(d.workerPool.WorkerLogic())

	d.cm = cm.Build()
	d.Component = d.cm

	return d
}

// distribute is called by the workers to process the event. It calls the OnDisallowListNotification method on all registered
// consumers.
// It does not return an error because the event is already in the store, so it will be retried.
func (d *DisallowListNotificationDistributor) distribute(notification *p2p.RemoteNodesAllowListingUpdate) error {
	d.consumerLock.RLock()
	defer d.consumerLock.RUnlock()

	for _, consumer := range d.consumers {
		consumer.OnDisallowListNotification(notification)
	}

	return nil
}

// AddConsumer adds a consumer to the distributor. The consumer will be called the distributor distributes a new event.
// AddConsumer must be concurrency safe. Once a consumer is added, it must be called for all future events.
// There is no guarantee that the consumer will be called for events that were already received by the distributor.
func (d *DisallowListNotificationDistributor) AddConsumer(consumer network.DisallowListNotificationConsumer) {
	d.consumerLock.Lock()
	defer d.consumerLock.Unlock()

	d.consumers = append(d.consumers, consumer)
}

// DistributeBlockListNotification distributes the event to all the consumers.
// Implementation is non-blocking, it submits the event to the worker pool and returns immediately.
// The event will be distributed to the consumers in the order it was submitted but asynchronously.
// If the worker pool is full, the event will be dropped and a warning will be logged.
// This implementation returns no error.
func (d *DisallowListNotificationDistributor) DistributeBlockListNotification(disallowList flow.IdentifierList) error {
	ok := d.workerPool.Submit(&p2p.RemoteNodesAllowListingUpdate{FlowIds: disallowList})
	if !ok {
		// we use a queue to buffer the events, so this may happen if the queue is full or the event is duplicate. In this case, we log a warning.
		d.logger.Warn().Msg("node disallow list update notification queue is full or the event is duplicate, dropping event")
	}

	return nil
}
