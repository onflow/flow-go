package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/dkg"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/trace"
	jsoncodec "github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	"github.com/dapperlabs/flow-go/state/dkg/wrapper"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

const notSet = "not set"

// BaseConfig is the general config for the FlowNodeBuilder
type BaseConfig struct {
	nodeIDHex    string
	NodeName     string
	Timeout      time.Duration
	datadir      string
	level        string
	metricsPort  uint
	nClusters    uint
	bootstrapDir string
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
//   GenesisHandler, if any and if genesis was generate
// Components handlers, if any, wait sequentially
// Run() <- main loop
// Components destructors, if any
type FlowNodeBuilder struct {
	BaseConfig     BaseConfig
	NodeID         flow.Identifier
	flags          *pflag.FlagSet
	name           string
	Logger         zerolog.Logger
	Tracer         trace.Tracer
	DB             *badger.DB
	Me             *local.Local
	State          *protocol.State
	DKGState       *wrapper.State
	modules        []namedModuleFunc
	components     []namedComponentFunc
	doneObject     []namedDoneObject
	sig            chan os.Signal
	Network        *libp2p.Network
	genesisHandler func(node *FlowNodeBuilder, block *flow.Block)
	postInitFns    []func(*FlowNodeBuilder)
	stakingKey     crypto.PrivateKey
	networkKey     crypto.PrivateKey

	// genesis information
	GenesisBlock *flow.Block
	GenesisQC    *model.QuorumCertificate
}

func (fnb *FlowNodeBuilder) baseFlags() {
	homedir, _ := os.UserHomeDir()
	datadir := filepath.Join(homedir, ".flow", "database")
	// bind configuration parameters
	fnb.flags.StringVar(&fnb.BaseConfig.nodeIDHex, "nodeid", notSet, "identity of our node")
	fnb.flags.StringVarP(&fnb.BaseConfig.NodeName, "nodename", "n", "node1", "identity of our node")
	fnb.flags.StringVarP(&fnb.BaseConfig.bootstrapDir, "bootstrapdir", "b", "./bootstrap", "path to the bootstrap directory")
	fnb.flags.DurationVarP(&fnb.BaseConfig.Timeout, "timeout", "t", 1*time.Minute, "how long to try connecting to the network")
	fnb.flags.StringVarP(&fnb.BaseConfig.datadir, "datadir", "d", datadir, "directory to store the protocol State")
	fnb.flags.StringVarP(&fnb.BaseConfig.level, "loglevel", "l", "info", "level for logging output")
	fnb.flags.UintVarP(&fnb.BaseConfig.metricsPort, "metricport", "m", 8080, "port for /metrics endpoint")
	fnb.flags.UintVar(&fnb.BaseConfig.nClusters, "nclusters", 2, "number of collection node clusters")
}

func (fnb *FlowNodeBuilder) enqueueNetworkInit() {
	fnb.Component("network", func(builder *FlowNodeBuilder) (module.ReadyDoneAware, error) {

		codec := jsoncodec.NewCodec()

		mw, err := libp2p.NewMiddleware(fnb.Logger.Level(zerolog.ErrorLevel), codec, fnb.Me.Address(), fnb.Me.NodeID(), fnb.networkKey)
		if err != nil {
			return nil, fmt.Errorf("could not initialize middleware: %w", err)
		}

		ids, err := fnb.State.Final().Identities()
		if err != nil {
			return nil, fmt.Errorf("could not get network identities: %w", err)
		}

		net, err := libp2p.NewNetwork(fnb.Logger, codec, ids, fnb.Me, mw, 10e6, libp2p.NewRandPermTopology())
		if err != nil {
			return nil, fmt.Errorf("could not initialize network: %w", err)
		}

		fnb.Network = net
		return net, err
	})
}

func (fnb *FlowNodeBuilder) enqueueMetricsServerInit() {
	fnb.Component("metrics", func(builder *FlowNodeBuilder) (module.ReadyDoneAware, error) {
		server := metrics.NewServer(fnb.Logger, fnb.BaseConfig.metricsPort)
		return server, nil
	})
}

func (fnb *FlowNodeBuilder) initNodeInfo() {
	if fnb.BaseConfig.nodeIDHex == notSet {
		fnb.Logger.Fatal().Msg("cannot start without node ID")
	}

	nodeID, err := flow.HexStringToIdentifier(fnb.BaseConfig.nodeIDHex)
	if err != nil {
		fnb.Logger.Fatal().Err(err).Msg("could not parse hex ID")
	}

	info, err := loadPrivateNodeInfo(fnb.BaseConfig.bootstrapDir, nodeID)
	if err != nil {
		fnb.Logger.Fatal().Err(err).Msg("failed to load private node info")
	}

	fnb.NodeID = nodeID
	fnb.stakingKey = info.StakingPrivKey.PrivateKey
	fnb.networkKey = info.NetworkPrivKey.PrivateKey
}

func (fnb *FlowNodeBuilder) initLogger() {
	// configure logger with standard level, node ID and UTC timestamp
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := fnb.Logger.With().Timestamp().Str("node_id", fnb.BaseConfig.nodeIDHex).Logger()

	log.Info().Msgf("flow %s node starting up", fnb.name)

	// parse config log level and apply to logger
	lvl, err := zerolog.ParseLevel(strings.ToLower(fnb.BaseConfig.level))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log.Level(lvl)

	log.Info().Msg("initializing engine modules")

	fnb.Logger = log
}

func (fnb *FlowNodeBuilder) initDatabase() {
	//Pre-create DB path (Badger creates only one-level dirs)
	err := os.MkdirAll(fnb.BaseConfig.datadir, 0700)
	fnb.MustNot(err).Msgf("could not create datadir %s", fnb.BaseConfig.datadir)

	opts := badger.DefaultOptions(fnb.BaseConfig.datadir).WithLogger(nil)
	db, err := badger.Open(opts)
	fnb.MustNot(err).Msg("could not open key-value store")
	fnb.DB = db
}

func (fnb *FlowNodeBuilder) initTracer() {
	tracer, err := trace.NewTracer(fnb.Logger)
	fnb.MustNot(err).Msg("could not initialize tracer")
	fnb.Tracer = tracer
}

func (fnb *FlowNodeBuilder) initState() {
	state, err := protocol.NewState(fnb.DB, protocol.SetClusters(fnb.BaseConfig.nClusters))
	fnb.MustNot(err).Msg("could not initialize flow state")

	// check if database is initialized
	head, err := state.Final().Head()
	if errors.Is(err, storage.ErrNotFound) {
		// Bootstrap!

		fnb.Logger.Info().Msg("bootstrapping empty database")

		// Load the rest of the genesis info, eventually needed for the consensus follower
		fnb.GenesisBlock, err = loadTrustedRootBlock(fnb.BaseConfig.bootstrapDir)
		if err != nil {
			fnb.Logger.Fatal().Err(err).Msg("could not bootstrap, reading genesis header")
		}

		// load genesis QC and DKG data from bootstrap files
		fnb.GenesisQC, err = loadRootBlockQC(fnb.BaseConfig.bootstrapDir)
		if err != nil {
			fnb.Logger.Fatal().Err(err).Msg("could not bootstrap, reading root block sigs")
		}

		dkgPubData, err := loadDKGPublicData(fnb.BaseConfig.bootstrapDir)
		if err != nil {
			fnb.Logger.Fatal().Err(err).Msg("could not bootstrap, reading dkg public data")
		}
		fnb.DKGState = wrapper.NewState(dkgPubData)

		err = state.Mutate().Bootstrap(fnb.GenesisBlock)
		if err != nil {
			fnb.Logger.Fatal().Err(err).Msg("could not bootstrap protocol state")
		}
	} else if err != nil {
		fnb.Logger.Fatal().Err(err).Msg("could not check existing database")
	} else {
		fnb.Logger.Info().
			Hex("final_id", logging.ID(head.ID())).
			Uint64("final_height", head.Height).
			Msg("using existing database")
	}

	myID, err := flow.HexStringToIdentifier(fnb.BaseConfig.nodeIDHex)
	fnb.MustNot(err).Msg("could not parse node identifier")

	allIdentities, err := state.Final().Identities()
	fnb.MustNot(err).Msg("could not retrieve finalized identities")
	fnb.Logger.Debug().Msgf("known nodes: %v", allIdentities)

	id, err := state.Final().Identity(myID)
	fnb.MustNot(err).Msg("could not get identity")

	fnb.Me, err = local.New(id, fnb.stakingKey)
	fnb.MustNot(err).Msg("could not initialize local")

	fnb.State = state
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
	case <-time.After(fnb.BaseConfig.Timeout):
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
	case <-time.After(fnb.BaseConfig.Timeout):
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

// GenesisHandler sets up handler which will be executed when a genesis block is generated
func (fnb *FlowNodeBuilder) GenesisHandler(handler func(node *FlowNodeBuilder, block *flow.Block)) *FlowNodeBuilder {
	fnb.genesisHandler = handler
	return fnb
}

func (fnb *FlowNodeBuilder) PostInit(f func(node *FlowNodeBuilder)) *FlowNodeBuilder {
	fnb.postInitFns = append(fnb.postInitFns, f)
	return fnb
}

// FlowNode creates a new Flow node builder with the given name.
func FlowNode(name string) *FlowNodeBuilder {

	builder := &FlowNodeBuilder{
		BaseConfig: BaseConfig{},
		Logger:     zerolog.New(os.Stderr),
		flags:      pflag.CommandLine,
		name:       name,
	}

	builder.baseFlags()

	builder.enqueueNetworkInit()

	builder.enqueueMetricsServerInit()

	return builder
}

// Run initiates all common components (logger, database, protocol state etc.)
// then starts each component. It also sets up a channel to gracefully shut
// down each component if a SIGINT is received.
func (fnb *FlowNodeBuilder) Run() {

	// initialize signal catcher
	fnb.sig = make(chan os.Signal, 1)
	signal.Notify(fnb.sig, os.Interrupt, syscall.SIGTERM)

	// parse configuration parameters
	pflag.Parse()

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	fnb.initNodeInfo()

	fnb.initLogger()

	fnb.initTracer()

	fnb.initDatabase()

	fnb.initState()

	for _, f := range fnb.postInitFns {
		fnb.handlePostInit(f)
	}

	// set up all modules
	for _, f := range fnb.modules {
		fnb.handleModule(f)
	}

	if fnb.GenesisBlock != nil && fnb.genesisHandler != nil {
		fnb.genesisHandler(fnb, fnb.GenesisBlock)
	}

	// initialize all components
	for _, f := range fnb.components {
		fnb.handleComponent(f)
	}

	fnb.Logger.Info().Msgf("%s node startup complete", fnb.name)

	<-fnb.sig

	fnb.Logger.Info().Msgf("%s node shutting down", fnb.name)

	for i := len(fnb.doneObject) - 1; i >= 0; i-- {
		doneObject := fnb.doneObject[i]

		fnb.handleDoneObject(doneObject)
	}

	fnb.closeDatabase()

	fnb.Logger.Info().Msgf("%s node shutdown complete", fnb.name)

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

func loadDKGPublicData(path string) (*dkg.PublicData, error) {
	data, err := ioutil.ReadFile(filepath.Join(path, bootstrap.FilenameDKGDataPub))
	if err != nil {
		return nil, err
	}
	dkgPubData := &bootstrap.DKGDataPub{}
	err = json.Unmarshal(data, dkgPubData)
	return dkgPubData.ForHotStuff(), err
}

func loadTrustedRootBlock(path string) (*flow.Block, error) {
	data, err := ioutil.ReadFile(filepath.Join(path, bootstrap.FilenameGenesisBlock))
	if err != nil {
		return nil, err
	}
	var genesisBlock flow.Block
	err = json.Unmarshal(data, &genesisBlock)
	return &genesisBlock, err

}

func loadRootBlockQC(path string) (*model.QuorumCertificate, error) {
	data, err := ioutil.ReadFile(filepath.Join(path, bootstrap.FilenameGenesisQC))
	if err != nil {
		return nil, err
	}
	qc := &model.QuorumCertificate{}
	err = json.Unmarshal(data, qc)
	return qc, err
}

// Loads the private info for this node from disk (eg. private staking/network keys).
func loadPrivateNodeInfo(path string, myID flow.Identifier) (*bootstrap.NodeInfoPriv, error) {
	data, err := ioutil.ReadFile(filepath.Join(path, fmt.Sprintf(bootstrap.FilenameNodeInfoPriv, myID)))
	if err != nil {
		return nil, err
	}
	var info bootstrap.NodeInfoPriv
	err = json.Unmarshal(data, &info)
	return &info, err
}
