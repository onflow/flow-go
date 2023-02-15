package distributer

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

// NodeBlockListDistributor subscribes to changes in the NodeBlocklistWrapper block list.
type NodeBlockListDistributor struct {
	component.Component
	cm *component.ComponentManager

	nodeBlockListConsumers []p2p.NodeBlockListConsumer
	handler                *handler.AsyncEventHandler
	logger                 zerolog.Logger
	lock                   sync.RWMutex
}

var _ p2p.NodeBlockListConsumer = (*NodeBlockListDistributor)(nil)

func DefaultNodeBlockListDistributor(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *NodeBlockListDistributor {
	cfg := &queue.HeroStoreConfig{
		SizeLimit: DefaultBlockListNotificationQueueCacheSize,
		Collector: metrics.NewNoopCollector(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	store := queue.NewHeroStore(cfg.SizeLimit, logger, cfg.Collector)
	return NewNodeBlockListDistributor(logger, store)
}

func NewNodeBlockListDistributor(logger zerolog.Logger, store engine.MessageStore) *NodeBlockListDistributor {
	h := handler.NewAsyncEventHandler(logger, store, blockListDistributorWorkerCount)

	d := &NodeBlockListDistributor{
		nodeBlockListConsumers: make([]p2p.NodeBlockListConsumer, 0),
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

// blockListUpdateEvent is the event that is submitted to the handler when the block list is updated.
type blockListUpdateEvent struct {
	blockList flow.IdentifierList
}

func (d *NodeBlockListDistributor) AddConsumer(consumer p2p.NodeBlockListConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.nodeBlockListConsumers = append(d.nodeBlockListConsumers, consumer)
}

func (d *NodeBlockListDistributor) OnNodeBlockListUpdate(blockList flow.IdentifierList) {
	// we submit the block list update event to the handler to be processed by the worker.
	// the origin id is set to flow.ZeroID because the block list update event is not associated with a specific node.
	// the distributor discards the origin id upon processing it.
	err := d.handler.Submit(flow.ZeroID, blockListUpdateEvent{
		blockList: blockList,
	})

	if err != nil {
		d.logger.Fatal().Err(err).Msg("failed to submit block list update event to handler")
	}
}

func (d *NodeBlockListDistributor) ProcessQueuedNotifications(_ flow.Identifier, notification interface{}) {
	var consumers []p2p.NodeBlockListConsumer
	d.lock.RLock()
	consumers = d.nodeBlockListConsumers
	d.lock.RUnlock()

	switch notification := notification.(type) {
	case blockListUpdateEvent:
		for _, consumer := range consumers {
			consumer.OnNodeBlockListUpdate(notification.blockList)
		}
	default:
		d.logger.Fatal().Msgf("unknown notification type: %T", notification)
	}
}
