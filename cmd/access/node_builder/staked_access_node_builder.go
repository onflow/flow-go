package node_builder

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/ingestion"
	pingeng "github.com/onflow/flow-go/engine/access/ping"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/metrics/unstaked"
	"github.com/onflow/flow-go/module/state_synchronization"
	edrequester "github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast"
	relaynet "github.com/onflow/flow-go/network/relay"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/utils/grpcutils"
)

// StakedAccessNodeBuilder builds a staked access node. The staked access node can optionally participate in the
// unstaked network publishing data for the unstaked access node downstream.
type StakedAccessNodeBuilder struct {
	*FlowAccessNodeBuilder
}

func NewStakedAccessNodeBuilder(builder *FlowAccessNodeBuilder) *StakedAccessNodeBuilder {
	return &StakedAccessNodeBuilder{
		FlowAccessNodeBuilder: builder,
	}
}

func (builder *StakedAccessNodeBuilder) InitIDProviders() {
	builder.Module("id providers", func(node *cmd.NodeConfig) error {
		idCache, err := p2p.NewProtocolStateIDCache(node.Logger, node.State, node.ProtocolEvents)
		if err != nil {
			return err
		}

		builder.IdentityProvider = idCache

		builder.SyncEngineParticipantsProviderFactory = func() id.IdentifierProvider {
			return id.NewIdentityFilterIdentifierProvider(
				filter.And(
					filter.HasRole(flow.RoleConsensus),
					filter.Not(filter.HasNodeID(node.Me.NodeID())),
					p2p.NotEjectedFilter,
				),
				idCache,
			)
		}

		builder.IDTranslator = p2p.NewHierarchicalIDTranslator(idCache, p2p.NewUnstakedNetworkIDTranslator())

		return nil
	})
}

func (builder *StakedAccessNodeBuilder) Initialize() error {
	builder.InitIDProviders()

	builder.EnqueueResolver()

	// enqueue the regular network
	builder.EnqueueNetworkInit()

	// if this is an access node that supports unstaked followers, enqueue the unstaked network
	if builder.supportsUnstakedFollower {
		builder.enqueueUnstakedNetworkInit()
		builder.enqueueRelayNetwork()
	}

	builder.EnqueuePingService()

	builder.EnqueueMetricsServerInit()

	if err := builder.RegisterBadgerMetrics(); err != nil {
		return err
	}

	builder.EnqueueAdminServerInit()

	builder.EnqueueTracer()
	builder.PreInit(cmd.DynamicStartPreInit)
	return nil
}

func (builder *StakedAccessNodeBuilder) enqueueRelayNetwork() {
	builder.Component("relay network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		relayNet := relaynet.NewRelayNetwork(
			node.Network,
			builder.AccessNodeConfig.PublicNetworkConfig.Network,
			node.Logger,
			[]network.Channel{engine.ReceiveBlocks},
		)
		node.Network = relayNet
		return relayNet, nil
	})
}

