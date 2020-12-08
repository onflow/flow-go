package factories

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/epochs"
	chainsync "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/state/cluster"
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
	proposal module.Engine,
	sync module.Engine,
	hotstuff module.HotStuff,
	err error,
) {

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
	state, headers, payloads, blocks, err = factory.state.Create(cluster.ChainID())
	if err != nil {
		err = fmt.Errorf("could not create cluster state: %w", err)
		return
	}
	_, err = state.Final().Head()
	// storage layer error while checking state - fail fast
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		err = fmt.Errorf("could not check cluster state db: %w", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		// no existing cluster state, bootstrap with root block for epoch
		err = state.Bootstrap(cluster.RootBlock())
		if err != nil {
			err = fmt.Errorf("could not bootstrap cluster state: %w", err)
			return
		}
	}

	// get the transaction pool for the epoch
	counter, err := epoch.Counter()
	if err != nil {
		err = fmt.Errorf("could not get epoch counter: %w", err)
		return
	}
	pool := factory.pools.ForEpoch(counter)

	builder, finalizer, err := factory.builder.Create(headers, payloads, pool)
	if err != nil {
		err = fmt.Errorf("could not create builder/finalizer: %w", err)
		return
	}

	proposalEng, err := factory.proposal.Create(state, headers, payloads)
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
		epoch,
		cluster,
		state,
		headers,
		payloads,
		builder,
		finalizer,
		proposalEng,
	)
	if err != nil {
		err = fmt.Errorf("could not create hotstuff: %w", err)
		return
	}

	// attach dependencies to the proposal engine
	proposal = proposalEng.
		WithHotStuff(hotstuff).
		WithSync(syncCore)

	return
}
