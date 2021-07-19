package cmd

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	jsoncodec "github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/state/protocol"
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

const notSet = "not set"

// BaseConfig is the general config for the FlowNodeBuilder
type BaseConfig struct {
	nodeIDHex             string
	bindAddr              string
	nodeRole              string
	timeout               time.Duration
	datadir               string
	level                 string
	metricsPort           uint
	BootstrapDir          string
	peerUpdateInterval    time.Duration
	unicastMessageTimeout time.Duration
	profilerEnabled       bool
	profilerDir           string
	profilerInterval      time.Duration
	profilerDuration      time.Duration
	tracerEnabled         bool
}

type Metrics struct {
	Network    module.NetworkMetrics
	Engine     module.EngineMetrics
	Compliance module.ComplianceMetrics
	Cache      module.CacheMetrics
	Mempool    module.MempoolMetrics
}

type Storage struct {
	Headers      storage.Headers
	Index        storage.Index
	Identities   storage.Identities
	Guarantees   storage.Guarantees
	Receipts     storage.ExecutionReceipts
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

type namedModuleFunc struct {
	fn   func(*FlowNodeBuilder) error
	name string
}

type namedComponentFunc struct {
	fn   func(*FlowNodeBuilder) (module.ReadyDoneAware, error)
	name string
}

type namedDoneObject struct {
	ob   module.ReadyDoneAware
	name string
}

// FlowNodeBuilder is the builder struct used for all flow nodes
// It runs a node process with following structure, in sequential order
// Base inits (network, storage, state, logger)
//   PostInit handlers, if any
// Components handlers, if any, wait sequentially
// Run() <- main loop
// Components destructors, if any
type FlowNodeBuilder struct {
	BaseConfig        BaseConfig
	NodeID            flow.Identifier
	flags             *pflag.FlagSet
	Logger            zerolog.Logger
	Me                *local.Local
	Tracer            module.Tracer
	MetricsRegisterer prometheus.Registerer
	Metrics           Metrics
	DB                *badger.DB
	Storage           Storage
	ProtocolEvents    *events.Distributor
	State             protocol.State
	Middleware        *p2p.Middleware
	Network           *p2p.Network
	MsgValidators     []network.MessageValidator
	FvmOptions        []fvm.Option
	modules           []namedModuleFunc
	components        []namedComponentFunc
	doneObject        []namedDoneObject
	sig               chan os.Signal
	postInitFns       []func(*FlowNodeBuilder)
	stakingKey        crypto.PrivateKey
	NetworkKey        crypto.PrivateKey

	// root state information
	RootBlock   *flow.Block
	RootQC      *flow.QuorumCertificate
	RootResult  *flow.ExecutionResult
	RootSeal    *flow.Seal
	RootChainID flow.ChainID
}

func (fnb *FlowNodeBuilder) baseFlags() {
	homedir, _ := os.UserHomeDir()
	datadir := filepath.Join(homedir, ".flow", "database")
	// bind configuration parameters
	fnb.flags.StringVar(&fnb.BaseConfig.nodeIDHex, "nodeid", notSet, "identity of our node")
	fnb.flags.StringVar(&fnb.BaseConfig.bindAddr, "bind", notSet, "address to bind on")
	fnb.flags.StringVarP(&fnb.BaseConfig.BootstrapDir, "bootstrapdir", "b", "bootstrap", "path to the bootstrap directory")
	fnb.flags.DurationVarP(&fnb.BaseConfig.timeout, "timeout", "t", 1*time.Minute, "how long to try connecting to the network")
	fnb.flags.StringVarP(&fnb.BaseConfig.datadir, "datadir", "d", datadir, "directory to store the protocol state")
	fnb.flags.StringVarP(&fnb.BaseConfig.level, "loglevel", "l", "info", "level for logging output")
	fnb.flags.DurationVar(&fnb.BaseConfig.peerUpdateInterval, "peerupdate-interval", p2p.DefaultPeerUpdateInterval, "how often to refresh the peer connections for the node")
	fnb.flags.DurationVar(&fnb.BaseConfig.unicastMessageTimeout, "unicast-timeout", p2p.DefaultUnicastTimeout, "how long a unicast transmission can take to complete")
	fnb.flags.UintVarP(&fnb.BaseConfig.metricsPort, "metricport", "m", 8080, "port for /metrics endpoint")
	fnb.flags.BoolVar(&fnb.BaseConfig.profilerEnabled, "profiler-enabled", false, "whether to enable the auto-profiler")
	fnb.flags.StringVar(&fnb.BaseConfig.profilerDir, "profiler-dir", "profiler", "directory to create auto-profiler profiles")
	fnb.flags.DurationVar(&fnb.BaseConfig.profilerInterval, "profiler-interval", 15*time.Minute,
		"the interval between auto-profiler runs")
	fnb.flags.DurationVar(&fnb.BaseConfig.profilerDuration, "profiler-duration", 10*time.Second,
		"the duration to run the auto-profile for")
	fnb.flags.BoolVar(&fnb.BaseConfig.tracerEnabled, "tracer-enabled", false,
		"whether to enable tracer")

}

func (fnb *FlowNodeBuilder) enqueueNetworkInit() {
	fnb.Component("network", func(builder *FlowNodeBuilder) (module.ReadyDoneAware, error) {

		codec := jsoncodec.NewCodec()

		myAddr := fnb.Me.Address()
		if fnb.BaseConfig.bindAddr != notSet {
			myAddr = fnb.BaseConfig.bindAddr
		}

		// setup the Ping provider to return the software version and the finalized block height
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
		}

		libP2PNodeFactory, err := p2p.DefaultLibP2PNodeFactory(fnb.Logger.Level(zerolog.ErrorLevel),
			fnb.Me.NodeID(),
			myAddr,
			fnb.NetworkKey,
			fnb.RootBlock.ID().String(),
			p2p.DefaultMaxPubSubMsgSize,
			fnb.Metrics.Network,
			pingProvider)
		if err != nil {
			return nil, fmt.Errorf("could not generate libp2p node factory: %w", err)
		}

		fnb.Middleware = p2p.NewMiddleware(fnb.Logger.Level(zerolog.ErrorLevel),
			libP2PNodeFactory,
			fnb.Me.NodeID(),
			fnb.Metrics.Network,
			fnb.RootBlock.ID().String(),
			fnb.BaseConfig.peerUpdateInterval,
			fnb.BaseConfig.unicastMessageTimeout,
			fnb.MsgValidators...)

		participants, err := fnb.State.Final().Identities(p2p.NetworkingSetFilter)
		if err != nil {
			return nil, fmt.Errorf("could not get network identities: %w", err)
		}

		// creates topology, topology manager, and subscription managers
		//
		// topology
		// subscription manager
		subscriptionManager := p2p.NewChannelSubscriptionManager(fnb.Middleware)
		top, err := topology.NewTopicBasedTopology(fnb.NodeID, fnb.Logger, fnb.State)
		if err != nil {
			return nil, fmt.Errorf("could not create topology: %w", err)
		}
		topologyCache := topology.NewCache(fnb.Logger, top)

		// creates network instance
		net, err := p2p.NewNetwork(fnb.Logger,
			codec,
			participants,
			fnb.Me,
			fnb.Middleware,
			10e6,
			topologyCache,
			subscriptionManager,
			fnb.Metrics.Network)
		if err != nil {
			return nil, fmt.Errorf("could not initialize network: %w", err)
		}

		fnb.Network = net

		idRefresher := p2p.NewNodeIDRefresher(fnb.Logger, fnb.State, net.SetIDs)
		idEvents := gadgets.NewIdentityDeltas(idRefresher.OnIdentityTableChanged)
		fnb.ProtocolEvents.AddConsumer(idEvents)

		return net, err
	})
}

