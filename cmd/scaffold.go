package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/admin/commands/common"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	sutil "github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/debug"
	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/logging"
)

type Metrics struct {
	Network        module.NetworkMetrics
	Engine         module.EngineMetrics
	Compliance     module.ComplianceMetrics
	Cache          module.CacheMetrics
	Mempool        module.MempoolMetrics
	CleanCollector module.CleanerMetrics
}

type Storage struct {
	Headers      storage.Headers
	Index        storage.Index
	Identities   storage.Identities
	Guarantees   storage.Guarantees
	Receipts     *bstorage.ExecutionReceipts
	Results      storage.ExecutionResults
	Seals        storage.Seals
	Payloads     storage.Payloads
	Blocks       storage.Blocks
	Transactions storage.Transactions
	Collections  storage.Collections
	Setups       storage.EpochSetups
	Commits      storage.EpochCommits
	Statuses     storage.EpochStatuses
}

type BuilderFunc func(nodeConfig *NodeConfig) error
type ComponentBuilderFunc func(node *NodeConfig, lookup component.LookupFunc) (module.ReadyDoneAware, error)
type BackgroundComponentBuilderFunc func(node *NodeConfig, lookup component.LookupFunc) (component.Component, error)

type namedModuleFunc struct {
	fn   BuilderFunc
	name string
}

// FlowNodeBuilder is the default builder struct used for all flow nodes
// It runs a node process with following structure, in sequential order
// Base inits (network, storage, state, logger)
//   PostInit handlers, if any
// Components handlers, if any, wait sequentially
// Run() <- main loop
// Components destructors, if any
// The initialization can be proceeded and succeeded with  PreInit and PostInit functions that allow customization
// of the process in case of nodes such as the unstaked access node where the NodeInfo is not part of the genesis data
type FlowNodeBuilder struct {
	*NodeConfig
	flags                    *pflag.FlagSet
	modules                  []namedModuleFunc
	preInitFns               []BuilderFunc
	postInitFns              []BuilderFunc
	extraFlagCheck           func() error
	adminCommandBootstrapper *admin.CommandRunnerBootstrapper
	adminCommands            map[string]func(config *NodeConfig) commands.AdminCommand
	componentBuilder         component.ComponentManagerBuilder
}

func (fnb *FlowNodeBuilder) BaseFlags() {
	defaultConfig := DefaultBaseConfig()

	// bind configuration parameters
	fnb.flags.StringVar(&fnb.BaseConfig.nodeIDHex, "nodeid", defaultConfig.nodeIDHex, "identity of our node")
	fnb.flags.StringVar(&fnb.BaseConfig.BindAddr, "bind", defaultConfig.BindAddr, "address to bind on")
	fnb.flags.StringVarP(&fnb.BaseConfig.BootstrapDir, "bootstrapdir", "b", defaultConfig.BootstrapDir, "path to the bootstrap directory")
	fnb.flags.StringVarP(&fnb.BaseConfig.datadir, "datadir", "d", defaultConfig.datadir, "directory to store the public database (protocol state)")
	fnb.flags.StringVar(&fnb.BaseConfig.secretsdir, "secretsdir", defaultConfig.secretsdir, "directory to store private database (secrets)")
	fnb.flags.StringVarP(&fnb.BaseConfig.level, "loglevel", "l", defaultConfig.level, "level for logging output")
	fnb.flags.DurationVar(&fnb.BaseConfig.PeerUpdateInterval, "peerupdate-interval", defaultConfig.PeerUpdateInterval, "how often to refresh the peer connections for the node")
	fnb.flags.DurationVar(&fnb.BaseConfig.UnicastMessageTimeout, "unicast-timeout", defaultConfig.UnicastMessageTimeout, "how long a unicast transmission can take to complete")
	fnb.flags.UintVarP(&fnb.BaseConfig.metricsPort, "metricport", "m", defaultConfig.metricsPort, "port for /metrics endpoint")
	fnb.flags.BoolVar(&fnb.BaseConfig.profilerEnabled, "profiler-enabled", defaultConfig.profilerEnabled, "whether to enable the auto-profiler")
	fnb.flags.StringVar(&fnb.BaseConfig.profilerDir, "profiler-dir", defaultConfig.profilerDir, "directory to create auto-profiler profiles")
	fnb.flags.DurationVar(&fnb.BaseConfig.profilerInterval, "profiler-interval", defaultConfig.profilerInterval,
		"the interval between auto-profiler runs")
	fnb.flags.DurationVar(&fnb.BaseConfig.profilerDuration, "profiler-duration", defaultConfig.profilerDuration,
		"the duration to run the auto-profile for")
	fnb.flags.BoolVar(&fnb.BaseConfig.tracerEnabled, "tracer-enabled", defaultConfig.tracerEnabled,
		"whether to enable tracer")
	fnb.flags.UintVar(&fnb.BaseConfig.tracerSensitivity, "tracer-sensitivity", defaultConfig.tracerSensitivity,
		"adjusts the level of sampling when tracing is enabled. 0 means capture everything, higher value results in less samples")

	fnb.flags.StringVar(&fnb.BaseConfig.AdminAddr, "admin-addr", defaultConfig.AdminAddr, "address to bind on for admin HTTP server")
	fnb.flags.StringVar(&fnb.BaseConfig.AdminCert, "admin-cert", defaultConfig.AdminCert, "admin cert file (for TLS)")
	fnb.flags.StringVar(&fnb.BaseConfig.AdminKey, "admin-key", defaultConfig.AdminKey, "admin key file (for TLS)")
	fnb.flags.StringVar(&fnb.BaseConfig.AdminClientCAs, "admin-client-certs", defaultConfig.AdminClientCAs, "admin client certs (for mutual TLS)")

	fnb.flags.DurationVar(&fnb.BaseConfig.DNSCacheTTL, "dns-cache-ttl", defaultConfig.DNSCacheTTL, "time-to-live for dns cache")
	fnb.flags.StringVar(&fnb.BaseConfig.LibP2PStreamCompression, "stream-compression", p2p.NoCompression,
		"networking stream compression mechanism")
	fnb.flags.IntVar(&fnb.BaseConfig.NetworkReceivedMessageCacheSize, "networking-receive-cache-size", p2p.DefaultCacheSize,
		"incoming message cache size at networking layer")
	fnb.flags.UintVar(&fnb.BaseConfig.guaranteesCacheSize, "guarantees-cache-size", bstorage.DefaultCacheSize, "collection guarantees cache size")
	fnb.flags.UintVar(&fnb.BaseConfig.receiptsCacheSize, "receipts-cache-size", bstorage.DefaultCacheSize, "receipts cache size")
}

