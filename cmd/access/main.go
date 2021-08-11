package main

import (
	"fmt"
	"strings"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/ingestion"
	pingeng "github.com/onflow/flow-go/engine/access/ping"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/synchronization"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	storage "github.com/onflow/flow-go/storage/badger"
	grpcutils "github.com/onflow/flow-go/utils/grpc"
)

func main() {
	defaultConfig := DefaultAccessNodeConfig()

	anb := FlowAccessNode() // use the generic Access Node builder till it is determined if this is a staked AN or an unstaked AN

	anb.PrintBuildVersionDetails()

	anb.ExtraFlags(func(flags *pflag.FlagSet) {
		flags.UintVar(&anb.receiptLimit, "receipt-limit", defaultConfig.receiptLimit, "maximum number of execution receipts in the memory pool")
		flags.UintVar(&anb.collectionLimit, "collection-limit", defaultConfig.collectionLimit, "maximum number of collections in the memory pool")
		flags.UintVar(&anb.blockLimit, "block-limit", defaultConfig.blockLimit, "maximum number of result blocks in the memory pool")
		flags.UintVar(&anb.collectionGRPCPort, "collection-ingress-port", defaultConfig.collectionGRPCPort, "the grpc ingress port for all collection nodes")
		flags.UintVar(&anb.executionGRPCPort, "execution-ingress-port", defaultConfig.executionGRPCPort, "the grpc ingress port for all execution nodes")
		flags.StringVarP(&anb.rpcConf.UnsecureGRPCListenAddr, "rpc-addr", "r", defaultConfig.rpcConf.UnsecureGRPCListenAddr, "the address the unsecured gRPC server listens on")
		flags.StringVar(&anb.rpcConf.SecureGRPCListenAddr, "secure-rpc-addr", defaultConfig.rpcConf.SecureGRPCListenAddr, "the address the secure gRPC server listens on")
		flags.StringVarP(&anb.rpcConf.HTTPListenAddr, "http-addr", "h", defaultConfig.rpcConf.HTTPListenAddr, "the address the http proxy server listens on")
		flags.StringVarP(&anb.rpcConf.CollectionAddr, "static-collection-ingress-addr", "", defaultConfig.rpcConf.CollectionAddr, "the address (of the collection node) to send transactions to")
		flags.StringVarP(&anb.ExecutionNodeAddress, "script-addr", "s", defaultConfig.ExecutionNodeAddress, "the address (of the execution node) forward the script to")
		flags.StringVarP(&anb.rpcConf.HistoricalAccessAddrs, "historical-access-addr", "", defaultConfig.rpcConf.HistoricalAccessAddrs, "comma separated rpc addresses for historical access nodes")
		flags.DurationVar(&anb.rpcConf.CollectionClientTimeout, "collection-client-timeout", defaultConfig.rpcConf.CollectionClientTimeout, "grpc client timeout for a collection node")
		flags.DurationVar(&anb.rpcConf.ExecutionClientTimeout, "execution-client-timeout", defaultConfig.rpcConf.ExecutionClientTimeout, "grpc client timeout for an execution node")
		flags.UintVar(&anb.rpcConf.MaxHeightRange, "rpc-max-height-range", defaultConfig.rpcConf.MaxHeightRange, "maximum size for height range requests")
		flags.StringSliceVar(&anb.rpcConf.PreferredExecutionNodeIDs, "preferred-execution-node-ids", defaultConfig.rpcConf.PreferredExecutionNodeIDs, "comma separated list of execution nodes ids to choose from when making an upstream call e.g. b4a4dbdcd443d...,fb386a6a... etc.")
		flags.StringSliceVar(&anb.rpcConf.FixedExecutionNodeIDs, "fixed-execution-node-ids", defaultConfig.rpcConf.FixedExecutionNodeIDs, "comma separated list of execution nodes ids to choose from when making an upstream call if no matching preferred execution id is found e.g. b4a4dbdcd443d...,fb386a6a... etc.")
		flags.BoolVar(&anb.logTxTimeToFinalized, "log-tx-time-to-finalized", defaultConfig.logTxTimeToFinalized, "log transaction time to finalized")
		flags.BoolVar(&anb.logTxTimeToExecuted, "log-tx-time-to-executed", defaultConfig.logTxTimeToExecuted, "log transaction time to executed")
		flags.BoolVar(&anb.logTxTimeToFinalizedExecuted, "log-tx-time-to-finalized-executed", defaultConfig.logTxTimeToFinalizedExecuted, "log transaction time to finalized and executed")
		flags.BoolVar(&anb.pingEnabled, "ping-enabled", defaultConfig.pingEnabled, "whether to enable the ping process that pings all other peers and report the connectivity to metrics")
		flags.BoolVar(&anb.retryEnabled, "retry-enabled", defaultConfig.retryEnabled, "whether to enable the retry mechanism at the access node level")
		flags.BoolVar(&anb.rpcMetricsEnabled, "rpc-metrics-enabled", defaultConfig.rpcMetricsEnabled, "whether to enable the rpc metrics")
		flags.StringVarP(&anb.nodeInfoFile, "node-info-file", "", defaultConfig.nodeInfoFile, "full path to a json file which provides more details about nodes when reporting its reachability metrics")
		flags.StringToIntVar(&anb.apiRatelimits, "api-rate-limits", defaultConfig.apiRatelimits, "per second rate limits for Access API methods e.g. Ping=300,GetTransaction=500 etc.")
		flags.StringToIntVar(&anb.apiBurstlimits, "api-burst-limits", defaultConfig.apiBurstlimits, "burst limits for Access API methods e.g. Ping=100,GetTransaction=100 etc.")
		flags.BoolVar(&anb.staked, "staked", defaultConfig.staked, "whether this node is a staked access node or not")
		flags.StringSliceVar(&anb.bootstrapNodeAddresses, "bootstrap-node-addresses", defaultConfig.bootstrapNodeAddresses, "the network addresses of the bootstrap access node if this is an unstaked access node e.g. access-001.mainnet.flow.org:9653,access-002.mainnet.flow.org:9653")
		flags.StringSliceVar(&anb.bootstrapNodePublicKeys, "bootstrap-node-public-keys", defaultConfig.bootstrapNodePublicKeys, "the networking public key of the bootstrap access node if this is an unstaked access node (in the same order as the bootstrap node addresses) e.g. \"d57a5e9c5.....\",\"44ded42d....\"")
		flags.StringVar(&anb.unstakedNetworkBindAddr, "unstaked-bind-addr", defaultConfig.unstakedNetworkBindAddr, "address to bind on for the unstaked network")
	})

	// parse all the command line args
	anb.parseFlags()

	// choose a staked or an unstaked node builder based on anb.staked
	var nodeBuilder AccessNodeBuilder
	if anb.staked {
		nodeBuilder = NewStakedAccessNodeBuilder(anb)
	} else {
		nodeBuilder = NewUnstakedAccessNodeBuilder(anb)
	}

	nodeBuilder.
		Initialize().
		Module("mutable follower state", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currently supported but read-only state has type %T", node.State)
			}
			var err error
			anb.FollowerState, err = badgerState.NewFollowerState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
				blocktimer.DefaultBlockTimer,
			)
			return err
		}).
		Module("collection node client", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// collection node address is optional (if not specified, collection nodes will be chosen at random)
			if strings.TrimSpace(anb.rpcConf.CollectionAddr) == "" {
				node.Logger.Info().Msg("using a dynamic collection node address")
				return nil
			}

			node.Logger.Info().
				Str("collection_node", anb.rpcConf.CollectionAddr).
				Msg("using the static collection node address")

			collectionRPCConn, err := grpc.Dial(
				anb.rpcConf.CollectionAddr,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure(),
				backend.WithClientUnaryInterceptor(anb.rpcConf.CollectionClientTimeout))
			if err != nil {
				return err
			}
			anb.CollectionRPC = access.NewAccessAPIClient(collectionRPCConn)
			return nil
		}).
		Module("historical access node clients", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			addrs := strings.Split(anb.rpcConf.HistoricalAccessAddrs, ",")
			for _, addr := range addrs {
				if strings.TrimSpace(addr) == "" {
					continue
				}
				node.Logger.Info().Str("access_nodes", addr).Msg("historical access node addresses")

				historicalAccessRPCConn, err := grpc.Dial(
					addr,
					grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
					grpc.WithInsecure())
				if err != nil {
					return err
				}
				anb.HistoricalAccessRPCs = append(anb.HistoricalAccessRPCs, access.NewAccessAPIClient(historicalAccessRPCConn))
			}
			return nil
		}).
		Module("block cache", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			anb.ConCache = buffer.NewPendingBlocks()
			return nil
		}).
		Module("sync core", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			var err error
			anb.SyncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Module("transaction timing mempools", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			var err error
			anb.TransactionTimings, err = stdmap.NewTransactionTimings(1500 * 300) // assume 1500 TPS * 300 seconds
			if err != nil {
				return err
			}

			anb.CollectionsToMarkFinalized, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			anb.CollectionsToMarkExecuted, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			anb.BlocksToMarkExecuted, err = stdmap.NewTimes(1 * 300) // assume 1 block per second * 300 seconds
			return err
		}).
		Module("transaction metrics", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			anb.TransactionMetrics = metrics.NewTransactionCollector(anb.TransactionTimings, node.Logger, anb.logTxTimeToFinalized,
				anb.logTxTimeToExecuted, anb.logTxTimeToFinalizedExecuted)
			return nil
		}).
		Module("ping metrics", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			anb.PingMetrics = metrics.NewPingCollector()
			return nil
		}).
		Module("server certificate", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// generate the server certificate that will be served by the GRPC server
			x509Certificate, err := grpcutils.X509Certificate(node.NetworkKey)
			if err != nil {
				return err
			}
			tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
			anb.rpcConf.TransportCredentials = credentials.NewTLS(tlsConfig)
			return nil
		}).
		Component("RPC engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			anb.RpcEng = rpc.New(
				node.Logger,
				node.State,
				anb.rpcConf,
				anb.CollectionRPC,
				anb.HistoricalAccessRPCs,
				node.Storage.Blocks,
				node.Storage.Headers,
				node.Storage.Collections,
				node.Storage.Transactions,
				node.Storage.Receipts,
				node.RootChainID,
				anb.TransactionMetrics,
				anb.collectionGRPCPort,
				anb.executionGRPCPort,
				anb.retryEnabled,
				anb.rpcMetricsEnabled,
				anb.apiRatelimits,
				anb.apiBurstlimits,
			)
			return anb.RpcEng, nil
		}).
		Component("ingestion engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var err error
			anb.RequestEng, err = requester.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				engine.RequestCollections,
				filter.HasRole(flow.RoleCollection),
				func() flow.Entity { return &flow.Collection{} },
			)
			if err != nil {
				return nil, fmt.Errorf("could not create requester engine: %w", err)
			}
			anb.IngestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Me, anb.RequestEng, node.Storage.Blocks, node.Storage.Headers, node.Storage.Collections, node.Storage.Transactions, node.Storage.Receipts, anb.TransactionMetrics,
				anb.CollectionsToMarkFinalized, anb.CollectionsToMarkExecuted, anb.BlocksToMarkExecuted, anb.RpcEng)
			anb.RequestEng.WithHandle(anb.IngestEng.OnCollection)
			return anb.IngestEng, err
		}).
		Component("requester engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
			// order for it to properly start and shut down, we should still return it as its own engine here, so it can
			// be handled by the scaffold.
			return anb.RequestEng, nil
		}).
		Component("follower engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			cleaner := storage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)

			// create a finalizer that will handle updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, anb.FollowerState)

			// initialize the staking & beacon verifiers, signature joiner
			staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
			beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
			merger := signature.NewCombiner(encodable.ConsensusVoteSigLen, encodable.RandomBeaconSigLen)

			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			committee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(committee, staking, beacon, merger)

			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			anb.FinalizationDistributor = pubsub.NewFinalizationDistributor()
			anb.FinalizationDistributor.AddConsumer(anb.IngestEng)

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			followerCore, err := consensus.NewFollower(node.Logger, committee, node.Storage.Headers, final, verifier,
				anb.FinalizationDistributor, node.RootBlock.Header, node.RootQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not initialize follower core: %w", err)
			}

			anb.FollowerEng, err = followereng.New(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				anb.FollowerState,
				anb.ConCache,
				followerCore,
				anb.SyncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			return anb.FollowerEng, nil
		}).
		Component("finalized snapshot", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			finalizedHeader, err := synceng.NewFinalizedHeaderCache(node.Logger, node.State, anb.FinalizationDistributor)
			if err != nil {
				return nil, fmt.Errorf("could not create finalized snapshot cache: %w", err)
			}
			anb.FinalizedHeader = finalizedHeader

			return anb.FinalizedHeader, nil
		}).
		Component("sync engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			sync, err := synceng.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.Storage.Blocks,
				anb.FollowerEng,
				anb.SyncCore,
				anb.FinalizedHeader,
				node.State,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			return sync, nil
		})

	// the ping engine is only needed for the staked access node
	if nodeBuilder.IsStaked() {
		nodeBuilder.Component("ping engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			ping, err := pingeng.New(
				node.Logger,
				node.State,
				node.Me,
				anb.PingMetrics,
				anb.pingEnabled,
				node.Middleware,
				anb.nodeInfoFile,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create ping engine: %w", err)
			}
			return ping, nil
		})
	}

	nodeBuilder.Run()
}
