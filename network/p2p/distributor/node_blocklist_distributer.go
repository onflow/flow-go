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

// blockListDistributorWorkerCount is the number of workers used to process block list updates.
const blockListDistributorWorkerCount = 1
const DefaultBlockListNotificationQueueCacheSize = 100

// DisallowListNotificationConsumer subscribes to changes in the NodeBlocklistWrapper block list.
type DisallowListNotificationConsumer struct {
	component.Component
	cm *component.ComponentManager

	nodeBlockListConsumers []p2p.DisallowListConsumer
	handler                *handler.AsyncEventHandler
	logger                 zerolog.Logger
	lock                   sync.RWMutex
}

var _ p2p.DisallowListConsumer = (*DisallowListNotificationConsumer)(nil)

func DefaultDisallowListNotificationConsumer(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *DisallowListNotificationConsumer {
	cfg := &queue.HeroStoreConfig{
		SizeLimit: DefaultBlockListNotificationQueueCacheSize,
		Collector: metrics.NewNoopCollector(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	store := queue.NewHeroStore(cfg.SizeLimit, logger, cfg.Collector)
	return NewDisallowListConsumer(logger, store)
}

func NewDisallowListConsumer(logger zerolog.Logger, store engine.MessageStore) *DisallowListNotificationConsumer {
	h := handler.NewAsyncEventHandler(logger, store, blockListDistributorWorkerCount)

	d := &DisallowListNotificationConsumer{
		nodeBlockListConsumers: make([]p2p.DisallowListConsumer, 0),
		handler:                h,
		logger:                 logger.With().Str("component", "node_blocklist_distributor").Logger(),
	}

	h.RegisterProcessor(d.ProcessQueuedNotifications)

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()

		h.Start(ctx)
		<-h.Ready()
		d.logger.Info().Msg("node block list distributor started")

		<-ctx.Done()
		d.logger.Debug().Msg("node block list distributor shutting down")
	})

	d.cm = cm.Build()
	d.Component = d.cm

	return d
}

// BlockListUpdateNotification is the event that is submitted to the handler when the block list is updated.
type BlockListUpdateNotification struct {
	BlockList flow.IdentifierList
}

func (d *DisallowListNotificationConsumer) AddConsumer(consumer p2p.DisallowListConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.nodeBlockListConsumers = append(d.nodeBlockListConsumers, consumer)
}

func (d *DisallowListNotificationConsumer) OnNodeBlockListUpdate(blockList flow.IdentifierList) {
	// we submit the block list update event to the handler to be processed by the worker.
	// the origin id is set to flow.ZeroID because the block list update event is not associated with a specific node.
	// the distributor discards the origin id upon processing it.
	err := d.handler.Submit(flow.ZeroID, BlockListUpdateNotification{
		BlockList: blockList,
	})

	if err != nil {
		d.logger.Fatal().Err(err).Msg("failed to submit block list update event to handler")
	}
}

func (d *DisallowListNotificationConsumer) ProcessQueuedNotifications(_ flow.Identifier, notification interface{}) {
	var consumers []p2p.DisallowListConsumer
	d.lock.RLock()
	consumers = d.nodeBlockListConsumers
	d.lock.RUnlock()

	switch notification := notification.(type) {
	case BlockListUpdateNotification:
		for _, consumer := range consumers {
			consumer.OnNodeBlockListUpdate(notification.BlockList)
		}
	default:
		d.logger.Fatal().Msgf("unknown notification type: %T", notification)
	}
}
