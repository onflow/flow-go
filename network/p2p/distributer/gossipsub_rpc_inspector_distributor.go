package distributer

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
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
	DefaultGossipSubInspectorNotificationQueueCacheSize   = 10_000
	DefaultGossipSubInspectorNotificationQueueWorkerCount = 1
)

// GossipSubInspectorNotificationDistributor is a component that distributes gossipsub rpc inspector notifications to
// registered consumers.
type GossipSubInspectorNotificationDistributor struct {
	component.Component
	cm *component.ComponentManager

	logger    zerolog.Logger
	handler   *handler.AsyncEventHandler
	notifiers []p2p.GossipSubRpcInspectorConsumer
	lock      sync.RWMutex
}

var _ p2p.GossipSubRpcInspectorConsumer = (*GossipSubInspectorNotificationDistributor)(nil)

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

func NewGossipSubInspectorNotificationDistributor(log zerolog.Logger, store engine.MessageStore) *GossipSubInspectorNotificationDistributor {
	h := handler.NewAsyncEventHandler(log, store, DefaultGossipSubInspectorNotificationQueueWorkerCount)
	g := &GossipSubInspectorNotificationDistributor{
		handler: h,
		logger:  log.With().Str("component", "gossipsub_rpc_inspector_distributor").Logger(),
	}

	g.handler.RegisterProcessor(g.ProcessQueuedNotifications)

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()

		h.Start(ctx)
		<-h.Ready()
		g.logger.Info().Msg("node block list distributor started")

		<-ctx.Done()
		g.logger.Debug().Msg("node block list distributor shutting down")
	})

	g.cm = cm.Build()
	g.Component = g.cm

	return g
}

type invalidControlMessage struct {
	peerID  peer.ID
	msgType p2p.ControlMessageType
	count   int
}

func (g *GossipSubInspectorNotificationDistributor) OnInvalidControlMessage(id peer.ID, messageType p2p.ControlMessageType, i int) {
	err := g.handler.Submit(flow.ZeroID, invalidControlMessage{
		peerID:  id,
		msgType: messageType,
		count:   i,
	})
	if err != nil {
		g.logger.Fatal().Err(err).Msg("failed to submit invalid control message event to handler")
	}
}

func (g *GossipSubInspectorNotificationDistributor) AddConsumer(consumer p2p.GossipSubRpcInspectorConsumer) {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.notifiers = append(g.notifiers, consumer)
}

func (g *GossipSubInspectorNotificationDistributor) ProcessQueuedNotifications(_ flow.Identifier, notification interface{}) {
	var consumers []p2p.GossipSubRpcInspectorConsumer
	g.lock.RLock()
	consumers = g.notifiers
	g.lock.RUnlock()

	switch notification := notification.(type) {
	case invalidControlMessage:
		for _, notifier := range consumers {
			notifier.OnInvalidControlMessage(notification.peerID, notification.msgType, notification.count)
		}
	default:
		g.logger.Fatal().Msgf("unknown notification type: %T", notification)
	}
}
