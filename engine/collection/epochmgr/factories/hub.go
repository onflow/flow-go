package factories

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/engine/collection/message_hub"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type MessageHubFactory struct {
	log           zerolog.Logger
	me            module.Local
	net           network.EngineRegistry
	protoState    protocol.State
	engineMetrics module.EngineMetrics
}

func NewMessageHubFactory(log zerolog.Logger,
	net network.EngineRegistry,
	me module.Local,
	engineMetrics module.EngineMetrics,
	protoState protocol.State) *MessageHubFactory {
	return &MessageHubFactory{
		log:           log,
		me:            me,
		net:           net,
		protoState:    protoState,
		engineMetrics: engineMetrics,
	}
}

func (f *MessageHubFactory) Create(
	clusterState cluster.State,
	payloads storage.ClusterPayloads,
	hotstuff module.HotStuff,
	compliance collection.Compliance,
	hotstuffModules *consensus.HotstuffModules,
) (*message_hub.MessageHub, error) {
	return message_hub.NewMessageHub(
		f.log,
		f.engineMetrics,
		f.net,
		f.me,
		compliance,
		hotstuff,
		hotstuffModules.VoteAggregator,
		hotstuffModules.TimeoutAggregator,
		f.protoState,
		clusterState,
		payloads,
	)
}