func (fnb *FlowNodeBuilder) EnqueueNetworkInit() {
	fnb.Component("network", func(node *NodeConfig, lookup component.LookupFunc) (module.ReadyDoneAware, error) {

		codec := cborcodec.NewCodec()

		myAddr := fnb.NodeConfig.Me.Address()
		if fnb.BaseConfig.BindAddr != NotSet {
			myAddr = fnb.BaseConfig.BindAddr
		}

		// setup the Ping provider to return the software version and the sealed block height
		pingProvider := p2p.PingInfoProviderImpl{
			SoftwareVersionFun: func() string {
				return build.Semver()
			},
			SealedBlockHeightFun: func() (uint64, error) {
				head, err := fnb.State.Sealed().Head()
				if err != nil {
					return 0, err
				}
				return head.Height, nil
			},
			HotstuffViewFun: nil, // set in next code block, depending on role
		}

		// only consensus roles will need to report hotstuff view
		if fnb.BaseConfig.NodeRole == flow.RoleConsensus.String() {
			// initialize the persister
			persist := persister.New(node.DB, node.RootChainID)

			pingProvider.HotstuffViewFun = func() (uint64, error) {
				curView, err := persist.GetStarted()
				if err != nil {
					return 0, err
				}

				return curView, nil
			}
		} else {
			// non-consensus will not report any hotstuff view
			pingProvider.HotstuffViewFun = func() (uint64, error) {
				return 0, fmt.Errorf("non-consensus nodes do not report hotstuff view in ping")
			}
		}

		streamFactory, err := p2p.LibP2PStreamCompressorFactoryFunc(fnb.BaseConfig.LibP2PStreamCompression)
		if err != nil {
			return nil, fmt.Errorf("could not convert stream factory: %w", err)
		}

		libP2PNodeFactory, err := p2p.DefaultLibP2PNodeFactory(
			fnb.Logger,
			fnb.Me.NodeID(),
			myAddr,
			fnb.NetworkKey,
			fnb.RootBlock.ID(),
			fnb.IdentityProvider,
			p2p.DefaultMaxPubSubMsgSize,
			fnb.Metrics.Network,
			pingProvider,
			fnb.BaseConfig.DNSCacheTTL,
			fnb.BaseConfig.NodeRole,
			streamFactory)

		if err != nil {
			return nil, fmt.Errorf("could not generate libp2p node factory: %w", err)
		}

		var mwOpts []p2p.MiddlewareOption
		if len(fnb.MsgValidators) > 0 {
			mwOpts = append(mwOpts, p2p.WithMessageValidators(fnb.MsgValidators...))
		}

		// run peer manager with the specified interval and let is also prune connections
		peerManagerFactory := p2p.PeerManagerFactory([]p2p.Option{p2p.WithInterval(fnb.PeerUpdateInterval)})
		mwOpts = append(mwOpts, p2p.WithPeerManager(peerManagerFactory))

		fnb.Middleware = p2p.NewMiddleware(
			fnb.Logger,
			libP2PNodeFactory,
			fnb.Me.NodeID(),
			fnb.Metrics.Network,
			fnb.RootBlock.ID(),
			fnb.BaseConfig.UnicastMessageTimeout,
			true,
			fnb.IDTranslator,
			mwOpts...,
		)

		subscriptionManager := p2p.NewChannelSubscriptionManager(fnb.Middleware)

		top, err := topology.NewTopicBasedTopology(
			fnb.NodeID,
			fnb.Logger,
			fnb.State,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create topology: %w", err)
		}
		topologyCache := topology.NewCache(fnb.Logger, top)

		// creates network instance
		net, err := p2p.NewNetwork(fnb.Logger,
			codec,
			fnb.Me,
			func() (network.Middleware, error) { return fnb.Middleware, nil },
			fnb.NetworkReceivedMessageCacheSize,
			topologyCache,
			subscriptionManager,
			fnb.Metrics.Network,
			fnb.IdentityProvider,
		)
		if err != nil {
			return nil, fmt.Errorf("could not initialize network: %w", err)
		}

		fnb.Network = net

		idEvents := gadgets.NewIdentityDeltas(func() {
			fnb.Middleware.UpdateNodeAddresses()
			fnb.Middleware.UpdateAllowList()
		})
		fnb.ProtocolEvents.AddConsumer(idEvents)

		return net, nil
	})
}

