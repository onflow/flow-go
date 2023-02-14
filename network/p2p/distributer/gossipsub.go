package distributer

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/handler"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
)

type GossipSubInspectorNotificationDistributor struct {
	log       zerolog.Logger
	handler   *handler.AsyncEventHandler
	notifiers []p2p.GossipSubRpcInspectorConsumer
}

func NewGossipSubInspectorNotificationDistributor(log zerolog.Logger, queueSize uint32, workerCount uint) *GossipSubInspectorNotificationDistributor {
	g := &GossipSubInspectorNotificationDistributor{
		handler: handler.NewAsyncEventHandler(log, queueSize, workerCount),
	}

	g.handler.RegisterProcessor(g.ProcessQueuedNotifications)

	return g
}

var _ p2p.GossipSubRpcInspectorConsumer = (*GossipSubInspectorNotificationDistributor)(nil)

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
		g.log.Fatal().Err(err).Msg("failed to submit invalid control message event to handler")
	}
}

func (g *GossipSubInspectorNotificationDistributor) ProcessQueuedNotifications(_ flow.Identifier, notification interface{}) {
	switch notification := notification.(type) {
	case invalidControlMessage:
		for _, notifier := range g.notifiers {
			notifier.OnInvalidControlMessage(notification.peerID, notification.msgType, notification.count)
		}
	default:
		g.log.Fatal().Msgf("unknown notification type: %T", notification)
	}
}
