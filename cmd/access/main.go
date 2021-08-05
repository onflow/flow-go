package main

import (
	"fmt"
	"strings"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/ingestion"
	pingeng "github.com/onflow/flow-go/engine/access/ping"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	grpcutils "github.com/onflow/flow-go/utils/grpc"
)

func main() {
	anb := FlowAccessNode() // use the generic Access Node builder till it is determined if this is a staked AN or an unstaked AN

	anb.PrintBuildVersionDetails()

	anb.ExtraFlags(func(flags *pflag.FlagSet) {
		flags.UintVar(&anb.receiptLimit, "receipt-limit", anb.AccessNodeConfig.receiptLimit, "maximum number of execution receipts in the memory pool")
		flags.UintVar(&anb.collectionLimit, "collection-limit", anb.AccessNodeConfig.collectionLimit, "maximum number of collections in the memory pool")
		flags.UintVar(&anb.blockLimit, "block-limit", anb.AccessNodeConfig.blockLimit, "maximum number of result blocks in the memory pool")
		flags.UintVar(&anb.collectionGRPCPort, "collection-ingress-port", anb.AccessNodeConfig.collectionGRPCPort, "the grpc ingress port for all collection nodes")
		flags.UintVar(&anb.executionGRPCPort, "execution-ingress-port", anb.AccessNodeConfig.executionGRPCPort, "the grpc ingress port for all execution nodes")
		flags.StringVarP(&anb.rpcConf.UnsecureGRPCListenAddr, "rpc-addr", "r", anb.AccessNodeConfig.rpcConf.UnsecureGRPCListenAddr, "the address the unsecured gRPC server listens on")
		flags.StringVar(&anb.rpcConf.SecureGRPCListenAddr, "secure-rpc-addr", anb.AccessNodeConfig.rpcConf.SecureGRPCListenAddr, "the address the secure gRPC server listens on")
		flags.StringVarP(&anb.rpcConf.HTTPListenAddr, "http-addr", "h", anb.AccessNodeConfig.rpcConf.HTTPListenAddr, "the address the http proxy server listens on")
		flags.StringVarP(&anb.rpcConf.CollectionAddr, "static-collection-ingress-addr", "", anb.AccessNodeConfig.rpcConf.CollectionAddr, "the address (of the collection node) to send transactions to")
		flags.StringVarP(&anb.ExecutionNodeAddress, "script-addr", "s", anb.AccessNodeConfig.ExecutionNodeAddress, "the address (of the execution node) forward the script to")
		flags.StringVarP(&anb.rpcConf.HistoricalAccessAddrs, "historical-access-addr", "", anb.AccessNodeConfig.rpcConf.HistoricalAccessAddrs, "comma separated rpc addresses for historical access nodes")
		flags.DurationVar(&anb.rpcConf.CollectionClientTimeout, "collection-client-timeout", anb.AccessNodeConfig.rpcConf.CollectionClientTimeout, "grpc client timeout for a collection node")
		flags.DurationVar(&anb.rpcConf.ExecutionClientTimeout, "execution-client-timeout", anb.AccessNodeConfig.rpcConf.ExecutionClientTimeout, "grpc client timeout for an execution node")
		flags.UintVar(&anb.rpcConf.MaxHeightRange, "rpc-max-height-range", anb.AccessNodeConfig.rpcConf.MaxHeightRange, "maximum size for height range requests")
		flags.StringSliceVar(&anb.rpcConf.PreferredExecutionNodeIDs, "preferred-execution-node-ids", anb.AccessNodeConfig.rpcConf.PreferredExecutionNodeIDs, "comma separated list of execution nodes ids to choose from when making an upstream call e.g. b4a4dbdcd443d...,fb386a6a... etc.")
		flags.StringSliceVar(&anb.rpcConf.FixedExecutionNodeIDs, "fixed-execution-node-ids", anb.AccessNodeConfig.rpcConf.FixedExecutionNodeIDs, "comma separated list of execution nodes ids to choose from when making an upstream call if no matching preferred execution id is found e.g. b4a4dbdcd443d...,fb386a6a... etc.")
		flags.BoolVar(&anb.logTxTimeToFinalized, "log-tx-time-to-finalized", anb.AccessNodeConfig.logTxTimeToFinalized, "log transaction time to finalized")
		flags.BoolVar(&anb.logTxTimeToExecuted, "log-tx-time-to-executed", anb.AccessNodeConfig.logTxTimeToExecuted, "log transaction time to executed")
		flags.BoolVar(&anb.logTxTimeToFinalizedExecuted, "log-tx-time-to-finalized-executed", anb.AccessNodeConfig.logTxTimeToFinalizedExecuted, "log transaction time to finalized and executed")
		flags.BoolVar(&anb.pingEnabled, "ping-enabled", anb.AccessNodeConfig.pingEnabled, "whether to enable the ping process that pings all other peers and report the connectivity to metrics")
		flags.BoolVar(&anb.retryEnabled, "retry-enabled", anb.AccessNodeConfig.retryEnabled, "whether to enable the retry mechanism at the access node level")
		flags.BoolVar(&anb.rpcMetricsEnabled, "rpc-metrics-enabled", anb.AccessNodeConfig.rpcMetricsEnabled, "whether to enable the rpc metrics")
		flags.StringVarP(&anb.nodeInfoFile, "node-info-file", "", anb.AccessNodeConfig.nodeInfoFile, "full path to a json file which provides more details about nodes when reporting its reachability metrics")
		flags.StringToIntVar(&anb.apiRatelimits, "api-rate-limits", anb.AccessNodeConfig.apiRatelimits, "per second rate limits for Access API methods e.g. Ping=300,GetTransaction=500 etc.")
		flags.StringToIntVar(&anb.apiBurstlimits, "api-burst-limits", anb.AccessNodeConfig.apiBurstlimits, "burst limits for Access API methods e.g. Ping=100,GetTransaction=100 etc.")
		flags.BoolVar(&anb.staked, "staked", anb.AccessNodeConfig.staked, "whether this node is a staked access node or not")
		flags.StringVar(&anb.stakedAccessNodeIDHex, "staked-access-node-id", anb.AccessNodeConfig.stakedAccessNodeIDHex, "the node ID of the upstream staked access node if this is an unstaked access node")
		flags.StringVar(&anb.unstakedNetworkBindAddr, "unstaked-bind-addr", anb.AccessNodeConfig.unstakedNetworkBindAddr, "address to bind on for the unstaked network")
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
		Component("requester engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
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

			return anb.RequestEng, nil
		}).
		Component("ingestion engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var err error
			anb.IngestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Me, anb.RequestEng, node.Storage.Blocks, node.Storage.Headers, node.Storage.Collections, node.Storage.Transactions, node.Storage.Receipts, anb.TransactionMetrics,
				anb.CollectionsToMarkFinalized, anb.CollectionsToMarkExecuted, anb.BlocksToMarkExecuted, anb.RpcEng)
			anb.RequestEng.WithHandle(anb.IngestEng.OnCollection)
			anb.FinalizationDistributor.AddConsumer(anb.IngestEng)

			return anb.IngestEng, err
		})

	anb.BuildConsensusFollower()

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