func (fnb *FlowNodeBuilder) EnqueueMetricsServerInit() {
	fnb.Component("metrics server", func(node *NodeConfig, lookup component.LookupFunc) (module.ReadyDoneAware, error) {
		server := metrics.NewServer(fnb.Logger, fnb.BaseConfig.metricsPort, fnb.BaseConfig.profilerEnabled)
		return server, nil
	})
}

func (fnb *FlowNodeBuilder) EnqueueAdminServerInit() {
	if fnb.AdminAddr != NotSet {
		if (fnb.AdminCert != NotSet || fnb.AdminKey != NotSet || fnb.AdminClientCAs != NotSet) &&
			!(fnb.AdminCert != NotSet && fnb.AdminKey != NotSet && fnb.AdminClientCAs != NotSet) {
			fnb.Logger.Fatal().Msg("admin cert / key and client certs must all be provided to enable mutual TLS")
		}
		fnb.RegisterDefaultAdminCommands()
		fnb.Component("admin server", func(node *NodeConfig, lookup component.LookupFunc) (module.ReadyDoneAware, error) {
			var opts []admin.CommandRunnerOption

			if node.AdminCert != NotSet {
				serverCert, err := tls.LoadX509KeyPair(node.AdminCert, node.AdminKey)
				if err != nil {
					return nil, err
				}
				clientCAs, err := ioutil.ReadFile(node.AdminClientCAs)
				if err != nil {
					return nil, err
				}
				certPool := x509.NewCertPool()
				certPool.AppendCertsFromPEM(clientCAs)
				config := &tls.Config{
					MinVersion:   tls.VersionTLS13,
					Certificates: []tls.Certificate{serverCert},
					ClientAuth:   tls.RequireAndVerifyClientCert,
					ClientCAs:    certPool,
				}

				opts = append(opts, admin.WithTLS(config))
			}

			command_runner := fnb.adminCommandBootstrapper.Bootstrap(fnb.Logger, fnb.AdminAddr, opts...)

			return command_runner, nil
		})
	}
}

func (fnb *FlowNodeBuilder) RegisterBadgerMetrics() error {
	return metrics.RegisterBadgerMetrics()
}

func (fnb *FlowNodeBuilder) EnqueueTracer() {
	fnb.Component("tracer", func(node *NodeConfig, lookup component.LookupFunc) (module.ReadyDoneAware, error) {
		return fnb.Tracer, nil
	})
}

func (fnb *FlowNodeBuilder) ParseAndPrintFlags() {
	// parse configuration parameters
	pflag.Parse()

	// print all flags
	log := fnb.Logger.Info()

	pflag.VisitAll(func(flag *pflag.Flag) {
		log = log.Str(flag.Name, flag.Value.String())
	})

	log.Msg("flags loaded")
}

func (fnb *FlowNodeBuilder) ValidateFlags(f func() error) NodeBuilder {
	fnb.extraFlagCheck = f
	return fnb
}

func (fnb *FlowNodeBuilder) PrintBuildVersionDetails() {
	fnb.Logger.Info().Str("version", build.Semver()).Str("commit", build.Commit()).Msg("build details")
}

func (fnb *FlowNodeBuilder) initNodeInfo() {
	if fnb.BaseConfig.nodeIDHex == NotSet {
		fnb.Logger.Fatal().Msg("cannot start without node ID")
	}

	nodeID, err := flow.HexStringToIdentifier(fnb.BaseConfig.nodeIDHex)
	if err != nil {
		fnb.Logger.Fatal().Err(err).Msgf("could not parse node ID from string: %v", fnb.BaseConfig.nodeIDHex)
	}

	info, err := LoadPrivateNodeInfo(fnb.BaseConfig.BootstrapDir, nodeID)
	if err != nil {
		fnb.Logger.Fatal().Err(err).Msg("failed to load private node info")
	}

	fnb.NodeID = nodeID
	fnb.NetworkKey = info.NetworkPrivKey.PrivateKey
	fnb.StakingKey = info.StakingPrivKey.PrivateKey
}

func (fnb *FlowNodeBuilder) initLogger() {
	// configure logger with standard level, node ID and UTC timestamp
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := fnb.Logger.With().
		Timestamp().
		Str("node_role", fnb.BaseConfig.NodeRole).
		Str("node_id", fnb.NodeID.String()).
		Logger()

	log.Info().Msgf("flow %s node starting up", fnb.BaseConfig.NodeRole)

	// parse config log level and apply to logger
	lvl, err := zerolog.ParseLevel(strings.ToLower(fnb.BaseConfig.level))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log = log.Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(lvl)

	fnb.Logger = log
}

func (fnb *FlowNodeBuilder) initMetrics() {

	fnb.Tracer = trace.NewNoopTracer()
	if fnb.BaseConfig.tracerEnabled {
		serviceName := fnb.BaseConfig.NodeRole + "-" + fnb.BaseConfig.nodeIDHex[:8]
		tracer, err := trace.NewTracer(fnb.Logger,
			serviceName,
			fnb.RootChainID.String(),
			fnb.tracerSensitivity)
		fnb.MustNot(err).Msg("could not initialize tracer")
		fnb.Logger.Info().Msg("Tracer Started")
		fnb.Tracer = tracer
	}

	fnb.Metrics = Metrics{
		Network:        metrics.NewNoopCollector(),
		Engine:         metrics.NewNoopCollector(),
		Compliance:     metrics.NewNoopCollector(),
		Cache:          metrics.NewNoopCollector(),
		Mempool:        metrics.NewNoopCollector(),
		CleanCollector: metrics.NewNoopCollector(),
	}
	if fnb.BaseConfig.metricsEnabled {
		fnb.MetricsRegisterer = prometheus.DefaultRegisterer

		mempools := metrics.NewMempoolCollector(5 * time.Second)

		fnb.Metrics = Metrics{
			Network:    metrics.NewNetworkCollector(),
			Engine:     metrics.NewEngineCollector(),
			Compliance: metrics.NewComplianceCollector(),
			// CacheControl metrics has been causing memory abuse, disable for now
			// Cache:          metrics.NewCacheCollector(fnb.RootChainID),
			Cache:          metrics.NewNoopCollector(),
			CleanCollector: metrics.NewCleanerCollector(),
			Mempool:        mempools,
		}

		// registers mempools as a Component so that its Ready method is invoked upon startup
		fnb.Component("mempools metrics", func(node *NodeConfig, lookup component.LookupFunc) (module.ReadyDoneAware, error) {
			return mempools, nil
		})
	}
}

