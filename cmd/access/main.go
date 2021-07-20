package main

import (
	"fmt"
	"strings"
	"time"

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
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	storage "github.com/onflow/flow-go/storage/badger"
	grpcutils "github.com/onflow/flow-go/utils/grpc"
)

func main() {

	var (
		blockLimit                   uint
		collectionLimit              uint
		receiptLimit                 uint
		collectionGRPCPort           uint
		executionGRPCPort            uint
		pingEnabled                  bool
		nodeInfoFile                 string
		apiRatelimits                map[string]int
		apiBurstlimits               map[string]int
		followerState                protocol.MutableState
		ingestEng                    *ingestion.Engine
		requestEng                   *requester.Engine
		followerEng                  *followereng.Engine
		syncCore                     *synchronization.Core
		rpcConf                      rpc.Config
		rpcEng                       *rpc.Engine
		finalizationDistributor      *pubsub.FinalizationDistributor
		collectionRPC                access.AccessAPIClient
		executionNodeAddress         string // deprecated
		historicalAccessRPCs         []access.AccessAPIClient
		err                          error
		conCache                     *buffer.PendingBlocks // pending block cache for follower
		transactionTimings           *stdmap.TransactionTimings
		collectionsToMarkFinalized   *stdmap.Times
		collectionsToMarkExecuted    *stdmap.Times
		blocksToMarkExecuted         *stdmap.Times
		transactionMetrics           module.TransactionMetrics
		pingMetrics                  module.PingMetrics
		logTxTimeToFinalized         bool
		logTxTimeToExecuted          bool
		logTxTimeToFinalizedExecuted bool
		retryEnabled                 bool
		rpcMetricsEnabled            bool
	)

	cmd.FlowNode(flow.RoleAccess.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&receiptLimit, "receipt-limit", 1000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&collectionLimit, "collection-limit", 1000, "maximum number of collections in the memory pool")
			flags.UintVar(&blockLimit, "block-limit", 1000, "maximum number of result blocks in the memory pool")
			flags.UintVar(&collectionGRPCPort, "collection-ingress-port", 9000, "the grpc ingress port for all collection nodes")
			flags.UintVar(&executionGRPCPort, "execution-ingress-port", 9000, "the grpc ingress port for all execution nodes")
			flags.StringVarP(&rpcConf.UnsecureGRPCListenAddr, "rpc-addr", "r", "localhost:9000", "the address the unsecured gRPC server listens on")
			flags.StringVar(&rpcConf.SecureGRPCListenAddr, "secure-rpc-addr", "localhost:9001", "the address the secure gRPC server listens on")
			flags.StringVarP(&rpcConf.HTTPListenAddr, "http-addr", "h", "localhost:8000", "the address the http proxy server listens on")
			flags.StringVarP(&rpcConf.CollectionAddr, "static-collection-ingress-addr", "", "", "the address (of the collection node) to send transactions to")
			flags.StringVarP(&executionNodeAddress, "script-addr", "s", "localhost:9000", "the address (of the execution node) forward the script to")
			flags.StringVarP(&rpcConf.HistoricalAccessAddrs, "historical-access-addr", "", "", "comma separated rpc addresses for historical access nodes")
			flags.DurationVar(&rpcConf.CollectionClientTimeout, "collection-client-timeout", 3*time.Second, "grpc client timeout for a collection node")
			flags.DurationVar(&rpcConf.ExecutionClientTimeout, "execution-client-timeout", 3*time.Second, "grpc client timeout for an execution node")
			flags.UintVar(&rpcConf.MaxHeightRange, "rpc-max-height-range", backend.DefaultMaxHeightRange, "maximum size for height range requests")
			flags.StringSliceVar(&rpcConf.PreferredExecutionNodeIDs, "preferred-execution-node-ids", nil, "comma separated list of execution nodes ids to choose from when making an upstream call e.g. b4a4dbdcd443d...,fb386a6a... etc.")
			flags.StringSliceVar(&rpcConf.FixedExecutionNodeIDs, "fixed-execution-node-ids", nil, "comma separated list of execution nodes ids to choose from when making an upstream call if no matching preferred execution id is found e.g. b4a4dbdcd443d...,fb386a6a... etc.")
			flags.BoolVar(&logTxTimeToFinalized, "log-tx-time-to-finalized", false, "log transaction time to finalized")
			flags.BoolVar(&logTxTimeToExecuted, "log-tx-time-to-executed", false, "log transaction time to executed")
			flags.BoolVar(&logTxTimeToFinalizedExecuted, "log-tx-time-to-finalized-executed", false, "log transaction time to finalized and executed")
			flags.BoolVar(&pingEnabled, "ping-enabled", false, "whether to enable the ping process that pings all other peers and report the connectivity to metrics")
			flags.BoolVar(&retryEnabled, "retry-enabled", false, "whether to enable the retry mechanism at the access node level")
			flags.BoolVar(&rpcMetricsEnabled, "rpc-metrics-enabled", false, "whether to enable the rpc metrics")
			flags.StringVarP(&nodeInfoFile, "node-info-file", "", "", "full path to a json file which provides more details about nodes when reporting its reachability metrics")
			flags.StringToIntVar(&apiRatelimits, "api-rate-limits", nil, "per second rate limits for Access API methods e.g. Ping=300,GetTransaction=500 etc.")
			flags.StringToIntVar(&apiBurstlimits, "api-burst-limits", nil, "burst limits for Access API methods e.g. Ping=100,GetTransaction=100 etc.")
		}).
		Module("mutable follower state", func(node *cmd.FlowNodeBuilder) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currenlty supported but read-only state has type %T", node.State)
			}
			followerState, err = badgerState.NewFollowerState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
				blocktimer.DefaultBlockTimer,
			)
			return err
		}).
		Module("collection node client", func(node *cmd.FlowNodeBuilder) error {
			// collection node address is optional (if not specified, collection nodes will be chosen at random)
			if strings.TrimSpace(rpcConf.CollectionAddr) == "" {
				node.Logger.Info().Msg("using a dynamic collection node address")
				return nil
			}

			node.Logger.Info().
				Str("collection_node", rpcConf.CollectionAddr).
				Msg("using the static collection node address")

			collectionRPCConn, err := grpc.Dial(
				rpcConf.CollectionAddr,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure(),
				backend.WithClientUnaryInterceptor(rpcConf.CollectionClientTimeout))
			if err != nil {
				return err
			}
			collectionRPC = access.NewAccessAPIClient(collectionRPCConn)
			return nil
		}).
		Module("historical access node clients", func(node *cmd.FlowNodeBuilder) error {
			addrs := strings.Split(rpcConf.HistoricalAccessAddrs, ",")
			for _, addr := range addrs {
				if strings.TrimSpace(addr) == "" {
					continue
				}
				node.Logger.Info().Err(err).Msgf("Historical access node Addr: %s", addr)

				historicalAccessRPCConn, err := grpc.Dial(
					addr,
					grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
					grpc.WithInsecure())
				if err != nil {
					return err
				}
				historicalAccessRPCs = append(historicalAccessRPCs, access.NewAccessAPIClient(historicalAccessRPCConn))
			}
			return nil
		}).
		Module("block cache", func(node *cmd.FlowNodeBuilder) error {
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		Module("sync core", func(node *cmd.FlowNodeBuilder) error {
			syncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Module("transaction timing mempools", func(node *cmd.FlowNodeBuilder) error {
			transactionTimings, err = stdmap.NewTransactionTimings(1500 * 300) // assume 1500 TPS * 300 seconds
			if err != nil {
				return err
			}

			collectionsToMarkFinalized, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			collectionsToMarkExecuted, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			blocksToMarkExecuted, err = stdmap.NewTimes(1 * 300) // assume 1 block per second * 300 seconds
			return err
		}).
		Module("transaction metrics", func(node *cmd.FlowNodeBuilder) error {
			transactionMetrics = metrics.NewTransactionCollector(transactionTimings, node.Logger, logTxTimeToFinalized,
				logTxTimeToExecuted, logTxTimeToFinalizedExecuted)
			return nil
		}).
		Module("ping metrics", func(node *cmd.FlowNodeBuilder) error {
			pingMetrics = metrics.NewPingCollector()
			return nil
		}).
		Module("server certificate", func(node *cmd.FlowNodeBuilder) error {
			// generate the server certificate that will be served by the GRPC server
			x509Certificate, err := grpcutils.X509Certificate(node.NetworkKey)
			if err != nil {
				return err
			}
			tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
			rpcConf.TransportCredentials = credentials.NewTLS(tlsConfig)
			return nil
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			rpcEng = rpc.New(
				node.Logger,
				node.State,
				rpcConf,
				collectionRPC,
				historicalAccessRPCs,
				node.Storage.Blocks,
				node.Storage.Headers,
				node.Storage.Collections,
				node.Storage.Transactions,
				node.Storage.Receipts,
				node.RootChainID,
				transactionMetrics,
				collectionGRPCPort,
				executionGRPCPort,
				retryEnabled,
				rpcMetricsEnabled,
				apiRatelimits,
				apiBurstlimits,
			)
			return rpcEng, nil
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			requestEng, err = requester.New(
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
			ingestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Me, requestEng, node.Storage.Blocks, node.Storage.Headers, node.Storage.Collections, node.Storage.Transactions, node.Storage.Receipts, transactionMetrics,
				collectionsToMarkFinalized, collectionsToMarkExecuted, blocksToMarkExecuted, rpcEng)
			requestEng.WithHandle(ingestEng.OnCollection)
			return ingestEng, err
		}).
		Component("requester engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
			// order for it to properly start and shut down, we should still return it as its own engine here, so it can
			// be handled by the scaffold.
			return requestEng, nil
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			cleaner := storage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)

			// create a finalizer that will handle updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, followerState)

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

			finalizationDistributor = pubsub.NewFinalizationDistributor()
			finalizationDistributor.AddConsumer(ingestEng)

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			followerCore, err := consensus.NewFollower(node.Logger, committee, node.Storage.Headers, final, verifier,
				finalizationDistributor, node.RootBlock.Header, node.RootQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not initialize follower core: %w", err)
			}

			followerEng, err = followereng.New(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				followerState,
				conCache,
				followerCore,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			return followerEng, nil
		}).
		Component("sync engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			sync, err := synceng.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				followerEng,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			finalizationDistributor.AddOnBlockFinalizedConsumer(sync.OnFinalizedBlock)

			return sync, nil
		}).
		Component("ping engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ping, err := pingeng.New(
				node.Logger,
				node.State,
				node.Me,
				pingMetrics,
				pingEnabled,
				node.Middleware,
				nodeInfoFile,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create ping engine: %w", err)
			}
			return ping, nil
		}).
		Run()
}
