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

const NotSet = "not set"

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
	fn   func(NodeBuilder) error
	name string
}

type namedComponentFunc struct {
	fn   func(NodeBuilder) (module.ReadyDoneAware, error)
	name string
}

type namedDoneObject struct {
	ob   module.ReadyDoneAware
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
	BaseConfig        BaseConfig
	nodeID            flow.Identifier
	flags             *pflag.FlagSet
	logger            zerolog.Logger
	me                *local.Local
	tracer            module.Tracer
	metricsRegisterer prometheus.Registerer
	metrics           Metrics
	db                *badger.DB
	storage           Storage
	protocolEvents    *events.Distributor
	state             protocol.State
	middleware        *p2p.Middleware
	network           *p2p.Network
	msgValidators     []network.MessageValidator
	fvmOptions        []fvm.Option
	modules           []namedModuleFunc
	components        []namedComponentFunc
	doneObject        []namedDoneObject
	sig               chan os.Signal
	preInitFns        []func(NodeBuilder)
	postInitFns       []func(NodeBuilder)
	stakingKey        crypto.PrivateKey
	networkKey        crypto.PrivateKey

	// root state information
	rootBlock   *flow.Block
	rootQC      *flow.QuorumCertificate
	rootResult  *flow.ExecutionResult
	rootSeal    *flow.Seal
	rootChainID flow.ChainID
}

func (fnb *FlowNodeBuilder) Config() BaseConfig {
	return fnb.BaseConfig
}

func (fnb *FlowNodeBuilder) NodeID() flow.Identifier {
	return fnb.nodeID
}

func (fnb *FlowNodeBuilder) Logger() zerolog.Logger {
	return fnb.logger
}

func (fnb *FlowNodeBuilder) Me() *local.Local {
	return fnb.me
}

func (fnb *FlowNodeBuilder) SetMe(me *local.Local) {
	fnb.me = me
}

func (fnb *FlowNodeBuilder) Tracer() module.Tracer {
	return fnb.tracer
}

func (fnb *FlowNodeBuilder) MetricsRegisterer() prometheus.Registerer {
	return fnb.metricsRegisterer
}

func (fnb *FlowNodeBuilder) Metrics() Metrics {
	return fnb.metrics
}

func (fnb *FlowNodeBuilder) DB() *badger.DB {
	return fnb.db
}

func (fnb *FlowNodeBuilder) Storage() Storage {
	return fnb.storage
}

func (fnb *FlowNodeBuilder) ProtocolEvents() *events.Distributor {
	return fnb.protocolEvents
}

func (fnb *FlowNodeBuilder) ProtocolState() protocol.State {
	return fnb.state
}

func (fnb *FlowNodeBuilder) Middleware() *p2p.Middleware {
	return fnb.middleware
}

func (fnb *FlowNodeBuilder) SetMiddleware(m *p2p.Middleware) {
	fnb.middleware = m
}

func (fnb *FlowNodeBuilder) Network() *p2p.Network {
	return fnb.network
}

func (fnb *FlowNodeBuilder) SetNetwork(n *p2p.Network) {
	fnb.network = n
}

func (fnb *FlowNodeBuilder) MsgValidators() []network.MessageValidator {
	return fnb.msgValidators
}

func (fnb *FlowNodeBuilder) SetMsgValidators(validators []network.MessageValidator) {
	fnb.msgValidators = validators
}

func (fnb *FlowNodeBuilder) FvmOptions() []fvm.Option {
	return fnb.fvmOptions
}

func (fnb *FlowNodeBuilder) NetworkKey() crypto.PrivateKey {
	return fnb.networkKey
}

func (fnb *FlowNodeBuilder) RootBlock() *flow.Block {
	return fnb.rootBlock
}

func (fnb *FlowNodeBuilder) RootQC() *flow.QuorumCertificate {
	return fnb.rootQC
}

func (fnb *FlowNodeBuilder) RootSeal() *flow.Seal {
	return fnb.rootSeal
}