func (fnb *FlowNodeBuilder) enqueueMetricsServerInit() {
	fnb.Component("metrics server", func(builder *FlowNodeBuilder) (module.ReadyDoneAware, error) {
		server := metrics.NewServer(fnb.Logger, fnb.BaseConfig.metricsPort, fnb.BaseConfig.profilerEnabled)
		return server, nil
	})
}

func (fnb *FlowNodeBuilder) registerBadgerMetrics() {
	metrics.RegisterBadgerMetrics()
}

func (fnb *FlowNodeBuilder) enqueueTracer() {
	fnb.Component("tracer", func(builder *FlowNodeBuilder) (module.ReadyDoneAware, error) {
		return fnb.Tracer, nil
	})
}

func (fnb *FlowNodeBuilder) parseAndPrintFlags() {
	// parse configuration parameters
	pflag.Parse()

	// print all flags
	log := fnb.Logger.Info()

	pflag.VisitAll(func(flag *pflag.Flag) {
		log = log.Str(flag.Name, flag.Value.String())
	})

	log.Msg("flags loaded")
}

func (fnb *FlowNodeBuilder) printBuildVersionDetails() {
	fnb.Logger.Info().Str("version", build.Semver()).Str("commit", build.Commit()).Msg("build details")
}

func (fnb *FlowNodeBuilder) initNodeInfo() {
	if fnb.BaseConfig.nodeIDHex == notSet {
		fnb.Logger.Fatal().Msg("cannot start without node ID")
	}

	nodeID, err := flow.HexStringToIdentifier(fnb.BaseConfig.nodeIDHex)
	if err != nil {
		fnb.Logger.Fatal().Err(err).Msgf("could not parse node ID from string: %v", fnb.BaseConfig.nodeIDHex)
	}

	info, err := loadPrivateNodeInfo(fnb.BaseConfig.BootstrapDir, nodeID)
	if err != nil {
		fnb.Logger.Fatal().Err(err).Msg("failed to load private node info")
	}

	fnb.NodeID = nodeID
	fnb.stakingKey = info.StakingPrivKey.PrivateKey
	fnb.NetworkKey = info.NetworkPrivKey.PrivateKey
}

