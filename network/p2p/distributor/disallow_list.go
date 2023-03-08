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
	"github.com/onflow/flow-go/network/p2p"
)

const (
	// disallowListDistributorWorkerCount is the number of workers used to process disallow list updates.
	disallowListDistributorWorkerCount = 1

	// DefaultDisallowListNotificationQueueCacheSize is the default size of the disallow list notification queue.
	DefaultDisallowListNotificationQueueCacheSize = 100
)

// DisallowListUpdateNotificationDistributor is a component that distributes disallow list updates to registered consumers in a
// non-blocking manner.
type DisallowListUpdateNotificationDistributor struct {
	component.Component
	cm     *component.ComponentManager
	logger zerolog.Logger

	consumerLock sync.RWMutex // protects the consumer field from concurrent updates
	consumers    []p2p.DisallowListConsumer
	workerPool   *worker.Pool[DisallowListUpdateNotification]
}

// DisallowListUpdateNotificationConsumer is an adapter that allows the DisallowListUpdateNotificationDistributor to be used as an
// EventConsumer.
type DisallowListUpdateNotificationConsumer struct {
	consumer p2p.DisallowListConsumer
}

func (d *DisallowListUpdateNotificationConsumer) ConsumeEvent(msg DisallowListUpdateNotification) {
	d.consumer.OnNodeDisallowListUpdate(msg.DisallowList)
}

var _ p2p.DisallowListConsumer = (*DisallowListUpdateNotificationDistributor)(nil)

func DefaultDisallowListNotificationConsumer(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *DisallowListUpdateNotificationDistributor {
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

func NewDisallowListConsumer(logger zerolog.Logger, store engine.MessageStore) *DisallowListUpdateNotificationDistributor {
	lg := logger.With().Str("component", "node_disallow_distributor").Logger()

	d := &DisallowListUpdateNotificationDistributor{
		logger: lg,
	}

	pool := worker.NewWorkerPoolBuilder[DisallowListUpdateNotification](
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

// distribute is called by the workers to process the event. It calls the OnNodeDisallowListUpdate method on all registered
// consumers.
// It does not return an error because the event is already in the store, so it will be retried.
func (d *DisallowListUpdateNotificationDistributor) distribute(msg DisallowListUpdateNotification) error {
	d.consumerLock.RLock()
	defer d.consumerLock.RUnlock()

	for _, consumer := range d.consumers {
		consumer.OnNodeDisallowListUpdate(msg.DisallowList)
	}

	return nil
}

// DisallowListUpdateNotification is the event that is submitted to the distributor when the disallow list is updated.
type DisallowListUpdateNotification struct {
	DisallowList flow.IdentifierList
}

// AddConsumer registers a consumer with the distributor. The distributor will call the consumer's OnNodeDisallowListUpdate
// method when the node disallow list is updated.
func (d *DisallowListUpdateNotificationDistributor) AddConsumer(consumer p2p.DisallowListConsumer) {
	d.consumerLock.Lock()
	defer d.consumerLock.Unlock()

	d.consumers = append(d.consumers, consumer)
}

// OnNodeDisallowListUpdate is called when the node disallow list is updated. It submits the event to the distributor to be
// processed asynchronously.
func (d *DisallowListUpdateNotificationDistributor) OnNodeDisallowListUpdate(disallowList flow.IdentifierList) {
	ok := d.workerPool.Submit(DisallowListUpdateNotification{
		DisallowList: disallowList,
	})

	if !ok {
		d.logger.Fatal().Msg("failed to submit disallow list update event to distributor")
	}
}
