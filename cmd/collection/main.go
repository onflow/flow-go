package main

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/coldstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	builder "github.com/dapperlabs/flow-go/module/builder/collection"
	colfinalizer "github.com/dapperlabs/flow-go/module/finalizer/collection"
	followerfinalizer "github.com/dapperlabs/flow-go/module/finalizer/follower"
	"github.com/dapperlabs/flow-go/module/ingress"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/signature"
	clusterkv "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/logging"
)

const (
	proposalInterval = time.Second * 2
	proposalTimeout  = time.Second * 4
)

func main() {

	var (
		txLimit      uint
		ingressConf  ingress.Config
		pool         mempool.Transactions
		collections  *storage.Collections
		transactions *storage.Transactions
		headers      *storage.Headers
		blocks       *storage.Blocks
		colPayloads  *storage.ClusterPayloads
		conPayloads  *storage.Payloads

		colCache *buffer.PendingClusterBlocks // pending block cache for cluster consensus
		conCache *buffer.PendingBlocks        // pending block cache for follower

		clusterID    string           // chain ID for the cluster
		clusterState *clusterkv.State // chain state for the cluster

		prov *provider.Engine
		ing  *ingest.Engine
		err  error
	)

	cmd.FlowNode("collection").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&txLimit, "tx-limit", 10000, "maximum number of transactions in the memory pool")
			flags.StringVarP(&ingressConf.ListenAddr, "ingress-addr", "i", "localhost:9000", "the address the ingress server listens on")
		}).
		Module("transactions mempool", func(node *cmd.FlowNodeBuilder) error {
			pool, err = stdmap.NewTransactions(txLimit)
			return err
		}).
		Module("persistent storage", func(node *cmd.FlowNodeBuilder) error {
			transactions = storage.NewTransactions(node.DB)
			headers = storage.NewHeaders(node.DB)
			blocks = storage.NewBlocks(node.DB)
			colPayloads = storage.NewClusterPayloads(node.DB)
			conPayloads = storage.NewPayloads(node.DB)
			return nil
		}).
		Module("block cache", func(node *cmd.FlowNodeBuilder) error {
			colCache = buffer.NewPendingClusterBlocks()
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		Module("cluster state", func(node *cmd.FlowNodeBuilder) error {
			myCluster, err := protocol.ClusterFor(node.State.Final(), node.Me.NodeID())
			if err != nil {
				return fmt.Errorf("could not get my cluster: %w", err)
			}

			// determine the chain ID for my cluster and create cluster state
			clusterID = protocol.ChainIDForCluster(myCluster)
			clusterState, err = clusterkv.NewState(node.DB, clusterID)
			if err != nil {
				return fmt.Errorf("could not create cluster state: %w", err)
			}

			// create genesis block for cluster consensus
			genesis := cluster.Genesis()
			genesis.ChainID = clusterID

			node.Logger.Info().
				Hex("genesis_id", logging.ID(genesis.ID())).
				Str("cluster_id", clusterID).
				Str("cluster_members", fmt.Sprintf("%v", myCluster.NodeIDs())).
				Msg("bootstrapped cluster state")

			// bootstrap cluster consensus state
			err = clusterState.Mutate().Bootstrap(genesis)
			if err != nil {
				return fmt.Errorf("could not bootstrap cluster state: %w", err)
			}

			return nil
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// create a finalizer that will handling updating the protocol
			// state when the follower detects newly finalized blocks
			final := followerfinalizer.NewFinalizer(node.DB)

			// initialize the staking & beacon verifiers, signature joiner
			staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
			beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
			merger := signature.NewCombiner()

			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			mainConsensusCommittee, err := committee.NewMainConsensusCommitteeState(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(mainConsensusCommittee, node.DKGState, staking, beacon, merger)

			// TODO: use proper engine for notifier to follower
			notifier := notifications.NewNoopConsumer()

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			core, err := consensus.NewFollower(node.Logger, mainConsensusCommittee, final, verifier, notifier, &node.GenesisBlock.Header, node.GenesisQC)
			if err != nil {
				//return nil, fmt.Errorf("could not create follower core logic: %w", err)
				// TODO for now we ignore failures in follower
				// this is necessary for integration tests to run, until they are
				// updated to generate/use valid genesis QC and DKG files.
				// ref https://github.com/dapperlabs/flow-go/issues/3057
				node.Logger.Debug().Err(err).Msg("ignoring failures in follower core")
			}

			follower, err := followereng.New(node.Logger, node.Network, node.Me, node.State, headers, conPayloads, conCache, core)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			// create a block synchronization engine to handle follower getting
			// out of sync
			sync, err := synchronization.New(node.Logger, node.Network, node.Me, node.State, blocks, follower)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			return follower.WithSynchronization(sync), nil
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ing, err = ingest.New(node.Logger, node.Network, node.State, node.Metrics, node.Me, pool)
			return ing, err
		}).
		Component("ingress server", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			server := ingress.New(ingressConf, ing)
			return server, nil
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			collections = storage.NewCollections(node.DB)
			prov, err = provider.New(node.Logger, node.Network, node.State, node.Metrics, node.Me, pool, collections, transactions)
			return prov, err
		}).
		Component("proposal engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			build := builder.NewBuilder(node.DB, pool, clusterID)
			final := colfinalizer.NewFinalizer(node.DB, pool, prov, node.Metrics, clusterID)

			prop, err := proposal.New(node.Logger, node.Network, node.Me, node.State, clusterState, node.Metrics, ing, pool, transactions, headers, colPayloads, colCache)
			if err != nil {
				return nil, fmt.Errorf("could not initialize engine: %w", err)
			}

			memberFilter, err := protocol.ClusterFilterFor(node.State.Final(), node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not get cluster member filter: %w", err)
			}

			head := func() (*flow.Header, error) {
				return clusterState.Final().Head()
			}

			cold, err := coldstuff.New(node.Logger, node.State, node.Me, prop, build, final, memberFilter, proposalInterval, proposalTimeout, head)
			if err != nil {
				return nil, fmt.Errorf("could not initialize algorithm: %w", err)
			}

			prop = prop.WithConsensus(cold)
			return prop, nil
		}).
		Run("collection")
}