func (fnb *FlowNodeBuilder) initLogger() {
	// configure logger with standard level, node ID and UTC timestamp
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := fnb.Logger.With().
		Timestamp().
		Str("node_role", fnb.BaseConfig.nodeRole).
		Str("node_id", fnb.BaseConfig.nodeIDHex).
		Logger()

	log.Info().Msgf("flow %s node starting up", fnb.BaseConfig.nodeRole)

	// parse config log level and apply to logger
	lvl, err := zerolog.ParseLevel(strings.ToLower(fnb.BaseConfig.level))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log = log.Level(lvl)

	fnb.Logger = log
}

func (fnb *FlowNodeBuilder) initMetrics() {

	fnb.Tracer = trace.NewNoopTracer()
	if fnb.BaseConfig.tracerEnabled {
		tracer, err := trace.NewTracer(fnb.Logger, fnb.BaseConfig.nodeRole)
		fnb.MustNot(err).Msg("could not initialize tracer")
		fnb.Logger.Info().Msg("Tracer Started")
		fnb.Tracer = tracer
	}
	fnb.MetricsRegisterer = prometheus.DefaultRegisterer

	mempools := metrics.NewMempoolCollector(5 * time.Second)

	fnb.Metrics = Metrics{
		Network:    metrics.NewNetworkCollector(),
		Engine:     metrics.NewEngineCollector(),
		Compliance: metrics.NewComplianceCollector(),
		Cache:      metrics.NewCacheCollector(fnb.RootChainID),
		Mempool:    mempools,
	}

	// registers mempools as a Component so that its Ready method is invoked upon startup
	fnb.Component("mempools metrics", func(builder *FlowNodeBuilder) (module.ReadyDoneAware, error) {
		return mempools, nil
	})
}

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
	fnb.Component("profiler", func(node *FlowNodeBuilder) (module.ReadyDoneAware, error) {
		return profiler, nil
	})
}