func (fnb *FlowNodeBuilder) RootChainID() flow.ChainID {
	return fnb.rootChainID
}

func (fnb *FlowNodeBuilder) BaseFlags() {
	homedir, _ := os.UserHomeDir()
	datadir := filepath.Join(homedir, ".flow", "database")
	// bind configuration parameters
	fnb.flags.StringVar(&fnb.BaseConfig.nodeIDHex, "nodeid", NotSet, "identity of our node")
	fnb.flags.StringVar(&fnb.BaseConfig.bindAddr, "bind", NotSet, "address to bind on")
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

func (fnb *FlowNodeBuilder) EnqueueNetworkInit() {
	fnb.Component("network", func(builder NodeBuilder) (module.ReadyDoneAware, error) {

		codec := jsoncodec.NewCodec()

		myAddr := fnb.me.Address()
		if fnb.BaseConfig.bindAddr != NotSet {
			myAddr = fnb.BaseConfig.bindAddr
		}

		// setup the Ping provider to return the software version and the sealed block height
		pingProvider := p2p.PingInfoProviderImpl{
			SoftwareVersionFun: func() string {
				return build.Semver()
			},
			SealedBlockHeightFun: func() (uint64, error) {
				head, err := fnb.state.Sealed().Head()
				if err != nil {
					return 0, err
				}
				return head.Height, nil
			},
		}

		libP2PNodeFactory, err := p2p.DefaultLibP2PNodeFactory(fnb.logger.Level(zerolog.ErrorLevel),
			fnb.me.NodeID(),
			myAddr,
			fnb.networkKey,
			fnb.rootBlock.ID().String(),
			p2p.DefaultMaxPubSubMsgSize,
			fnb.metrics.Network,
			pingProvider)
		if err != nil {
			return nil, fmt.Errorf("could not generate libp2p node factory: %w", err)
		}

		fnb.middleware = p2p.NewMiddleware(fnb.logger.Level(zerolog.ErrorLevel),
			libP2PNodeFactory,
			fnb.me.NodeID(),
			fnb.metrics.Network,
			fnb.rootBlock.ID().String(),
			fnb.BaseConfig.peerUpdateInterval,
			fnb.BaseConfig.unicastMessageTimeout,
			fnb.msgValidators...)

		participants, err := fnb.state.Final().Identities(p2p.NetworkingSetFilter)
		if err != nil {
			return nil, fmt.Errorf("could not get network identities: %w", err)
		}

		subscriptionManager := p2p.NewChannelSubscriptionManager(fnb.middleware)
		top, err := topology.NewTopicBasedTopology(fnb.nodeID, fnb.logger, fnb.state)
		if err != nil {
			return nil, fmt.Errorf("could not create topology: %w", err)
		}
		topologyCache := topology.NewCache(fnb.logger, top)

		// creates network instance
		net, err := p2p.NewNetwork(fnb.logger,
			codec,
			participants,
			fnb.me,
			fnb.middleware,
			p2p.DefaultCacheSize,
			topologyCache,
			subscriptionManager,
			fnb.metrics.Network)
		if err != nil {
			return nil, fmt.Errorf("could not initialize network: %w", err)
		}

		fnb.network = net

		idRefresher := p2p.NewNodeIDRefresher(fnb.logger, fnb.state, net.SetIDs)
		idEvents := gadgets.NewIdentityDeltas(idRefresher.OnIdentityTableChanged)
		fnb.protocolEvents.AddConsumer(idEvents)

		return net, err
	})
}

func (fnb *FlowNodeBuilder) EnqueueMetricsServerInit() {
	fnb.Component("metrics server", func(builder NodeBuilder) (module.ReadyDoneAware, error) {
		server := metrics.NewServer(fnb.logger, fnb.BaseConfig.metricsPort, fnb.BaseConfig.profilerEnabled)
		return server, nil
	})
}

func (fnb *FlowNodeBuilder) RegisterBadgerMetrics() {
	metrics.RegisterBadgerMetrics()
}

func (fnb *FlowNodeBuilder) EnqueueTracer() {
	fnb.Component("tracer", func(builder NodeBuilder) (module.ReadyDoneAware, error) {
		return fnb.tracer, nil
	})
}

func (fnb *FlowNodeBuilder) ParseAndPrintFlags() {
	// parse configuration parameters
	pflag.Parse()

	// print all flags
	log := fnb.logger.Info()

	pflag.VisitAll(func(flag *pflag.Flag) {
		log = log.Str(flag.Name, flag.Value.String())
	})

	log.Msg("flags loaded")
}

func (fnb *FlowNodeBuilder) PrintBuildVersionDetails() {
	fnb.logger.Info().Str("version", build.Semver()).Str("commit", build.Commit()).Msg("build details")
}

func (fnb *FlowNodeBuilder) initNodeInfo() {
	if fnb.BaseConfig.nodeIDHex == NotSet {
		fnb.logger.Fatal().Msg("cannot start without node ID")
	}

	nodeID, err := flow.HexStringToIdentifier(fnb.BaseConfig.nodeIDHex)
	if err != nil {
		fnb.logger.Fatal().Err(err).Msgf("could not parse node ID from string: %v", fnb.BaseConfig.nodeIDHex)
	}

	info, err := loadPrivateNodeInfo(fnb.BaseConfig.BootstrapDir, nodeID)
	if err != nil {
		fnb.logger.Fatal().Err(err).Msg("failed to load private node info")
	}

	fnb.nodeID = nodeID
	fnb.networkKey = info.NetworkPrivKey.PrivateKey
	fnb.stakingKey = info.StakingPrivKey.PrivateKey
}

func (fnb *FlowNodeBuilder) initLogger() {
	// configure logger with standard level, node ID and UTC timestamp
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := fnb.logger.With().
		Timestamp().
		Str("node_role", fnb.BaseConfig.NodeRole).
		Str("node_id", fnb.BaseConfig.nodeIDHex).
		Logger()

	log.Info().Msgf("flow %s node starting up", fnb.BaseConfig.NodeRole)

	// parse config log level and apply to logger
	lvl, err := zerolog.ParseLevel(strings.ToLower(fnb.BaseConfig.level))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log = log.Level(lvl)

	fnb.logger = log
}

func (fnb *FlowNodeBuilder) initMetrics() {

	fnb.tracer = trace.NewNoopTracer()
	if fnb.BaseConfig.tracerEnabled {
		tracer, err := trace.NewTracer(fnb.logger, fnb.BaseConfig.NodeRole)
		fnb.MustNot(err).Msg("could not initialize tracer")
		fnb.logger.Info().Msg("Tracer Started")
		fnb.tracer = tracer
	}
	fnb.metricsRegisterer = prometheus.DefaultRegisterer

	mempools := metrics.NewMempoolCollector(5 * time.Second)

	fnb.metrics = Metrics{
		Network:    metrics.NewNetworkCollector(),
		Engine:     metrics.NewEngineCollector(),
		Compliance: metrics.NewComplianceCollector(),
		Cache:      metrics.NewCacheCollector(fnb.rootChainID),
		Mempool:    mempools,
	}

	// registers mempools as a Component so that its Ready method is invoked upon startup
	fnb.Component("mempools metrics", func(builder NodeBuilder) (module.ReadyDoneAware, error) {
		return mempools, nil
	})
}

func (fnb *FlowNodeBuilder) initProfiler() {
	if !fnb.BaseConfig.profilerEnabled {
		return
	}
	profiler, err := debug.NewAutoProfiler(
		fnb.logger,
		fnb.BaseConfig.profilerDir,
		fnb.BaseConfig.profilerInterval,
		fnb.BaseConfig.profilerDuration,
	)
	fnb.MustNot(err).Msg("could not initialize profiler")
	fnb.Component("profiler", func(node NodeBuilder) (module.ReadyDoneAware, error) {
		return profiler, nil
	})
}

func (fnb *FlowNodeBuilder) initDB() {
	// Pre-create DB path (Badger creates only one-level dirs)
	err := os.MkdirAll(fnb.BaseConfig.datadir, 0700)
	fnb.MustNot(err).Str("dir", fnb.BaseConfig.datadir).Msg("could not create datadir")

	log := sutil.NewLogger(fnb.logger)

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
	fnb.db = db
}

func (fnb *FlowNodeBuilder) initStorage() {

	// in order to void long iterations with big keys when initializing with an
	// already populated database, we bootstrap the initial maximum key size
	// upon starting
	err := operation.RetryOnConflict(fnb.db.Update, func(tx *badger.Txn) error {
		return operation.InitMax(tx)
	})
	fnb.MustNot(err).Msg("could not initialize max tracker")

	headers := bstorage.NewHeaders(fnb.metrics.Cache, fnb.db)
	guarantees := bstorage.NewGuarantees(fnb.metrics.Cache, fnb.db)
	seals := bstorage.NewSeals(fnb.metrics.Cache, fnb.db)
	results := bstorage.NewExecutionResults(fnb.metrics.Cache, fnb.db)
	receipts := bstorage.NewExecutionReceipts(fnb.metrics.Cache, fnb.db, results)
	index := bstorage.NewIndex(fnb.metrics.Cache, fnb.db)
	payloads := bstorage.NewPayloads(fnb.db, index, guarantees, seals, receipts, results)
	blocks := bstorage.NewBlocks(fnb.db, headers, payloads)
	transactions := bstorage.NewTransactions(fnb.metrics.Cache, fnb.db)
	collections := bstorage.NewCollections(fnb.db, transactions)
	setups := bstorage.NewEpochSetups(fnb.metrics.Cache, fnb.db)
	commits := bstorage.NewEpochCommits(fnb.metrics.Cache, fnb.db)
	statuses := bstorage.NewEpochStatuses(fnb.metrics.Cache, fnb.db)

	fnb.storage = Storage{
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
	fnb.protocolEvents = events.NewDistributor()

	// load the root protocol state snapshot from disk
	rootSnapshot, err := loadRootProtocolSnapshot(fnb.BaseConfig.BootstrapDir)
	fnb.MustNot(err).Msg("failed to read protocol snapshot from disk")

	fnb.rootResult, fnb.rootSeal, err = rootSnapshot.SealedResult()
	fnb.MustNot(err).Msg("failed to read root sealed result")
	sealingSegment, err := rootSnapshot.SealingSegment()
	fnb.MustNot(err).Msg("failed to read root sealing segment")
	fnb.rootBlock = sealingSegment[len(sealingSegment)-1]
	fnb.rootQC, err = rootSnapshot.QuorumCertificate()
	fnb.MustNot(err).Msg("failed to read root qc")
	// set the chain ID based on the root header
	// TODO: as the root header can now be loaded from protocol state, we should
	// not use a global variable for chain ID anymore, but rely on the protocol
	// state as final authority on what the chain ID is
	// => https://github.com/dapperlabs/flow-go/issues/4167
	fnb.rootChainID = fnb.rootBlock.Header.ChainID

	isBootStrapped, err := badgerState.IsBootstrapped(fnb.db)
	fnb.MustNot(err).Msg("failed to determine whether database contains bootstrapped state")
	if isBootStrapped {
		state, err := badgerState.OpenState(
			fnb.metrics.Compliance,
			fnb.db,
			fnb.storage.Headers,
			fnb.storage.Seals,
			fnb.storage.Results,
			fnb.storage.Blocks,
			fnb.storage.Setups,
			fnb.storage.Commits,
			fnb.storage.Statuses,
		)
		fnb.MustNot(err).Msg("could not open flow state")
		fnb.state = state

		// Verify root block in protocol state is consistent with bootstrap information stored on-disk.
		// Inconsistencies can happen when the bootstrap root block is updated (because of new spork),
		// but the protocol state is not updated, so they don't match
		// when this happens during a spork, we could try deleting the protocol state database.
		// TODO: revisit this check when implementing Epoch
		rootBlockFromState, err := state.Params().Root()
		fnb.MustNot(err).Msg("could not load root block from protocol state")
		if fnb.rootBlock.ID() != rootBlockFromState.ID() {
			fnb.logger.Fatal().Msgf("mismatching root block ID, protocol state block ID: %v, bootstrap root block ID: %v",
				rootBlockFromState.ID(),
				fnb.rootBlock.ID(),
			)
		}
	} else {
		// Bootstrap!
		fnb.logger.Info().Msg("bootstrapping empty protocol state")

		fnb.state, err = badgerState.Bootstrap(
			fnb.metrics.Compliance,
			fnb.db,
			fnb.storage.Headers,
			fnb.storage.Seals,
			fnb.storage.Results,
			fnb.storage.Blocks,
			fnb.storage.Setups,
			fnb.storage.Commits,
			fnb.storage.Statuses,
			rootSnapshot,
		)
		fnb.MustNot(err).Msg("could not bootstrap protocol state")

		fnb.logger.Info().
			Hex("root_result_id", logging.Entity(fnb.rootResult)).
			Hex("root_state_commitment", fnb.rootSeal.FinalState[:]).
			Hex("root_block_id", logging.Entity(fnb.rootBlock)).
			Uint64("root_block_height", fnb.rootBlock.Header.Height).
			Msg("genesis state bootstrapped")
	}

	// initialize local if it hasn't been initialized yet
	if fnb.Me() == nil {
		fnb.initLocal()
	}

	lastFinalized, err := fnb.state.Final().Head()
	fnb.MustNot(err).Msg("could not get last finalized block header")
	fnb.logger.Info().
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

	self, err := fnb.state.Final().Identity(myID)
	fnb.MustNot(err).Msgf("node identity not found in the identity list of the finalized state: %v", myID)

	// Verify that my role (as given in the configuration) is consistent with the protocol state.
	// We enforce this strictly for MainNet. For other networks (e.g. TestNet or BenchNet), we
	// are lenient, to allow ghost node to run as any role.
	if self.Role.String() != fnb.BaseConfig.NodeRole {
		rootBlockHeader, err := fnb.state.Params().Root()
		fnb.MustNot(err).Msg("could not get root block from protocol state")
		if rootBlockHeader.ChainID == flow.Mainnet {
			fnb.logger.Fatal().Msgf("running as incorrect role, expected: %v, actual: %v, exiting",
				self.Role.String(),
				fnb.BaseConfig.NodeRole)
		} else {
			fnb.logger.Warn().Msgf("running as incorrect role, expected: %v, actual: %v, continuing",
				self.Role.String(),
				fnb.BaseConfig.NodeRole)
		}
	}

	// ensure that the configured staking/network keys are consistent with the protocol state
	if !self.NetworkPubKey.Equals(fnb.networkKey.PublicKey()) {
		fnb.logger.Fatal().Msg("configured networking key does not match protocol state")
	}
	if !self.StakingPubKey.Equals(fnb.stakingKey.PublicKey()) {
		fnb.logger.Fatal().Msg("configured staking key does not match protocol state")
	}

	fnb.me, err = local.New(self, fnb.stakingKey)
	fnb.MustNot(err).Msg("could not initialize local")
}

func (fnb *FlowNodeBuilder) initFvmOptions() {
	blockFinder := fvm.NewBlockFinder(fnb.storage.Headers)
	vmOpts := []fvm.Option{
		fvm.WithChain(fnb.rootChainID.Chain()),
		fvm.WithBlocks(blockFinder),
		fvm.WithAccountStorageLimit(true),
	}
	if fnb.rootChainID == flow.Testnet {
		vmOpts = append(vmOpts,
			fvm.WithRestrictedDeployment(false),
			fvm.WithTransactionFeesEnabled(true),
		)
	}
	fnb.fvmOptions = vmOpts
}

func (fnb *FlowNodeBuilder) handleModule(v namedModuleFunc) {
	err := v.fn(fnb)
	if err != nil {
		fnb.logger.Fatal().Err(err).Str("module", v.name).Msg("module initialization failed")
	} else {
		fnb.logger.Info().Str("module", v.name).Msg("module initialization complete")
	}
}

func (fnb *FlowNodeBuilder) handleComponent(v namedComponentFunc) {

	log := fnb.logger.With().Str("component", v.name).Logger()

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

	log := fnb.logger.With().Str("component", v.name).Logger()

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
func (fnb *FlowNodeBuilder) ExtraFlags(f func(*pflag.FlagSet)) NodeBuilder {
	f(fnb.flags)
	return fnb
}

// Module enables setting up dependencies of the engine with the builder context.
func (fnb *FlowNodeBuilder) Module(name string, f func(builder NodeBuilder) error) NodeBuilder {
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
		return fnb.logger.Fatal().Err(err)
	}
	return nil
}

// Component adds a new component to the node that conforms to the ReadyDone
// interface.
//
// When the node is run, this component will be started with `Ready`. When the
// node is stopped, we will wait for the component to exit gracefully with
// `Done`.
func (fnb *FlowNodeBuilder) Component(name string, f func(NodeBuilder) (module.ReadyDoneAware, error)) NodeBuilder {
	fnb.components = append(fnb.components, namedComponentFunc{
		fn:   f,
		name: name,
	})

	return fnb
}

func (fnb *FlowNodeBuilder) PreInit(f func(node NodeBuilder)) NodeBuilder {
	fnb.preInitFns = append(fnb.preInitFns, f)
	return fnb
}

func (fnb *FlowNodeBuilder) PostInit(f func(node NodeBuilder)) NodeBuilder {
	fnb.postInitFns = append(fnb.postInitFns, f)
	return fnb
}

// FlowNode creates a new Flow node builder with the given name.
func FlowNode(role string) *FlowNodeBuilder {

	builder := &FlowNodeBuilder{
		BaseConfig: BaseConfig{
			NodeRole: role,
		},
		logger: zerolog.New(os.Stderr),
		flags:  pflag.CommandLine,
	}

	return builder
}

func (fnb *FlowNodeBuilder) Initialize() NodeBuilder {

	fnb.PrintBuildVersionDetails()

	fnb.BaseFlags()

	fnb.ParseAndPrintFlags()

	fnb.EnqueueNetworkInit()

	fnb.EnqueueMetricsServerInit()

	fnb.RegisterBadgerMetrics()

	fnb.EnqueueTracer()

	return fnb
}

// Run initiates all common components (logger, database, protocol state etc.)
// then starts each component. It also sets up a channel to gracefully shut
// down each component if a SIGINT is received.
func (fnb *FlowNodeBuilder) Run() {

	// initialize signal catcher
	fnb.sig = make(chan os.Signal, 1)
	signal.Notify(fnb.sig, os.Interrupt, syscall.SIGTERM)

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	fnb.initNodeInfo()

	fnb.initLogger()

	fnb.initProfiler()

	fnb.initDB()

	fnb.initMetrics()

	fnb.initStorage()

	for _, f := range fnb.preInitFns {
		fnb.handlePreInit(f)
	}

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

	fnb.logger.Info().Msgf("%s node startup complete", fnb.BaseConfig.NodeRole)

	<-fnb.sig

	fnb.logger.Info().Msgf("%s node shutting down", fnb.BaseConfig.NodeRole)

	for i := len(fnb.doneObject) - 1; i >= 0; i-- {
		doneObject := fnb.doneObject[i]

		fnb.handleDoneObject(doneObject)
	}

	fnb.closeDatabase()

	fnb.logger.Info().Msgf("%s node shutdown complete", fnb.BaseConfig.NodeRole)

	os.Exit(0)
}

func (fnb *FlowNodeBuilder) handlePreInit(f func(node NodeBuilder)) {
	f(fnb)
}

func (fnb *FlowNodeBuilder) handlePostInit(f func(node NodeBuilder)) {
	f(fnb)
}

func (fnb *FlowNodeBuilder) closeDatabase() {
	err := fnb.db.Close()
	if err != nil {
		fnb.logger.Error().
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
