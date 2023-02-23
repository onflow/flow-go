package distributor

import (
	"sync"

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

	consumers []p2p.DisallowListConsumer
	handler   *handler.AsyncEventHandler
	logger    zerolog.Logger
	lock      sync.RWMutex
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
	h := handler.NewAsyncEventHandler(logger, store, disallowListDistributorWorkerCount)

	d := &DisallowListNotificationConsumer{
		consumers: make([]p2p.DisallowListConsumer, 0),
		handler:   h,
		logger:    logger.With().Str("component", "node_disallow_distributor").Logger(),
	}

	h.RegisterProcessor(d.ProcessQueuedNotifications)

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

func (d *DisallowListNotificationConsumer) AddConsumer(consumer p2p.DisallowListConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *DisallowListNotificationConsumer) OnNodeDisallowListUpdate(disallowList flow.IdentifierList) {
	// we submit the disallow list update event to the handler to be processed by the worker.
	// the origin id is set to flow.ZeroID because the disallow list update event is not associated with a specific node.
	// the distributor discards the origin id upon processing it.
	err := d.handler.Submit(DisallowListUpdateNotification{
		DisallowList: disallowList,
	})

	if err != nil {
		d.logger.Fatal().Err(err).Msg("failed to submit disallow list update event to handler")
	}
}

func (d *DisallowListNotificationConsumer) ProcessQueuedNotifications(_ flow.Identifier, notification interface{}) {
	var consumers []p2p.DisallowListConsumer
	d.lock.RLock()
	consumers = d.consumers
	d.lock.RUnlock()

	switch notification := notification.(type) {
	case DisallowListUpdateNotification:
		for _, consumer := range consumers {
			consumer.OnNodeDisallowListUpdate(notification.DisallowList)
		}
	default:
		d.logger.Fatal().Msgf("unknown notification type: %T", notification)
	}
}