func (fnb *FlowNodeBuilder) initDB() {
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

	db, err := badger.Open(opts)
	fnb.MustNot(err).Msg("could not open key-value store")
	fnb.DB = db
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
	guarantees := bstorage.NewGuarantees(fnb.Metrics.Cache, fnb.DB)
	seals := bstorage.NewSeals(fnb.Metrics.Cache, fnb.DB)
	results := bstorage.NewExecutionResults(fnb.Metrics.Cache, fnb.DB)
	receipts := bstorage.NewExecutionReceipts(fnb.Metrics.Cache, fnb.DB, results)
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
		)
		fnb.MustNot(err).Msg("could not bootstrap protocol state")

		fnb.Logger.Info().
			Hex("root_result_id", logging.Entity(fnb.RootResult)).
			Hex("root_state_commitment", fnb.RootSeal.FinalState[:]).
			Hex("root_block_id", logging.Entity(fnb.RootBlock)).
			Uint64("root_block_height", fnb.RootBlock.Header.Height).
			Msg("genesis state bootstrapped")
	}

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
	if self.Role.String() != fnb.BaseConfig.nodeRole {
		rootBlockHeader, err := fnb.State.Params().Root()
		fnb.MustNot(err).Msg("could not get root block from protocol state")
		if rootBlockHeader.ChainID == flow.Mainnet {
			fnb.Logger.Fatal().Msgf("running as incorrect role, expected: %v, actual: %v, exiting",
				self.Role.String(),
				fnb.BaseConfig.nodeRole)
		} else {
			fnb.Logger.Warn().Msgf("running as incorrect role, expected: %v, actual: %v, continuing",
				self.Role.String(),
				fnb.BaseConfig.nodeRole)
		}
	}

	// ensure that the configured staking/network keys are consistent with the protocol state
	if !self.NetworkPubKey.Equals(fnb.NetworkKey.PublicKey()) {
		fnb.Logger.Fatal().Msg("configured networking key does not match protocol state")
	}
	if !self.StakingPubKey.Equals(fnb.stakingKey.PublicKey()) {
		fnb.Logger.Fatal().Msg("configured staking key does not match protocol state")
	}

	fnb.Me, err = local.New(self, fnb.stakingKey)
	fnb.MustNot(err).Msg("could not initialize local")

	lastFinalized, err := fnb.State.Final().Head()
	fnb.MustNot(err).Msg("could not get last finalized block header")
	fnb.Logger.Info().
		Hex("block_id", logging.Entity(lastFinalized)).
		Uint64("height", lastFinalized.Height).
		Msg("last finalized block")
}

func (fnb *FlowNodeBuilder) initFvmOptions() {
	blockFinder := fvm.NewBlockFinder(fnb.Storage.Headers)
	vmOpts := []fvm.Option{
		fvm.WithChain(fnb.RootChainID.Chain()),
		fvm.WithBlocks(blockFinder),
		fvm.WithAccountStorageLimit(true),
	}
	if fnb.RootChainID == flow.Testnet {
		vmOpts = append(vmOpts,
			fvm.WithRestrictedDeployment(false),
			fvm.WithTransactionFeesEnabled(true),
		)
	}
	fnb.FvmOptions = vmOpts
}

func (fnb *FlowNodeBuilder) handleModule(v namedModuleFunc) {
	err := v.fn(fnb)
	if err != nil {
		fnb.Logger.Fatal().Err(err).Str("module", v.name).Msg("module initialization failed")
	} else {
		fnb.Logger.Info().Str("module", v.name).Msg("module initialization complete")
	}
}

func (fnb *FlowNodeBuilder) handleComponent(v namedComponentFunc) {

	log := fnb.Logger.With().Str("component", v.name).Logger()

	readyAware, err := v.fn(fnb)
	if err != nil {
		log.Fatal().Err(err).Msg("component initialization failed")
	} else {
		log.Info().Msg("component initialization complete")
	}

	select {
	case <-readyAware.Ready():
		log.Info().Msg("component startup complete")
	case <-time.After(fnb.BaseConfig.timeout):
		log.Fatal().Msg("component startup timed out")
	case <-fnb.sig:
		log.Warn().Msg("component startup aborted")
		os.Exit(1)
	}

	fnb.doneObject = append(fnb.doneObject, namedDoneObject{
		readyAware, v.name,
	})
}

func (fnb *FlowNodeBuilder) handleDoneObject(v namedDoneObject) {

	log := fnb.Logger.With().Str("component", v.name).Logger()

	select {
	case <-v.ob.Done():
		log.Info().Msg("component shutdown complete")
	case <-time.After(fnb.BaseConfig.timeout):
		log.Fatal().Msg("component shutdown timed out")
	case <-fnb.sig:
		log.Warn().Msg("component shutdown aborted")
		os.Exit(1)
	}
}

// ExtraFlags enables binding additional flags beyond those defined in BaseConfig.
func (fnb *FlowNodeBuilder) ExtraFlags(f func(*pflag.FlagSet)) *FlowNodeBuilder {
	f(fnb.flags)
	return fnb
}