// TODO: should this be added in Initialize()?
func (fnb *FlowNodeBuilder) initProfiler() {
	if !fnb.BaseConfig.profilerEnabled {
		return
	}
	profiler, err := debug.NewAutoProfiler(
		fnb.Logger,
		fnb.BaseConfig.profilerDir,
		fnb.BaseConfig.profilerInterval,
		fnb.BaseConfig.profilerDuration,
	)
	fnb.MustNot(err).Msg("could not initialize profiler")
	fnb.Component("profiler", func(node *NodeConfig, lookup component.LookupFunc) (module.ReadyDoneAware, error) {
		return profiler, nil
	})
}

func (fnb *FlowNodeBuilder) initDB() {

	// if a db has been passed in, use that instead of creating one
	if fnb.BaseConfig.db != nil {
		fnb.DB = fnb.BaseConfig.db
		return
	}

	// Pre-create DB path (Badger creates only one-level dirs)
	err := os.MkdirAll(fnb.BaseConfig.datadir, 0700)
	fnb.MustNot(err).Str("dir", fnb.BaseConfig.datadir).Msg("could not create datadir")

	log := sutil.NewLogger(fnb.Logger)

	// we initialize the database with options that allow us to keep the maximum
	// item size in the trie itself (up to 1MB) and where we keep all level zero
	// tables in-memory as well; this slows down compaction and increases memory
	// usage, but it improves overall performance and disk i/o
	opts := badger.
		DefaultOptions(fnb.BaseConfig.datadir).
		WithKeepL0InMemory(true).
		WithLogger(log).

		// the ValueLogFileSize option specifies how big the value of a
		// key-value pair is allowed to be saved into badger.
		// exceeding this limit, will fail with an error like this:
		// could not store data: Value with size <xxxx> exceeded 1073741824 limit
		// Maximum value size is 10G, needed by execution node
		// TODO: finding a better max value for each node type
		WithValueLogFileSize(128 << 23).
		WithValueLogMaxEntries(100000) // Default is 1000000

	publicDB, err := bstorage.InitPublic(opts)
	fnb.MustNot(err).Msg("could not open public db")
	fnb.DB = publicDB
}

func (fnb *FlowNodeBuilder) initSecretsDB() {

	// if the secrets DB is disabled (only applicable for Consensus Follower,
	// which makes use of this same logic), skip this initialization
	if !fnb.BaseConfig.secretsDBEnabled {
		return
	}

	if fnb.BaseConfig.secretsdir == NotSet {
		fnb.Logger.Fatal().Msgf("missing required flag '--secretsdir'")
	}

	err := os.MkdirAll(fnb.BaseConfig.secretsdir, 0700)
	fnb.MustNot(err).Str("dir", fnb.BaseConfig.secretsdir).Msg("could not create secrets db dir")

	log := sutil.NewLogger(fnb.Logger)

	opts := badger.DefaultOptions(fnb.BaseConfig.secretsdir).WithLogger(log)
	// attempt to read an encryption key for the secrets DB from the canonical path
	// TODO enforce encryption in an upcoming spork https://github.com/dapperlabs/flow-go/issues/5893
	encryptionKey, err := loadSecretsEncryptionKey(fnb.BootstrapDir, fnb.NodeID)
	if errors.Is(err, os.ErrNotExist) {
		fnb.Logger.Warn().Msg("starting with secrets database encryption disabled")
	} else if err != nil {
		fnb.Logger.Fatal().Err(err).Msg("failed to read secrets db encryption key")
	} else {
		opts = opts.WithEncryptionKey(encryptionKey)
	}

	secretsDB, err := bstorage.InitSecret(opts)
	fnb.MustNot(err).Msg("could not open secrets db")
	fnb.SecretsDB = secretsDB
}

