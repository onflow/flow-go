package factories

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/engine/collection/epochmgr"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type EpochComponentsFactory struct {
	me         module.Local
	pools      *epochs.TransactionPools
	builder    *BuilderFactory
	state      *ClusterStateFactory
	hotstuff   *HotStuffFactory
	compliance *ComplianceEngineFactory
	sync       *SyncEngineFactory
	syncCore   *SyncCoreFactory
	messageHub *MessageHubFactory
}

var _ epochmgr.EpochComponentsFactory = (*EpochComponentsFactory)(nil)

func NewEpochComponentsFactory(
	me module.Local,
	pools *epochs.TransactionPools,
	builder *BuilderFactory,
	state *ClusterStateFactory,
	hotstuff *HotStuffFactory,
	compliance *ComplianceEngineFactory,
	syncCore *SyncCoreFactory,
	sync *SyncEngineFactory,
	messageHub *MessageHubFactory,
) *EpochComponentsFactory {

	factory := &EpochComponentsFactory{
		me:         me,
		pools:      pools,
		builder:    builder,
		state:      state,
		hotstuff:   hotstuff,
		compliance: compliance,
		syncCore:   syncCore,
		sync:       sync,
		messageHub: messageHub,
	}
	return factory
}

func (factory *EpochComponentsFactory) Create(
	epoch protocol.Epoch,
) (
	state cluster.State,
	compliance component.Component,
	sync module.ReadyDoneAware,
	hotstuff module.HotStuff,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	messageHub component.Component,
	err error,
) {

	epochCounter, err := epoch.Counter()
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
		err = fmt.Errorf("%w (node_id=%x, epoch=%d)", epochmgr.ErrNotAuthorizedForEpoch, factory.me.NodeID(), epochCounter)
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

	stateRoot, err := badger.NewStateRoot(cluster.RootBlock(), cluster.RootQC(), cluster.EpochCounter())
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
	pool := factory.pools.ForEpoch(epochCounter)

	builder, finalizer, err := factory.builder.Create(state, headers, payloads, pool, epochCounter)
	if err != nil {
		err = fmt.Errorf("could not create builder/finalizer: %w", err)
		return
	}

	hotstuffModules, metrics, err := factory.hotstuff.CreateModules(epoch, cluster, state, headers, payloads, finalizer)
	if err != nil {
		err = fmt.Errorf("could not create consensus modules: %w", err)
		return
	}
	voteAggregator = hotstuffModules.VoteAggregator
	timeoutAggregator = hotstuffModules.TimeoutAggregator
	validator := hotstuffModules.Validator

	hotstuff, err = factory.hotstuff.Create(
		cluster,
		state,
		metrics,
		builder,
		headers,
		hotstuffModules,
	)
	if err != nil {
		err = fmt.Errorf("could not create hotstuff: %w", err)
		return
	}

	syncCore, err := factory.syncCore.Create(cluster.ChainID())
	if err != nil {
		err = fmt.Errorf("could not create sync core: %w", err)
		return
	}

	complianceEng, err := factory.compliance.Create(
		metrics,
		hotstuffModules.Notifier,
		mutableState,
		headers,
		payloads,
		syncCore,
		hotstuff,
		hotstuffModules.VoteAggregator,
		hotstuffModules.TimeoutAggregator,
		validator,
	)
	if err != nil {
		err = fmt.Errorf("could not create compliance engine: %w", err)
		return
	}
	compliance = complianceEng
	hotstuffModules.Notifier.AddOnBlockFinalizedConsumer(complianceEng.OnFinalizedBlock)

	sync, err = factory.sync.Create(cluster.Members(), state, blocks, syncCore, complianceEng)
	if err != nil {
		err = fmt.Errorf("could not create sync engine: %w", err)
		return
	}

	clusterMessageHub, err := factory.messageHub.Create(state, payloads, hotstuff, complianceEng, hotstuffModules)
	if err != nil {
		err = fmt.Errorf("could not create message hub: %w", err)
	}
	hotstuffModules.Notifier.AddConsumer(clusterMessageHub)
	messageHub = clusterMessageHub

	return
}
