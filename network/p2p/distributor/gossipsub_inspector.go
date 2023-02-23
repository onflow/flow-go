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
	// DefaultGossipSubInspectorNotificationQueueCacheSize is the default cache size for the gossipsub rpc inspector notification queue.
	DefaultGossipSubInspectorNotificationQueueCacheSize = 10_000
	// defaultGossipSubInspectorNotificationQueueWorkerCount is the default number of workers that will process the gossipsub rpc inspector notifications.
	defaultGossipSubInspectorNotificationQueueWorkerCount = 1
)

// GossipSubInspectorNotification is a component that distributes gossipsub rpc inspector notifications to
// registered consumers.
type GossipSubInspectorNotification struct {
	component.Component
	cm     *component.ComponentManager
	lock   sync.RWMutex
	logger zerolog.Logger

	// handler is the async event handler that will process the notifications asynchronously.
	handler *handler.AsyncEventHandler

	// notifiers is a list of consumers that will be notified when a new notification is received.
	notifiers []p2p.GossipSubRpcInspectorConsumer
}

var _ p2p.GossipSubRpcInspectorConsumer = (*GossipSubInspectorNotification)(nil)

// DefaultGossipSubInspectorNotification returns a new GossipSubInspectorNotification component with the default configuration.
func DefaultGossipSubInspectorNotification(logger zerolog.Logger, opts ...queue.HeroStoreConfigOption) *GossipSubInspectorNotification {
	cfg := &queue.HeroStoreConfig{
		SizeLimit: DefaultGossipSubInspectorNotificationQueueCacheSize,
		Collector: metrics.NewNoopCollector(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	store := queue.NewHeroStore(cfg.SizeLimit, logger, cfg.Collector)
	return NewGossipSubInspectorNotification(logger, store)
}

// NewGossipSubInspectorNotification returns a new GossipSubInspectorNotification component.
// It takes a message store to store the notifications in memory and process them asynchronously.
func NewGossipSubInspectorNotification(log zerolog.Logger, store engine.MessageStore) *GossipSubInspectorNotification {
	g := &GossipSubInspectorNotification{
		logger: log.With().Str("component", "gossipsub_rpc_inspector_distributor").Logger(),
	}

	h := handler.NewAsyncEventHandler(
		log,
		store,
		g.processQueuedNotifications,
		defaultGossipSubInspectorNotificationQueueWorkerCount)
	g.handler = h

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()

		h.Start(ctx)
		<-h.Ready()
		g.logger.Info().Msg("node disallow list notification distributor started")

		<-ctx.Done()
		g.logger.Debug().Msg("node disallow list distributor shutting down")
	})

	g.cm = cm.Build()
	g.Component = g.cm

	return g
}

// OnInvalidControlMessage is called when a control message is received that is invalid according to the
// Flow protocol specification.
// The int parameter is the count of invalid messages received from the peer.
// Prerequisites:
// Implementation must be concurrency safe and non-blocking.
func (g *GossipSubInspectorNotification) OnInvalidControlMessage(notification p2p.InvalidControlMessageNotification) {
	// we submit notification event to the handler to be processed by the worker.
	// the origin id is set to flow.ZeroID because the notification update event is not associated with a specific node.
	// the distributor discards the origin id upon processing it.
	err := g.handler.Submit(notification)
	if err != nil {
		g.logger.Fatal().Err(err).Msg("failed to submit invalid control message event to handler")
	}
}

// AddConsumer adds the given gossipsub inspector consumer to the distributor so that it can receive
// gossipsub rpc inspector notifications.
func (g *GossipSubInspectorNotification) AddConsumer(consumer p2p.GossipSubRpcInspectorConsumer) {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.notifiers = append(g.notifiers, consumer)
}

// processQueuedNotifications processes the queued notifications. It is called by the handler worker.
func (g *GossipSubInspectorNotification) processQueuedNotifications(_ flow.Identifier, notification interface{}) {
	var consumers []p2p.GossipSubRpcInspectorConsumer
	g.lock.RLock()
	consumers = g.notifiers
	g.lock.RUnlock()

	switch notification := notification.(type) {
	case p2p.InvalidControlMessageNotification:
		for _, consumer := range consumers {
			consumer.OnInvalidControlMessage(notification)
		}
	default:
		g.logger.Fatal().Msgf("unknown notification type: %T", notification)
	}
}