func (fnb *FlowNodeBuilder) initStorage() {

	// in order to void long iterations with big keys when initializing with an
	// already populated database, we bootstrap the initial maximum key size
	// upon starting
	err := operation.RetryOnConflict(fnb.DB.Update, func(tx *badger.Txn) error {
		return operation.InitMax(tx)
	})
	fnb.MustNot(err).Msg("could not initialize max tracker")

	headers := bstorage.NewHeaders(fnb.Metrics.Cache, fnb.DB)
	guarantees := bstorage.NewGuarantees(fnb.Metrics.Cache, fnb.DB, fnb.BaseConfig.guaranteesCacheSize)
	seals := bstorage.NewSeals(fnb.Metrics.Cache, fnb.DB)
	results := bstorage.NewExecutionResults(fnb.Metrics.Cache, fnb.DB)
	receipts := bstorage.NewExecutionReceipts(fnb.Metrics.Cache, fnb.DB, results, fnb.BaseConfig.receiptsCacheSize)
	index := bstorage.NewIndex(fnb.Metrics.Cache, fnb.DB)
	payloads := bstorage.NewPayloads(fnb.DB, index, guarantees, seals, receipts, results)
	blocks := bstorage.NewBlocks(fnb.DB, headers, payloads)
	transactions := bstorage.NewTransactions(fnb.Metrics.Cache, fnb.DB)
	collections := bstorage.NewCollections(fnb.DB, transactions)
	setups := bstorage.NewEpochSetups(fnb.Metrics.Cache, fnb.DB)
	commits := bstorage.NewEpochCommits(fnb.Metrics.Cache, fnb.DB)
	statuses := bstorage.NewEpochStatuses(fnb.Metrics.Cache, fnb.DB)

	fnb.Storage = Storage{
		Headers:      headers,
		Guarantees:   guarantees,
		Receipts:     receipts,
		Results:      results,
		Seals:        seals,
		Index:        index,
		Payloads:     payloads,
		Blocks:       blocks,
		Transactions: transactions,
		Collections:  collections,
		Setups:       setups,
		Commits:      commits,
		Statuses:     statuses,
	}
}

func (fnb *FlowNodeBuilder) InitIDProviders() {
	fnb.Module("id providers", func(node *NodeConfig) error {
		idCache, err := p2p.NewProtocolStateIDCache(node.Logger, node.State, node.ProtocolEvents)
		if err != nil {
			return err
		}

		node.IdentityProvider = idCache
		node.IDTranslator = idCache
		node.SyncEngineIdentifierProvider = id.NewIdentityFilterIdentifierProvider(
			filter.And(
				filter.HasRole(flow.RoleConsensus),
				filter.Not(filter.HasNodeID(node.Me.NodeID())),
				p2p.NotEjectedFilter,
			),
			idCache,
		)
		return nil
	})
}

func (fnb *FlowNodeBuilder) initState() {
	fnb.ProtocolEvents = events.NewDistributor()

	// load the root protocol state snapshot from disk
	rootSnapshot, err := loadRootProtocolSnapshot(fnb.BaseConfig.BootstrapDir)
	fnb.MustNot(err).Msg("failed to read protocol snapshot from disk")

	fnb.RootResult, fnb.RootSeal, err = rootSnapshot.SealedResult()
	fnb.MustNot(err).Msg("failed to read root sealed result")
	sealingSegment, err := rootSnapshot.SealingSegment()
	fnb.MustNot(err).Msg("failed to read root sealing segment")
	fnb.RootBlock = sealingSegment[len(sealingSegment)-1]
	fnb.RootQC, err = rootSnapshot.QuorumCertificate()
	fnb.MustNot(err).Msg("failed to read root qc")
	// set the chain ID based on the root header
	// TODO: as the root header can now be loaded from protocol state, we should
	// not use a global variable for chain ID anymore, but rely on the protocol
	// state as final authority on what the chain ID is
	// => https://github.com/dapperlabs/flow-go/issues/4167
	fnb.RootChainID = fnb.RootBlock.Header.ChainID

	isBootStrapped, err := badgerState.IsBootstrapped(fnb.DB)
	fnb.MustNot(err).Msg("failed to determine whether database contains bootstrapped state")
	if isBootStrapped {
		state, err := badgerState.OpenState(
			fnb.Metrics.Compliance,
			fnb.DB,
			fnb.Storage.Headers,
			fnb.Storage.Seals,
			fnb.Storage.Results,
			fnb.Storage.Blocks,
			fnb.Storage.Setups,
			fnb.Storage.Commits,
			fnb.Storage.Statuses,
		)
		fnb.MustNot(err).Msg("could not open flow state")
		fnb.State = state

		// Verify root block in protocol state is consistent with bootstrap information stored on-disk.
		// Inconsistencies can happen when the bootstrap root block is updated (because of new spork),
		// but the protocol state is not updated, so they don't match
		// when this happens during a spork, we could try deleting the protocol state database.
		// TODO: revisit this check when implementing Epoch
		rootBlockFromState, err := state.Params().Root()
		fnb.MustNot(err).Msg("could not load root block from protocol state")
		if fnb.RootBlock.ID() != rootBlockFromState.ID() {
			fnb.Logger.Fatal().Msgf("mismatching root block ID, protocol state block ID: %v, bootstrap root block ID: %v",
				rootBlockFromState.ID(),
				fnb.RootBlock.ID(),
			)
		}
	} else {
		// Bootstrap!
		fnb.Logger.Info().Msg("bootstrapping empty protocol state")

		// generate bootstrap config options as per NodeConfig
		var options []badgerState.BootstrapConfigOptions
		if fnb.SkipNwAddressBasedValidations {
			options = append(options, badgerState.SkipNetworkAddressValidation)
		}

		fnb.State, err = badgerState.Bootstrap(
			fnb.Metrics.Compliance,
			fnb.DB,
			fnb.Storage.Headers,
			fnb.Storage.Seals,
			fnb.Storage.Results,
			fnb.Storage.Blocks,
			fnb.Storage.Setups,
			fnb.Storage.Commits,
			fnb.Storage.Statuses,
			rootSnapshot,
			options...,
		)
		fnb.MustNot(err).Msg("could not bootstrap protocol state")

		fnb.Logger.Info().
			Hex("root_result_id", logging.Entity(fnb.RootResult)).
			Hex("root_state_commitment", fnb.RootSeal.FinalState[:]).
			Hex("root_block_id", logging.Entity(fnb.RootBlock)).
			Uint64("root_block_height", fnb.RootBlock.Header.Height).
			Msg("genesis state bootstrapped")
	}

	// initialize local if it hasn't been initialized yet
	if fnb.Me == nil {
		fnb.initLocal()
	}

	lastFinalized, err := fnb.State.Final().Head()
	fnb.MustNot(err).Msg("could not get last finalized block header")
	fnb.Logger.Info().
		Hex("block_id", logging.Entity(lastFinalized)).
		Uint64("height", lastFinalized.Height).
		Msg("last finalized block")
}

