package factories

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/engine/collection/epochmgr"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/epochs"
	chainsync "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type EpochComponentsFactory struct {
	me       module.Local
	pools    *epochs.TransactionPools
	builder  *BuilderFactory
	state    *ClusterStateFactory
	hotstuff *HotStuffFactory
	proposal *ProposalEngineFactory
	sync     *SyncEngineFactory
}

func NewEpochComponentsFactory(
	me module.Local,
	pools *epochs.TransactionPools,
	builder *BuilderFactory,
	state *ClusterStateFactory,
	hotstuff *HotStuffFactory,
	proposal *ProposalEngineFactory,
	sync *SyncEngineFactory,
) *EpochComponentsFactory {

	factory := &EpochComponentsFactory{
		me:       me,
		pools:    pools,
		builder:  builder,
		state:    state,
		hotstuff: hotstuff,
		proposal: proposal,
		sync:     sync,
	}
	return factory
}

func (factory *EpochComponentsFactory) Create(
	epoch protocol.Epoch,
) (
	state cluster.State,
	proposal network.Engine,
	sync network.Engine,
	hotstuff module.HotStuff,
	voteAggregator hotstuff.VoteAggregator,
	err error,
) {

	counter, err := epoch.Counter()
	if err != nil {
		err = fmt.Errorf("could not get epoch counter: %w", err)
		return
	}

	// if we are not an authorized participant in this epoch, return a sentinel
	identities, err := epoch.InitialIdentities()
	if err != nil {
		err = fmt.Errorf("could not get initial identities for epoch: %w", err)
		return
	}
	_, exists := identities.ByNodeID(factory.me.NodeID())
	if !exists {
		err = fmt.Errorf("%w (node_id=%x, epoch=%d)", epochmgr.ErrNotAuthorizedForEpoch, factory.me.NodeID(), counter)
		return
	}

	// determine this node's cluster for the epoch
	clusters, err := epoch.Clustering()
	if err != nil {
		err = fmt.Errorf("could not get clusters for epoch: %w", err)
		return
	}
	_, clusterIndex, ok := clusters.ByNodeID(factory.me.NodeID())
	if !ok {
		err = fmt.Errorf("could not find my cluster")
		return
	}
	cluster, err := epoch.Cluster(clusterIndex)
	if err != nil {
		err = fmt.Errorf("could not get cluster info: %w", err)
		return
	}

	// create the cluster state
	var (
		headers  storage.Headers
		payloads storage.ClusterPayloads
		blocks   storage.ClusterBlocks
	)

	stateRoot, err := badger.NewStateRoot(cluster.RootBlock())
	if err != nil {
		err = fmt.Errorf("could not create valid state root: %w", err)
		return
	}
	var mutableState *badger.MutableState
	mutableState, headers, payloads, blocks, err = factory.state.Create(stateRoot)
	state = mutableState
	if err != nil {
		err = fmt.Errorf("could not create cluster state: %w", err)
		return
	}

	// get the transaction pool for the epoch
	pool := factory.pools.ForEpoch(counter)

	builder, finalizer, err := factory.builder.Create(headers, payloads, pool)
	if err != nil {
		err = fmt.Errorf("could not create builder/finalizer: %w", err)
		return
	}

	hotstuffModules, metrics, err := factory.hotstuff.CreateModules(epoch, cluster, state, headers, payloads, finalizer)
	if err != nil {
		err = fmt.Errorf("could not create consensus modules: %w", err)
		return
	}
	voteAggregator = hotstuffModules.Aggregator

	proposalEng, err := factory.proposal.Create(mutableState, headers, payloads, hotstuffModules.Aggregator)
	if err != nil {
		err = fmt.Errorf("could not create proposal engine: %w", err)
		return
	}

	var syncCore *chainsync.Core
	syncCore, sync, err = factory.sync.Create(cluster.Members(), state, blocks, proposalEng)
	if err != nil {
		err = fmt.Errorf("could not create sync engine: %w", err)
		return
	}
	hotstuff, err = factory.hotstuff.Create(
		state,
		metrics,
		builder,
		headers,
		proposalEng,
		hotstuffModules,
	)
	if err != nil {
		err = fmt.Errorf("could not create hotstuff: %w", err)
		return
	}

	hotstuffModules.FinalizationDistributor.AddOnBlockFinalizedConsumer(proposalEng.OnFinalizedBlock)

	// attach dependencies to the proposal engine
	proposal = proposalEng.
		WithConsensus(hotstuff).
		WithSync(syncCore)

	return
}
