package node_builder

import (
	access2 "github.com/onflow/flow-go/cmd/access/node_builder"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine/access/ingestion"
	"github.com/onflow/flow-go/engine/access/rpc"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/state/protocol"
)

func NewObserverNodeBuilder(builder *access2.FlowAccessNodeBuilder) *access2.UnstakedAccessNodeBuilder {
	// the unstaked access node gets a version of the root snapshot file that does not contain any node addresses
	// hence skip all the root snapshot validations that involved an identity address
	builder.SkipNwAddressBasedValidations = true
	return &access2.UnstakedAccessNodeBuilder{
		FlowAccessNodeBuilder: builder,
	}
}

// ObserverNodeBuilder extends cmd.NodeBuilder and declares additional functions needed to bootstrap an Observer node.
// The Staked network allows the staked nodes to communicate among themselves, while the unstaked network allows the
// unstaked nodes and a staked Access node to communicate. Observer nodes can be attached to an unstaked or staked
// access node to fetch read only block, and execution information. Observer nodes are scalable.
//
//                                 unstaked network                           staked network
//  +------------------------+
//  |        Observer Node 1 |
//  +------------------------+
//              |
//              v
//  +------------------------+
//  |        Observer Node 2 |<--------------------------|
//  +------------------------+                           |
//              |                                        |
//              v                                        v
//  +------------------------+                         +--------------------+                 +------------------------+
//  | Unstaked Access Node 1 |<----------------------->| Staked Access Node |<--------------->| All other staked Nodes |
//  +------------------------+                         +--------------------+                 +------------------------+
//  +------------------------+                           ^
//  | Unstaked Access Node 2 |<--------------------------|
//  +------------------------+

type ObserverNodeBuilder interface {
	cmd.NodeBuilder
}

type PublicNetworkConfig struct {
	// NetworkKey crypto.PublicKey // TODO: do we need a different key for the public network?
	BindAddress string
	Network     network.Network
	Metrics     module.NetworkMetrics
}

// FlowAccessNodeBuilder provides the common functionality needed to bootstrap a Flow staked and unstaked access node
// It is composed of the FlowNodeBuilder, the AccessNodeConfig and contains all the components and modules needed for the
// staked and unstaked access nodes
type FlowObserverNodeBuilder struct {
	*cmd.FlowNodeBuilder
	*access2.ObserverNodeConfig

	// components
	LibP2PNode                 *p2p.Node
	FollowerState              protocol.MutableState
	SyncCore                   *synchronization.Core
	RpcEng                     *rpc.Engine
	FinalizationDistributor    *pubsub.FinalizationDistributor
	FinalizedHeader            *synceng.FinalizedHeaderCache
	CollectionRPC              access.AccessAPIClient
	TransactionTimings         *stdmap.TransactionTimings
	CollectionsToMarkFinalized *stdmap.Times
	CollectionsToMarkExecuted  *stdmap.Times
	BlocksToMarkExecuted       *stdmap.Times
	TransactionMetrics         module.TransactionMetrics
	PingMetrics                module.PingMetrics
	Committee                  hotstuff.Committee
	Finalized                  *flow.Header
	Pending                    []*flow.Header
	FollowerCore               module.HotStuffFollower
	// for the unstaked access node, the sync engine participants provider is the libp2p peer store which is not
	// available until after the network has started. Hence, a factory function that needs to be called just before
	// creating the sync engine
	SyncEngineParticipantsProviderFactory func() id.IdentifierProvider

	// engines
	IngestEng   *ingestion.Engine
	RequestEng  *requester.Engine
	FollowerEng *followereng.Engine
	SyncEng     *synceng.Engine
}

func FlowObserverNode() *access2.FlowAccessNodeBuilder {
	config := access2.DefaultObserverNodeConfig()
	accessConfig := access2.NewObserverNodeConfig(config)

	return access2.NewObserverNodeBuilder(accessConfig, config)
}


