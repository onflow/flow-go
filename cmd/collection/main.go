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
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	hotstuffmodel "github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/persister"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/consensus/recovery/cluster"
	protocolRecovery "github.com/dapperlabs/flow-go/consensus/recovery/protocol"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	clustermodel "github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	builder "github.com/dapperlabs/flow-go/module/builder/collection"
	colfinalizer "github.com/dapperlabs/flow-go/module/finalizer/collection"
	confinalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/ingress"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/signature"
	clusterkv "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
	storage "github.com/dapperlabs/flow-go/storage"
	storagekv "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/logging"
)

const (
	proposalTimeout = time.Second * 4
)

func main() {

	var (
		txLimit             uint
		maxCollectionSize   uint
		ingressExpiryBuffer uint
		builderExpiryBuffer uint
		hotstuffTimeout     time.Duration

		ingressConf     ingress.Config
		pool            mempool.Transactions
		transactions    *storagekv.Transactions
		colHeaders      *storagekv.Headers
		colPayloads     *storagekv.ClusterPayloads
		colCacheMetrics module.CacheMetrics

		colCache *buffer.PendingClusterBlocks // pending block cache for cluster consensus
		conCache *buffer.PendingBlocks        // pending block cache for follower

		myCluster    flow.IdentityList // cluster identity list
		clusterID    string            // chain ID for the cluster
		clusterState *clusterkv.State  // chain state for the cluster

		// from bootstrap files
		clusterGenesis *clustermodel.Block              // genesis block for the cluster
		clusterQC      *hotstuffmodel.QuorumCertificate // QC for the cluster

		prov           *provider.Engine
		ing            *ingest.Engine
		colMetrics     module.CollectionMetrics
		clusterMetrics module.HotstuffMetrics
		err            error
	)

	cmd.FlowNode(flow.RoleCollection.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&txLimit, "tx-limit", 50000, "maximum number of transactions in the memory pool")
			flags.UintVar(&ingressExpiryBuffer, "ingress-expiry-buffer", 30, "expiry buffer for inbound transactions")
			flags.UintVar(&builderExpiryBuffer, "builder-expiry-buffer", 15, "expiry buffer for transactions in proposed collections")
			flags.UintVar(&maxCollectionSize, "max-collection-size", 100, "maximum number of transactions in proposed collections")
			flags.StringVarP(&ingressConf.ListenAddr, "ingress-addr", "i", "localhost:9000", "the address the ingress server listens on")
			flags.DurationVar(&hotstuffTimeout, "hotstuff-timeout", proposalTimeout, "the initial timeout for the hotstuff pacemaker")
		}).
		Module("transactions mempool", func(node *cmd.FlowNodeBuilder) error {
			pool, err = stdmap.NewTransactions(txLimit)
			return err
		}).
		Module("persistent storage", func(node *cmd.FlowNodeBuilder) error {
			colCacheMetrics = metrics.NewCacheCollector("cluster")
			transactions = storagekv.NewTransactions(node.DB)
			colHeaders = storagekv.NewHeaders(colCacheMetrics, node.DB)
			colPayloads = storagekv.NewClusterPayloads(colCacheMetrics, node.DB)
			return nil
		}).
		Module("block mempool", func(node *cmd.FlowNodeBuilder) error {
			colCache = buffer.NewPendingClusterBlocks()
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		Module("collection cluster ID", func(node *cmd.FlowNodeBuilder) error {
			myCluster, err = protocol.ClusterFor(node.State.Final(), node.Me.NodeID())
			if err != nil {
				return fmt.Errorf("could not get my cluster: %w", err)
			}
			clusterID = protocol.ChainIDForCluster(myCluster)
			return nil
		}).
		Module("collection node metrics", func(node *cmd.FlowNodeBuilder) error {
			colMetrics = metrics.NewCollectionCollector(node.Tracer)
			return nil
		}).
		Module("hotstuff cluster metrics", func(node *cmd.FlowNodeBuilder) error {
			clusterMetrics = metrics.NewHotstuffCollector(clusterID)
			return nil
		}).
		// regardless of whether we are starting from scratch or from an
		// existing state, we load the genesis files
		Module("cluster consensus bootstrapping", func(node *cmd.FlowNodeBuilder) error {

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

			// initialize cluster state
			clusterState, err = clusterkv.NewState(node.DB, clusterID, colHeaders, colPayloads)
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

			// initialize cleaner for DB
			cleaner := storagekv.NewCleaner(node.Logger, node.DB)

			// create a finalizer that will handling updating the protocol
			// state when the follower detects newly finalized blocks
			finalizer := confinalizer.NewFinalizer(node.DB, node.Storage.Headers, node.Storage.Payloads, node.State)

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

			// use proper engine for notifier to follower
			notifier := notifications.NewNoopConsumer()

			finalized, pending, err := protocolRecovery.FindLatest(node.State, node.Storage.Headers, node.GenesisBlock.Header)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			// creates a consensus follower with noop consumer as the notifier
			core, err := consensus.NewFollower(node.Logger, mainConsensusCommittee, node.Storage.Headers, finalizer, verifier, notifier, node.GenesisBlock.Header, node.GenesisQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not create follower core logic: %w", err)
			}

			follower, err := followereng.New(node.Logger, node.Network, node.Me, node.Metrics.Engine, cleaner, node.Storage.Headers, node.Storage.Payloads, node.State, conCache, core)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			// create a block synchronization engine to handle follower getting
			// out of sync
			sync, err := synchronization.New(node.Logger, node.Metrics.Engine, node.Network, node.Me, node.State, node.Storage.Blocks, follower)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			return follower.WithSynchronization(sync), nil
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ing, err = ingest.New(node.Logger, node.Network, node.State, node.Metrics.Engine, colMetrics, node.Me, pool, ingressExpiryBuffer)
			return ing, err
		}).
		Component("ingress server", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			server := ingress.New(ingressConf, ing)
			return server, nil
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			collections := storagekv.NewCollections(node.DB)
			prov, err = provider.New(node.Logger, node.Network, node.State, node.Metrics.Engine, colMetrics, node.Me, pool, collections, transactions)
			return prov, err
		}).
		Component("proposal engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			builder := builder.NewBuilder(node.DB, colHeaders, colPayloads, pool,
				builder.WithMaxCollectionSize(maxCollectionSize),
				builder.WithExpiryBuffer(builderExpiryBuffer),
			)
			finalizer := colfinalizer.NewFinalizer(node.DB, pool, prov, colMetrics, clusterID)

			prop, err := proposal.New(node.Logger, node.Network, node.Me, colMetrics, node.Metrics.Engine, node.State, clusterState, ing, pool, transactions, colHeaders, colPayloads, colCache)
			if err != nil {
				return nil, fmt.Errorf("could not initialize engine: %w", err)
			}

			// collector cluster's HotStuff committee state
			committee, err := initClusterCommittee(node, colPayloads)
			if err != nil {
				return nil, fmt.Errorf("creating HotStuff committee state failed: %w", err)
			}

			// create a signing provider for signing HotStuff messages (within cluster)
			staking := signature.NewAggregationProvider(encoding.CollectorVoteTag, node.Me)
			signer := verification.NewSingleSigner(committee, staking, node.Me.NodeID())

			// initialize logging notifier for hotstuff
			notifier := createNotifier(node.Logger, clusterMetrics)

			persist := persister.New(node.DB)

			finalized, pending, err := cluster.FindLatest(clusterState, colHeaders)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve finalized/pending headers: %w", err)
			}

			hot, err := consensus.NewParticipant(
				node.Logger,
				notifier,
				clusterMetrics,
				colHeaders,
				committee,
				builder,
				finalizer,
				persist,
				signer,
				prop,
				clusterGenesis.Header,
				clusterQC,
				finalized,
				pending,
				consensus.WithTimeout(hotstuffTimeout),
			)
			if err != nil {
				return nil, fmt.Errorf("creating HotStuff participant failed: %w", err)
			}

			prop = prop.WithConsensus(hot)
			return prop, nil
		}).
		Run()
}