func (builder *StakedAccessNodeBuilder) Build() (cmd.Node, error) {

	builder.
		BuildConsensusFollower().
		Module("collection node client", func(node *cmd.NodeConfig) error {
			// collection node address is optional (if not specified, collection nodes will be chosen at random)
			if strings.TrimSpace(builder.rpcConf.CollectionAddr) == "" {
				node.Logger.Info().Msg("using a dynamic collection node address")
				return nil
			}

			node.Logger.Info().
				Str("collection_node", builder.rpcConf.CollectionAddr).
				Msg("using the static collection node address")

			collectionRPCConn, err := grpc.Dial(
				builder.rpcConf.CollectionAddr,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure(), //nolint:staticcheck
				backend.WithClientUnaryInterceptor(builder.rpcConf.CollectionClientTimeout))
			if err != nil {
				return err
			}
			builder.CollectionRPC = access.NewAccessAPIClient(collectionRPCConn)
			return nil
		}).
		Module("historical access node clients", func(node *cmd.NodeConfig) error {
			addrs := strings.Split(builder.rpcConf.HistoricalAccessAddrs, ",")
			for _, addr := range addrs {
				if strings.TrimSpace(addr) == "" {
					continue
				}
				node.Logger.Info().Str("access_nodes", addr).Msg("historical access node addresses")

				historicalAccessRPCConn, err := grpc.Dial(
					addr,
					grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
					grpc.WithInsecure()) //nolint:staticcheck
				if err != nil {
					return err
				}
				builder.HistoricalAccessRPCs = append(builder.HistoricalAccessRPCs, access.NewAccessAPIClient(historicalAccessRPCConn))
			}
			return nil
		}).
		Module("transaction timing mempools", func(node *cmd.NodeConfig) error {
			var err error
			builder.TransactionTimings, err = stdmap.NewTransactionTimings(1500 * 300) // assume 1500 TPS * 300 seconds
			if err != nil {
				return err
			}

			builder.CollectionsToMarkFinalized, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			builder.CollectionsToMarkExecuted, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			builder.BlocksToMarkExecuted, err = stdmap.NewTimes(1 * 300) // assume 1 block per second * 300 seconds
			return err
		}).
		Module("transaction metrics", func(node *cmd.NodeConfig) error {
			builder.TransactionMetrics = metrics.NewTransactionCollector(builder.TransactionTimings, node.Logger, builder.logTxTimeToFinalized,
				builder.logTxTimeToExecuted, builder.logTxTimeToFinalizedExecuted)
			return nil
		}).
		Module("ping metrics", func(node *cmd.NodeConfig) error {
			builder.PingMetrics = metrics.NewPingCollector()
			return nil
		}).
		Module("server certificate", func(node *cmd.NodeConfig) error {
			// generate the server certificate that will be served by the GRPC server
			x509Certificate, err := grpcutils.X509Certificate(node.NetworkKey)
			if err != nil {
				return err
			}
			tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
			builder.rpcConf.TransportCredentials = credentials.NewTLS(tlsConfig)
			return nil
		}).
		Component("RPC engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			builder.RpcEng = rpc.New(
				node.Logger,
				node.State,
				builder.rpcConf,
				builder.CollectionRPC,
				builder.HistoricalAccessRPCs,
				node.Storage.Blocks,
				node.Storage.Headers,
				node.Storage.Collections,
				node.Storage.Transactions,
				node.Storage.Receipts,
				node.Storage.Results,
				node.RootChainID,
				builder.TransactionMetrics,
				builder.collectionGRPCPort,
				builder.executionGRPCPort,
				builder.retryEnabled,
				builder.rpcMetricsEnabled,
				builder.apiRatelimits,
				builder.apiBurstlimits,
			)
			return builder.RpcEng, nil
		}).
		Component("ingestion engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var err error

			builder.RequestEng, err = requester.New(
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

			builder.IngestEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.State,
				node.Me,
				builder.RequestEng,
				node.Storage.Blocks,
				node.Storage.Headers,
				node.Storage.Collections,
				node.Storage.Transactions,
				node.Storage.Results,
				node.Storage.Receipts,
				builder.TransactionMetrics,
				builder.CollectionsToMarkFinalized,
				builder.CollectionsToMarkExecuted,
				builder.BlocksToMarkExecuted,
				builder.RpcEng,
			)
			if err != nil {
				return nil, err
			}
			builder.RequestEng.WithHandle(builder.IngestEng.OnCollection)
			builder.FinalizationDistributor.AddConsumer(builder.IngestEng)

			return builder.IngestEng, nil
		}).
		Component("requester engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
			// order for it to properly start and shut down, we should still return it as its own engine here, so it can
			// be handled by the scaffold.
			return builder.RequestEng, nil
		})

	if builder.executionDataSyncEnabled {
		var dstore datastore.Batching
		var blobservice network.BlobService

		builder.
			Component("execution data service", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
				err := os.MkdirAll(builder.executionDataDir, 0700)
				if err != nil {
					return nil, err
				}

				// TODO: we need to close this after the execution data services are stopped. it should probably be run as part of the node's postShutdown handler
				dstore, err := badger.NewDatastore(builder.executionDataDir, &badger.DefaultOptions)
				if err != nil {
					return nil, err
				}

				builder.ShutdownFunc(func() error {
					if err := dstore.Close(); err != nil {
						return fmt.Errorf("could not close execution data datastore: %w", err)
					}
					return nil
				})

				blobservice, err := node.Network.RegisterBlobService(engine.ExecutionDataService, dstore)
				if err != nil {
					return nil, err
				}

				eds := state_synchronization.NewExecutionDataService(
					new(cbor.Codec),
					compressor.NewLz4Compressor(),
					blobservice,
					metrics.NewExecutionDataServiceCollector(),
					builder.Logger,
				)

				builder.ExecutionDataService = eds

				return builder.ExecutionDataService, nil
			}).
			Component("execution data requester", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
				edr, err := edrequester.NewExecutionDataRequester(
					builder.Logger,
					metrics.NewNoopCollector(),
					dstore,
					blobservice,
					builder.ExecutionDataService,
					builder.RootBlock,
					builder.Storage.Blocks,
					builder.Storage.Results,
					builder.executionDataCheckEnabled,
				)

				if err != nil {
					return nil, fmt.Errorf("could not create execution data requester: %w", err)
				}

				builder.FinalizationDistributor.AddOnBlockFinalizedConsumer(edr.OnBlockFinalized)

				builder.ExecutionDataRequester = edr

				return builder.ExecutionDataRequester, nil
			})
	}

	if builder.supportsUnstakedFollower {
		builder.Component("unstaked sync request handler", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			syncRequestHandler, err := synceng.NewRequestHandlerEngine(
				node.Logger.With().Bool("unstaked", true).Logger(),
				unstaked.NewUnstakedEngineCollector(node.Metrics.Engine),
				builder.AccessNodeConfig.PublicNetworkConfig.Network,
				node.Me,
				node.Storage.Blocks,
				builder.SyncCore,
				builder.FinalizedHeader,
			)

			if err != nil {
				return nil, fmt.Errorf("could not create unstaked sync request handler: %w", err)
			}

			return syncRequestHandler, nil
		})
	}

	builder.Component("ping engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		ping, err := pingeng.New(
			node.Logger,
			node.IdentityProvider,
			node.IDTranslator,
			node.Me,
			builder.PingMetrics,
			builder.pingEnabled,
			builder.nodeInfoFile,
			node.PingService,
		)

		if err != nil {
			return nil, fmt.Errorf("could not create ping engine: %w", err)
		}

		return ping, nil
	})

	return builder.FlowAccessNodeBuilder.Build()
}