func (fnb *FlowNodeBuilder) initLocal() {
	// Verify that my ID (as given in the configuration) is known to the network
	// (i.e. protocol state). There are two cases that will cause the following error:
	// 1) used the wrong node id, which is not part of the identity list of the finalized state
	// 2) the node id is a new one for a new spork, but the bootstrap data has not been updated.
	myID, err := flow.HexStringToIdentifier(fnb.BaseConfig.nodeIDHex)
	fnb.MustNot(err).Msg("could not parse node identifier")

	self, err := fnb.State.Final().Identity(myID)
	fnb.MustNot(err).Msgf("node identity not found in the identity list of the finalized state: %v", myID)

	// Verify that my role (as given in the configuration) is consistent with the protocol state.
	// We enforce this strictly for MainNet. For other networks (e.g. TestNet or BenchNet), we
	// are lenient, to allow ghost node to run as any role.
	if self.Role.String() != fnb.BaseConfig.NodeRole {
		rootBlockHeader, err := fnb.State.Params().Root()
		fnb.MustNot(err).Msg("could not get root block from protocol state")
		if rootBlockHeader.ChainID == flow.Mainnet {
			fnb.Logger.Fatal().Msgf("running as incorrect role, expected: %v, actual: %v, exiting",
				self.Role.String(),
				fnb.BaseConfig.NodeRole)
		} else {
			fnb.Logger.Warn().Msgf("running as incorrect role, expected: %v, actual: %v, continuing",
				self.Role.String(),
				fnb.BaseConfig.NodeRole)
		}
	}

	// ensure that the configured staking/network keys are consistent with the protocol state
	if !self.NetworkPubKey.Equals(fnb.NetworkKey.PublicKey()) {
		fnb.Logger.Fatal().Msg("configured networking key does not match protocol state")
	}
	if !self.StakingPubKey.Equals(fnb.StakingKey.PublicKey()) {
		fnb.Logger.Fatal().Msg("configured staking key does not match protocol state")
	}

	fnb.Me, err = local.New(self, fnb.StakingKey)
	fnb.MustNot(err).Msg("could not initialize local")
}

func (fnb *FlowNodeBuilder) initFvmOptions() {
	blockFinder := fvm.NewBlockFinder(fnb.Storage.Headers)
	vmOpts := []fvm.Option{
		fvm.WithChain(fnb.RootChainID.Chain()),
		fvm.WithBlocks(blockFinder),
		fvm.WithAccountStorageLimit(true),
	}
	if fnb.RootChainID == flow.Testnet || fnb.RootChainID == flow.Canary || fnb.RootChainID == flow.Mainnet {
		vmOpts = append(vmOpts,
			fvm.WithTransactionFeesEnabled(true),
		)
	}
	if fnb.RootChainID == flow.Testnet || fnb.RootChainID == flow.Canary {
		vmOpts = append(vmOpts,
			fvm.WithRestrictedDeployment(false),
		)
	}
	fnb.FvmOptions = vmOpts
}

func (fnb *FlowNodeBuilder) handleModule(v namedModuleFunc) error {
	if err := v.fn(fnb.NodeConfig); err != nil {
		return fmt.Errorf("module %s initialization failed: %w", v.name, err)
	}

	fnb.Logger.Info().Str("module", v.name).Msg("module initialization complete")
	return nil
}

// ExtraFlags enables binding additional flags beyond those defined in BaseConfig.
func (fnb *FlowNodeBuilder) ExtraFlags(f func(*pflag.FlagSet)) NodeBuilder {
	f(fnb.flags)
	return fnb
}

// Module enables setting up dependencies of the engine with the builder context.
func (fnb *FlowNodeBuilder) Module(name string, f BuilderFunc) NodeBuilder {
	fnb.modules = append(fnb.modules, namedModuleFunc{
		fn:   f,
		name: name,
	})
	return fnb
}

func (fnb *FlowNodeBuilder) AdminCommand(command string, f func(config *NodeConfig) commands.AdminCommand) NodeBuilder {
	fnb.adminCommands[command] = f
	return fnb
}

// MustNot asserts that the given error must not occur.
//
// If the error is nil, returns a nil log event (which acts as a no-op).
// If the error is not nil, returns a fatal log event containing the error.
func (fnb *FlowNodeBuilder) MustNot(err error) *zerolog.Event {
	if err != nil {
		return fnb.Logger.Fatal().Err(err)
	}
	return nil
}

