package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus"
	hotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
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
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/logging"
)

const (
	proposalInterval = time.Second * 2
	proposalTimeout  = time.Second * 4
)

func main() {

	var (
		txLimit         uint
		ingressConf     ingress.Config
		hotstuffTimeout time.Duration

		pool         mempool.Transactions
		collections  *badger.Collections
		transactions *badger.Transactions
		headers      *badger.Headers
		blocks       *badger.Blocks
		colPayloads  *badger.ClusterPayloads
		conPayloads  *badger.Payloads
		views        *badger.Views

		colCache *buffer.PendingClusterBlocks // pending block cache for cluster consensus
		conCache *buffer.PendingBlocks        // pending block cache for follower

		clusterID    string           // chain ID for the cluster
		clusterState *clusterkv.State // chain state for the cluster

		// from bootstrap files
		clusterGenesis *cluster.Block              // genesis block for the cluster
		clusterQC      *hotstuff.QuorumCertificate // QC for the cluster

		prov *provider.Engine
		ing  *ingest.Engine
		err  error
	)

	cmd.FlowNode("collection").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&txLimit, "tx-limit", 100000, "maximum number of transactions in the memory pool")
			flags.StringVarP(&ingressConf.ListenAddr, "ingress-addr", "i", "localhost:9000", "the address the ingress server listens on")
			flags.DurationVar(&hotstuffTimeout, "hotstuff-timeout", 2*time.Second, "the initial timeout for the hotstuff pacemaker")
		}).
		Module("transactions mempool", func(node *cmd.FlowNodeBuilder) error {
			pool, err = stdmap.NewTransactions(txLimit)
			return err
		}).
		Module("persistent storage", func(node *cmd.FlowNodeBuilder) error {
			transactions = badger.NewTransactions(node.DB)
			headers = badger.NewHeaders(node.DB)
			blocks = badger.NewBlocks(node.DB)
			colPayloads = badger.NewClusterPayloads(node.DB)
			conPayloads = badger.NewPayloads(node.DB)
			views = badger.NewViews(node.DB)
			return nil
		}).
		Module("block cache", func(node *cmd.FlowNodeBuilder) error {
			colCache = buffer.NewPendingClusterBlocks()
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		// regardless of whether we are starting from scratch or from an
		// existing state, we load the genesis files
		Module("cluster consensus bootstrapping", func(node *cmd.FlowNodeBuilder) error {
			myCluster, err := protocol.ClusterFor(node.State.Final(), node.Me.NodeID())
			if err != nil {
				return fmt.Errorf("could not get my cluster: %w", err)
			}

			clusterID = protocol.ChainIDForCluster(myCluster)

			// read cluster bootstrapping files from standard bootstrap directory
			clusterGenesis, err = loadClusterBlock(node.BaseConfig.BootstrapDir, clusterID)
			if err != nil {
				return fmt.Errorf("could not load cluster block: %w", err)
			}

			clusterQC, err = loadClusterQC(node.BaseConfig.BootstrapDir, clusterID)
			if err != nil {
				return fmt.Errorf("could not load cluster qc: %w", err)
			}

			return nil
		}).
		// if a genesis cluster block already exists in the database, discard
		// the loaded bootstrap files and use the existing cluster state
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

			_, err = clusterState.Final().Head()
			// storage layer error while checking state - fail fast
			if err != nil && !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("could not check cluster state db: %w", err)
			}

			// no existing cluster state, bootstrap with block specified in
			// bootstrapping files
			if errors.Is(err, storage.ErrNotFound) {
				err = clusterState.Mutate().Bootstrap(clusterGenesis)
				if err != nil {
					return fmt.Errorf("could not bootstrap cluster state: %w", err)
				}

				node.Logger.Info().
					Hex("genesis_id", logging.ID(clusterGenesis.ID())).
					Str("cluster_id", clusterID).
					Str("cluster_members", fmt.Sprintf("%v", myCluster.NodeIDs())).
					Msg("bootstrapped cluster state")
			}

			// otherwise, we already have cluster state on-disk, no bootstrapping needed

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

			// define the node set that is valid as signers
			selector := filter.And(filter.HasRole(flow.RoleConsensus), filter.HasStake(true))

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(node.State, node.DKGState, staking, beacon, merger, selector)

			// TODO: use proper engine for notifier to follower
			notifier := notifications.NewNoopConsumer()

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			core, err := consensus.NewFollower(node.Logger, node.State, node.Me, final, verifier, notifier, &node.GenesisBlock.Header, node.GenesisQC, selector)
			if err != nil {
				return nil, fmt.Errorf("could not create follower core logic: %w", err)
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
			collections = badger.NewCollections(node.DB)
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

			// create a filter for consensus members for our cluster
			selector, err := protocol.ClusterFilterFor(node.State.Final(), node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not get cluster member filter: %w", err)
			}

			// create a signing provider for signing votes
			staking := signature.NewAggregationProvider(encoding.CollectorVoteTag, node.Me)
			signer := verification.NewSingleSigner(node.State, staking, selector, node.Me.NodeID())

			// TODO implement notification consumer for col nodes
			var notifier *notifications.NoopConsumer

			hot, err := consensus.NewParticipant(
				node.Logger, notifier, node.Metrics, headers, views, node.State, node.Me,
				build, final, signer, prop, selector, &clusterGenesis.Header, clusterQC,
				consensus.WithTimeout(hotstuffTimeout),
			)

			prop = prop.WithConsensus(hot)
			return prop, nil
		}).
		Run("collection")
}

func loadClusterBlock(path string, clusterID string) (*cluster.Block, error) {
	filename := fmt.Sprintf(bootstrap.FilenameGenesisClusterBlock, clusterID)
	data, err := ioutil.ReadFile(filepath.Join(path, filename))
	if err != nil {
		return nil, err
	}

	var block cluster.Block
	err = json.Unmarshal(data, &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func loadClusterQC(path string, clusterID string) (*hotstuff.QuorumCertificate, error) {
	filename := fmt.Sprintf(bootstrap.FilenameGenesisClusterQC, clusterID)
	data, err := ioutil.ReadFile(filepath.Join(path, filename))
	if err != nil {
		return nil, err
	}

	var qc hotstuff.QuorumCertificate
	err = json.Unmarshal(data, &qc)
	if err != nil {
		return nil, err
	}
	return &qc, nil
}