// Module enables setting up dependencies of the engine with the builder context.
func (fnb *FlowNodeBuilder) Module(name string, f func(builder *FlowNodeBuilder) error) *FlowNodeBuilder {
	fnb.modules = append(fnb.modules, namedModuleFunc{
		fn:   f,
		name: name,
	})
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

// Component adds a new component to the node that conforms to the ReadyDone
// interface.
//
// When the node is run, this component will be started with `Ready`. When the
// node is stopped, we will wait for the component to exit gracefully with
// `Done`.
func (fnb *FlowNodeBuilder) Component(name string, f func(*FlowNodeBuilder) (module.ReadyDoneAware, error)) *FlowNodeBuilder {
	fnb.components = append(fnb.components, namedComponentFunc{
		fn:   f,
		name: name,
	})

	return fnb
}

func (fnb *FlowNodeBuilder) PostInit(f func(node *FlowNodeBuilder)) *FlowNodeBuilder {
	fnb.postInitFns = append(fnb.postInitFns, f)
	return fnb
}

// FlowNode creates a new Flow node builder with the given name.
func FlowNode(role string) *FlowNodeBuilder {

	builder := &FlowNodeBuilder{
		BaseConfig: BaseConfig{
			nodeRole: role,
		},
		Logger: zerolog.New(os.Stderr),
		flags:  pflag.CommandLine,
	}

	builder.baseFlags()

	builder.enqueueNetworkInit()

	builder.enqueueMetricsServerInit()

	builder.registerBadgerMetrics()

	builder.enqueueTracer()

	return builder
}

// Run initiates all common components (logger, database, protocol state etc.)
// then starts each component. It also sets up a channel to gracefully shut
// down each component if a SIGINT is received.
func (fnb *FlowNodeBuilder) Run() {

	// initialize signal catcher
	fnb.sig = make(chan os.Signal, 1)
	signal.Notify(fnb.sig, os.Interrupt, syscall.SIGTERM)

	fnb.printBuildVersionDetails()

	fnb.parseAndPrintFlags()

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	fnb.initNodeInfo()

	fnb.initLogger()

	fnb.initProfiler()

	fnb.initDB()

	fnb.initMetrics()

	fnb.initStorage()

	fnb.initState()

	fnb.initFvmOptions()

	for _, f := range fnb.postInitFns {
		fnb.handlePostInit(f)
	}

	// set up all modules
	for _, f := range fnb.modules {
		fnb.handleModule(f)
	}

	// initialize all components
	for _, f := range fnb.components {
		fnb.handleComponent(f)
	}

	fnb.Logger.Info().Msgf("%s node startup complete", fnb.BaseConfig.nodeRole)

	<-fnb.sig

	fnb.Logger.Info().Msgf("%s node shutting down", fnb.BaseConfig.nodeRole)

	for i := len(fnb.doneObject) - 1; i >= 0; i-- {
		doneObject := fnb.doneObject[i]

		fnb.handleDoneObject(doneObject)
	}

	fnb.closeDatabase()

	fnb.Logger.Info().Msgf("%s node shutdown complete", fnb.BaseConfig.nodeRole)

	os.Exit(0)
}

func (fnb *FlowNodeBuilder) handlePostInit(f func(node *FlowNodeBuilder)) {
	f(fnb)
}

func (fnb *FlowNodeBuilder) closeDatabase() {
	err := fnb.DB.Close()
	if err != nil {
		fnb.Logger.Error().
			Err(err).
			Msg("could not close database")
	}
}

// loadRootProtocolSnapshot loads the root protocol snapshot from disk
func loadRootProtocolSnapshot(dir string) (*inmem.Snapshot, error) {
	data, err := io.ReadFile(filepath.Join(dir, bootstrap.PathRootProtocolStateSnapshot))
	if err != nil {
		return nil, err
	}

	var snapshot inmem.EncodableSnapshot
	err = json.Unmarshal(data, &snapshot)
	if err != nil {
		return nil, err
	}

	return inmem.SnapshotFromEncodable(snapshot), nil
}

// Loads the private info for this node from disk (eg. private staking/network keys).
func loadPrivateNodeInfo(dir string, myID flow.Identifier) (*bootstrap.NodeInfoPriv, error) {
	data, err := io.ReadFile(filepath.Join(dir, fmt.Sprintf(bootstrap.PathNodeInfoPriv, myID)))
	if err != nil {
		return nil, err
	}
	var info bootstrap.NodeInfoPriv
	err = json.Unmarshal(data, &info)
	return &info, err
}