// enqueueUnstakedNetworkInit enqueues the unstaked network component initialized for the staked node
func (builder *StakedAccessNodeBuilder) enqueueUnstakedNetworkInit() {
	builder.Component("unstaked network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		builder.PublicNetworkConfig.Metrics = metrics.NewNetworkCollector(metrics.WithNetworkPrefix("unstaked"))

		libP2PFactory := builder.initLibP2PFactory(builder.NodeConfig.NetworkKey)

		msgValidators := unstakedNetworkMsgValidators(node.Logger.With().Bool("unstaked", true).Logger(), node.IdentityProvider, builder.NodeID)

		middleware := builder.initMiddleware(builder.NodeID, builder.PublicNetworkConfig.Metrics, libP2PFactory, msgValidators...)

		// topology returns empty list since peers are not known upfront
		top := topology.EmptyListTopology{}

		network, err := builder.initNetwork(builder.Me, builder.PublicNetworkConfig.Metrics, middleware, top)
		if err != nil {
			return nil, err
		}

		builder.AccessNodeConfig.PublicNetworkConfig.Network = network

		node.Logger.Info().Msgf("network will run on address: %s", builder.PublicNetworkConfig.BindAddress)
		return network, nil
	})
}

// initLibP2PFactory creates the LibP2P factory function for the given node ID and network key.
// The factory function is later passed into the initMiddleware function to eventually instantiate the p2p.LibP2PNode instance
// The LibP2P host is created with the following options:
// 		DHT as server
// 		The address from the node config or the specified bind address as the listen address
// 		The passed in private key as the libp2p key
//		No connection gater
// 		Default Flow libp2p pubsub options
func (builder *StakedAccessNodeBuilder) initLibP2PFactory(networkKey crypto.PrivateKey) p2p.LibP2PFactoryFunc {
	return func(ctx context.Context) (*p2p.Node, error) {
		connManager := p2p.NewConnManager(builder.Logger, builder.PublicNetworkConfig.Metrics)

		libp2pNode, err := p2p.NewNodeBuilder(builder.Logger, builder.PublicNetworkConfig.BindAddress, networkKey, builder.SporkID).
			SetBasicResolver(builder.Resolver).
			SetSubscriptionFilter(
				p2p.NewRoleBasedFilter(
					flow.RoleAccess, builder.IdentityProvider,
				),
			).
			SetConnectionManager(connManager).
			SetRoutingSystem(func(ctx context.Context, h host.Host) (routing.Routing, error) {
				return p2p.NewDHT(ctx, h, unicast.FlowPublicDHTProtocolID(builder.SporkID), p2p.AsServer())
			}).
			SetPubSub(pubsub.NewGossipSub).
			Build(ctx)

		if err != nil {
			return nil, fmt.Errorf("could not build libp2p node for staked access node: %w", err)
		}

		builder.LibP2PNode = libp2pNode

		return builder.LibP2PNode, nil
	}
}

// initMiddleware creates the network.Middleware implementation with the libp2p factory function, metrics, peer update
// interval, and validators. The network.Middleware is then passed into the initNetwork function.
func (builder *StakedAccessNodeBuilder) initMiddleware(nodeID flow.Identifier,
	networkMetrics module.NetworkMetrics,
	factoryFunc p2p.LibP2PFactoryFunc,
	validators ...network.MessageValidator) network.Middleware {

	// disable connection pruning for the staked AN which supports the unstaked AN
	peerManagerFactory := p2p.PeerManagerFactory([]p2p.Option{p2p.WithInterval(builder.PeerUpdateInterval)}, p2p.WithConnectionPruning(false))

	builder.Middleware = p2p.NewMiddleware(
		builder.Logger.With().Bool("staked", false).Logger(),
		factoryFunc,
		nodeID,
		networkMetrics,
		builder.SporkID,
		p2p.DefaultUnicastTimeout,
		builder.IDTranslator,
		p2p.WithMessageValidators(validators...),
		p2p.WithPeerManager(peerManagerFactory),
		// use default identifier provider
	)

	return builder.Middleware
}