// Component adds a new component to the node that conforms to the ReadyDoneAware
// interface, and throws a Fatal() when an irrecoverable error is encountered
// Use Component if the component cannot be restarted when an irrecoverable error is encountered
// and the node should crash.
//
// When the node is run, this component will be started with `Ready`. When the
// node is stopped, we will wait for the component to exit gracefully with
// `Done`.
func (fnb *FlowNodeBuilder) Component(name string, f ComponentBuilderFunc) NodeBuilder {

	fnb.componentBuilder.AddWorker(name, func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc, lookup component.LookupFunc) {
		modules, _ := lookup("modules")
		if err := util.WaitClosed(ctx, modules.Ready()); err != nil || util.CheckClosed(ctx.Done()) {
			return
		}

		log := fnb.Logger.With().Str("component", name).Logger()

		readyAware, err := f(fnb.NodeConfig, lookup)
		if err != nil {
			ctx.Throw(fmt.Errorf("component %s initialization failed: %w", name, err))
		}
		log.Info().Msg("component initialization complete")

		// TODO: switch to using RunComponent after we've refactored each of the ReadyDoneAware
		//       objects to implement component.Component
		if component, ok := readyAware.(component.Component); ok {
			go component.Start(ctx)
		}

		if err := util.WaitClosed(ctx, readyAware.Ready()); err != nil {
			log.Info().Msg("component startup aborted")
		} else {
			log.Info().Msg("component startup complete")
			ready()
		}

		<-ctx.Done()
		log.Info().Msg("component shutdown started")

		<-readyAware.Done()
		log.Info().Msg("component shutdown complete")
	})

	return fnb
}

// BackgroundComponent adds a new component to the node that conforms to the ReadyDoneAware
// interface, and calls the provided error handler when an irrecoverable error is encountered.
// Use BackgroundComponent if the component is not critical to the node's safe operation and
// can/should be independently restarted when an irrecoverable error is encountered.
//
// Any irrecoverable errors thrown by the component will be passed to the provided error handler.
func (fnb *FlowNodeBuilder) BackgroundComponent(name string, f BackgroundComponentBuilderFunc, errHandler component.OnError) NodeBuilder {

	fnb.componentBuilder.AddWorker(name, func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc, lookup component.LookupFunc) {
		modules, _ := lookup("modules")
		if err := util.WaitClosed(ctx, modules.Ready()); err != nil || util.CheckClosed(ctx.Done()) {
			return
		}

		log := fnb.Logger.With().Str("component", name).Logger()

		componentFactory := func() (component.Component, error) {
			c, err := f(fnb.NodeConfig, lookup)
			if err != nil {
				return nil, err
			}
			log.Info().Msg("component initialization complete")

			go func() {
				if err := util.WaitClosed(ctx, c.Ready()); err != nil {
					log.Info().Msg("component startup aborted")
				} else {
					log.Info().Msg("component startup complete")
				}

				<-ctx.Done()
				log.Info().Msg("component shutdown started")
			}()
			return c, nil
		}

		// Note: we're marking the worker routine as ready before we even attempt to start the
		// component. the idea behind a background component is that the system as a whole should
		// not depend on it for safe operation, so the node does not need to wait for it to be ready.
		ready()
		err := component.RunComponent(ctx, componentFactory, errHandler)
		if err != nil && err != ctx.Err() {
			ctx.Throw(fmt.Errorf("component %s shutdown unexpectedly: %w", name, err))
		}

		log.Info().Msg("component shutdown complete")
	})

	return fnb
}

func (fnb *FlowNodeBuilder) PreInit(f BuilderFunc) NodeBuilder {
	fnb.preInitFns = append(fnb.preInitFns, f)
	return fnb
}

func (fnb *FlowNodeBuilder) PostInit(f BuilderFunc) NodeBuilder {
	fnb.postInitFns = append(fnb.postInitFns, f)
	return fnb
}

type Option func(*BaseConfig)

func WithBootstrapDir(bootstrapDir string) Option {
	return func(config *BaseConfig) {
		config.BootstrapDir = bootstrapDir
	}
}

func WithBindAddress(bindAddress string) Option {
	return func(config *BaseConfig) {
		config.BindAddr = bindAddress
	}
}

func WithDataDir(dataDir string) Option {
	return func(config *BaseConfig) {
		if config.db == nil {
			config.datadir = dataDir
		}
	}
}

func WithSecretsDBEnabled(enabled bool) Option {
	return func(config *BaseConfig) {
		config.secretsDBEnabled = enabled
	}
}

func WithMetricsEnabled(enabled bool) Option {
	return func(config *BaseConfig) {
		config.metricsEnabled = enabled
	}
}

func WithLogLevel(level string) Option {
	return func(config *BaseConfig) {
		config.level = level
	}
}

// WithDB takes precedence over WithDataDir and datadir will be set to empty if DB is set using this option
func WithDB(db *badger.DB) Option {
	return func(config *BaseConfig) {
		config.db = db
		config.datadir = ""
	}
}

// FlowNode creates a new Flow node builder with the given name.
func FlowNode(role string, opts ...Option) *FlowNodeBuilder {
	config := DefaultBaseConfig()
	config.NodeRole = role
	for _, opt := range opts {
		opt(config)
	}

	builder := &FlowNodeBuilder{
		NodeConfig: &NodeConfig{
			BaseConfig: *config,
			Logger:     zerolog.New(os.Stderr),
		},
		flags:                    pflag.CommandLine,
		adminCommandBootstrapper: admin.NewCommandRunnerBootstrapper(),
		adminCommands:            make(map[string]func(*NodeConfig) commands.AdminCommand),
	}
	return builder
}

func (fnb *FlowNodeBuilder) Initialize() error {
	fnb.PrintBuildVersionDetails()

	fnb.BaseFlags()

	fnb.ParseAndPrintFlags()

	if err := fnb.extraFlagsValidation(); err != nil {
		return err
	}

	fnb.InitComponentBuilder()

	// ID providers must be initialized before the network
	fnb.InitIDProviders()

	fnb.EnqueueNetworkInit()

	if fnb.metricsEnabled {
		fnb.EnqueueMetricsServerInit()
		if err := fnb.RegisterBadgerMetrics(); err != nil {
			return err
		}
	}

	fnb.EnqueueAdminServerInit()

	fnb.EnqueueTracer()

	return nil
}

