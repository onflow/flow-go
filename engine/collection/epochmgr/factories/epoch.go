package factories

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/indices"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	chainsync "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type EpochComponentsFactory struct {
	me       module.Local
	pool     mempool.Transactions // TODO make per-epoch
	builder  *BuilderFactory
	state    *ClusterStateFactory
	hotstuff *HotStuffFactory
	proposal *ProposalEngineFactory
	sync     *SyncEngineFactory
}

func NewEpochComponentsFactory(
	me module.Local,
	pool mempool.Transactions, // TODO make per-epoch
	builder *BuilderFactory,
	state *ClusterStateFactory,
	hotstuff *HotStuffFactory,
	proposal *ProposalEngineFactory,
	sync *SyncEngineFactory,
) *EpochComponentsFactory {

	factory := &EpochComponentsFactory{
		me:       me,
		pool:     pool,
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

	fmt.Println("EpochComponentsFactory.Create Clustering")
	// determine this node's cluster for the epoch
	clusters, err := epoch.Clustering()
	if err != nil {
		err = fmt.Errorf("could not get clusters for epoch: %w", err)
		return
	}
	fmt.Println("EpochComponentsFactory.Create ByNodeID")
	_, clusterIndex, ok := clusters.ByNodeID(factory.me.NodeID())
	if !ok {
		err = fmt.Errorf("could not find my cluster")
		return
	}
	fmt.Println("EpochComponentsFactory.Create Cluster")
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
	fmt.Println("EpochComponentsFactory.Create clsuter state")
	state, headers, payloads, blocks, err = factory.state.Create(cluster.ChainID())
	if err != nil {
		err = fmt.Errorf("could not create cluster state: %w", err)
		return
	}
	fmt.Println("EpochComponentsFactory.Create final head")
	_, err = state.Final().Head()
	fmt.Println("EpochComponentsFactory.Create final head err: ", err)
	// storage layer error while checking state - fail fast
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		err = fmt.Errorf("could not check cluster state db: %w", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		// no existing cluster state, bootstrap with root block for epoch
		fmt.Println("EpochComponentsFactory.Create Bootstrap")
		err = state.Mutate().Bootstrap(cluster.RootBlock())
		if err != nil {
			err = fmt.Errorf("could not bootstrap cluster state: %w", err)
			return
		}
	}

	fmt.Println("EpochComponentsFactory.Create builder/finalzier")
	builder, finalizer, err := factory.builder.Create(headers, payloads, factory.pool)
	if err != nil {
		err = fmt.Errorf("could not create builder/finalizer: %w", err)
		return
	}

	fmt.Println("EpochComponentsFactory.Create seed")
	seed, err := epoch.Seed(indices.ProtocolCollectorClusterLeaderSelection(clusterIndex)...)
	if err != nil {
		err = fmt.Errorf("could not get leader selection seed: %w", err)
		return
	}

	fmt.Println("EpochComponentsFactory.Create proposaleng")
	proposalEng, err := factory.proposal.Create(state, headers, payloads)
	if err != nil {
		err = fmt.Errorf("could not create proposal engine: %w", err)
		return
	}

	fmt.Println("EpochComponentsFactory.Create sync")
	var syncCore *chainsync.Core
	syncCore, sync, err = factory.sync.Create(cluster.Members(), state, blocks, proposalEng)
	if err != nil {
		err = fmt.Errorf("could not create sync engine: %w", err)
		return
	}
	fmt.Println("EpochComponentsFactory.Create hotstuff")
	hotstuff, err = factory.hotstuff.Create(
		cluster.ChainID(),
		cluster.Members(),
		state,
		headers,
		payloads,
		seed,
		builder,
		finalizer,
		proposalEng,
		cluster.RootBlock().Header,
		cluster.RootQC(),
	)
	if err != nil {
		err = fmt.Errorf("could not create hotstuff: %w", err)
		return
	}

	fmt.Println("EpochComponentsFactory.Create attach to proposal eng")
	// attach dependencies to the proposal engine
	proposal = proposalEng.
		WithHotStuff(hotstuff).
		WithSync(syncCore)

	return
}