// initClusterCommittee initializes the collector cluster's HotStuff committee state
func initClusterCommittee(node *cmd.FlowNodeBuilder, colPayloads *storagekv.ClusterPayloads) (hotstuff.Committee, error) {

	// create a filter for consensus members for our cluster
	cluster, err := protocol.ClusterFor(node.State.Final(), node.Me.NodeID())
	if err != nil {
		return nil, fmt.Errorf("could not get cluster members for node %x: %w", node.Me.NodeID(), err)
	}
	selector := filter.And(filter.In(cluster), filter.HasStake(true))

	translator := clusterkv.NewTranslator(colPayloads)

	return committee.New(node.State, translator, node.Me.NodeID(), selector, cluster.NodeIDs()), nil
}

func loadClusterBlock(path string, clusterID string) (*clustermodel.Block, error) {
	filename := fmt.Sprintf(bootstrap.FilenameGenesisClusterBlock, clusterID)
	data, err := ioutil.ReadFile(filepath.Join(path, filename))
	if err != nil {
		return nil, err
	}

	var block clustermodel.Block
	err = json.Unmarshal(data, &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func loadClusterQC(path string, clusterID string) (*hotstuffmodel.QuorumCertificate, error) {
	filename := fmt.Sprintf(bootstrap.FilenameGenesisClusterQC, clusterID)
	data, err := ioutil.ReadFile(filepath.Join(path, filename))
	if err != nil {
		return nil, err
	}

	var qc hotstuffmodel.QuorumCertificate
	err = json.Unmarshal(data, &qc)
	if err != nil {
		return nil, err
	}
	return &qc, nil
}