func (fnb *FlowNodeBuilder) RegisterDefaultAdminCommands() {
	fnb.AdminCommand("set-log-level", func(config *NodeConfig) commands.AdminCommand {
		return &common.SetLogLevelCommand{}
	}).AdminCommand("read-protocol-state-blocks", func(config *NodeConfig) commands.AdminCommand {
		return common.NewReadProtocolStateBlocksCommand(config.State, config.Storage.Blocks)
	})
}

func (fnb *FlowNodeBuilder) InitComponentBuilder() {
	fnb.componentBuilder = component.NewComponentManagerBuilder().
		AddWorker("main", func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc, lookup component.LookupFunc) {
			fnb.onStart(ctx)
			ready()

			// runs after all other workers have completed
			workers, _ := lookup("!main")
			<-workers.Done()
			fnb.closeDatabase()
		}).
		AddWorker("modules", func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc, lookup component.LookupFunc) {
			main, _ := lookup("main")
			if err := util.WaitClosed(ctx, main.Ready()); err != nil || util.CheckClosed(ctx.Done()) {
				return
			}

			// run all modules
			for _, f := range fnb.modules {
				if err := fnb.handleModule(f); err != nil {
					ctx.Throw(err)
				}
			}
			ready()
		})
}

// SerialStart configures the node to start components serially
// Ideally, components should have explicit dependency checks to handle startup ordering. This is
// here to ensure the existing assumption that components are started in order serially remains
// true until we've fully transitioned to explicit dependency checks
func (fnb *FlowNodeBuilder) SerialStart() NodeBuilder {
	fnb.componentBuilder.SetSerialStart(true)
	return fnb
}

func (fnb *FlowNodeBuilder) Build() Node {
	return &FlowNodeImp{
		ComponentManager: fnb.componentBuilder.Build(),
		NodeConfig:       fnb.NodeConfig,
		Logger:           fnb.Logger,
	}
}

func (fnb *FlowNodeBuilder) onStart(ctx irrecoverable.SignalerContext) {

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// init nodeinfo by reading the private bootstrap file if not already set
	if fnb.NodeID == flow.ZeroID {
		fnb.initNodeInfo()
	}

	fnb.initLogger()

	fnb.initProfiler()

	fnb.initDB()
	fnb.initSecretsDB()

	fnb.initMetrics()

	fnb.initStorage()

	for _, f := range fnb.preInitFns {
		if err := fnb.handlePreInit(f); err != nil {
			ctx.Throw(err)
		}
	}

	fnb.initState()

	fnb.initFvmOptions()

	for _, f := range fnb.postInitFns {
		if err := fnb.handlePostInit(f); err != nil {
			ctx.Throw(err)
		}
	}

	// set up all admin commands
	for commandName, commandFunc := range fnb.adminCommands {
		command := commandFunc(fnb.NodeConfig)
		fnb.adminCommandBootstrapper.RegisterHandler(commandName, command.Handler)
		fnb.adminCommandBootstrapper.RegisterValidator(commandName, command.Validator)
	}
}

func (fnb *FlowNodeBuilder) handlePreInit(f BuilderFunc) error {
	return f(fnb.NodeConfig)
}

func (fnb *FlowNodeBuilder) handlePostInit(f BuilderFunc) error {
	return f(fnb.NodeConfig)
}

func (fnb *FlowNodeBuilder) closeDatabase() {
	err := fnb.DB.Close()
	if err != nil {
		fnb.Logger.Error().
			Err(err).
			Msg("could not close database")
	}
}

func (fnb *FlowNodeBuilder) extraFlagsValidation() error {
	if fnb.extraFlagCheck != nil {
		err := fnb.extraFlagCheck()
		if err != nil {
			return fmt.Errorf("invalid flags: %w", err)
		}
	}
	return nil
}

// loadRootProtocolSnapshot loads the root protocol snapshot from disk
func loadRootProtocolSnapshot(dir string) (*inmem.Snapshot, error) {
	path := filepath.Join(dir, bootstrap.PathRootProtocolStateSnapshot)
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read root snapshot (path=%s): %w", path, err)
	}

	var snapshot inmem.EncodableSnapshot
	err = json.Unmarshal(data, &snapshot)
	if err != nil {
		return nil, err
	}

	return inmem.SnapshotFromEncodable(snapshot), nil
}

// Loads the private info for this node from disk (eg. private staking/network keys).
func LoadPrivateNodeInfo(dir string, myID flow.Identifier) (*bootstrap.NodeInfoPriv, error) {
	path := filepath.Join(dir, fmt.Sprintf(bootstrap.PathNodeInfoPriv, myID))
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read private node info (path=%s): %w", path, err)
	}
	var info bootstrap.NodeInfoPriv
	err = json.Unmarshal(data, &info)
	return &info, err
}

// loadSecretsEncryptionKey loads the encryption key for the secrets database.
// If the file does not exist, returns os.ErrNotExist.
func loadSecretsEncryptionKey(dir string, myID flow.Identifier) ([]byte, error) {
	path := filepath.Join(dir, fmt.Sprintf(bootstrap.PathSecretsEncryptionKey, myID))
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read secrets db encryption key (path=%s): %w", path, err)
	}
	return data, nil
}
