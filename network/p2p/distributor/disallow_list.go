package distributor

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/handler"
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

// DisallowListNotificationConsumer subscribes to changes in the NodeBlocklistWrapper block list.
type DisallowListNotificationConsumer struct {
	component.Component
	cm *component.ComponentManager

	handler *handler.AsyncEventDistributor[DisallowListUpdateNotification]
	logger  zerolog.Logger
}

// DisallowListUpdateNotificationProcessor is an adapter that allows the DisallowListNotificationConsumer to be used as an
// EventConsumer.
type DisallowListUpdateNotificationProcessor struct {
	consumer p2p.DisallowListConsumer
}

func (d *DisallowListUpdateNotificationProcessor) ConsumeEvent(msg DisallowListUpdateNotification) {
	d.consumer.OnNodeDisallowListUpdate(msg.DisallowList)
}

var _ p2p.DisallowListConsumer = (*DisallowListNotificationConsumer)(nil)

func DefaultDisallowListNotificationConsumer(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *DisallowListNotificationConsumer {
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

func NewDisallowListConsumer(logger zerolog.Logger, store engine.MessageStore) *DisallowListNotificationConsumer {
	lg := logger.With().Str("component", "node_disallow_distributor").Logger()

	h := handler.NewAsyncEventDistributor[DisallowListUpdateNotification](
		lg,
		store,
		disallowListDistributorWorkerCount)

	d := &DisallowListNotificationConsumer{
		logger:  lg,
		handler: h,
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

// DisallowListUpdateNotification is the event that is submitted to the handler when the disallow list is updated.
type DisallowListUpdateNotification struct {
	DisallowList flow.IdentifierList
}

// AddConsumer registers a consumer with the distributor. The distributor will call the consumer's OnNodeDisallowListUpdate
// method when the node disallow list is updated.
func (d *DisallowListNotificationConsumer) AddConsumer(consumer p2p.DisallowListConsumer) {
	d.handler.RegisterConsumer(&DisallowListUpdateNotificationProcessor{consumer: consumer})
}

// OnNodeDisallowListUpdate is called when the node disallow list is updated. It submits the event to the handler to be
// processed asynchronously.
func (d *DisallowListNotificationConsumer) OnNodeDisallowListUpdate(disallowList flow.IdentifierList) {
	err := d.handler.Submit(DisallowListUpdateNotification{
		DisallowList: disallowList,
	})

	if err != nil {
		d.logger.Fatal().Err(err).Msg("failed to submit disallow list update event to handler")
	}
}
