package distributor

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/distributor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
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

	// AsyncEventDistributor that will distribute the notifications asynchronously.
	asyncDistributor *distributor.AsyncEventDistributor[DisallowListUpdateNotification]
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

	h := distributor.NewAsyncEventDistributor[DisallowListUpdateNotification](
		lg,
		store,
		disallowListDistributorWorkerCount)

	d := &DisallowListUpdateNotificationDistributor{
		logger:           lg,
		asyncDistributor: h,
	}

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()

		h.Start(ctx)
		<-h.Ready()
		d.logger.Info().Msg("node disallow list distributor started")

		<-ctx.Done()
		d.logger.Debug().Msg("node disallow list distributor shutting down")
	})

	d.cm = cm.Build()
	d.Component = d.cm

	return d
}

// DisallowListUpdateNotification is the event that is submitted to the distributor when the disallow list is updated.
type DisallowListUpdateNotification struct {
	DisallowList flow.IdentifierList
}

// AddConsumer registers a consumer with the distributor. The distributor will call the consumer's OnNodeDisallowListUpdate
// method when the node disallow list is updated.
func (d *DisallowListUpdateNotificationDistributor) AddConsumer(consumer p2p.DisallowListConsumer) {
	d.asyncDistributor.RegisterConsumer(&DisallowListUpdateNotificationConsumer{consumer: consumer})
}

// OnNodeDisallowListUpdate is called when the node disallow list is updated. It submits the event to the distributor to be
// processed asynchronously.
func (d *DisallowListUpdateNotificationDistributor) OnNodeDisallowListUpdate(disallowList flow.IdentifierList) {
	err := d.asyncDistributor.Submit(DisallowListUpdateNotification{
		DisallowList: disallowList,
	})

	if err != nil {
		d.logger.Fatal().Err(err).Msg("failed to submit disallow list update event to distributor")
	}
}
