package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
)

const notSet = "not set"

// BaseConfig is the general config for the FlowNodeBuilder
type BaseConfig struct {
	NodeID      string
	NodeName    string
	Entries     []string
	Timeout     time.Duration
	Connections uint
	datadir     string
	level       string
	metricsPort uint
}

type namedReadyFn struct {
	fn   func(*FlowNodeBuilder) module.ReadyDoneAware
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
	flags          *pflag.FlagSet
	name           string
	Logger         zerolog.Logger
	Tracer         trace.Tracer
	DB             *badger.DB
	Me             *local.Local
	State          *protocol.State
	readyDoneFns   []namedReadyFn
	doneObject     []namedDoneObject
	sig            chan os.Signal
	Network        *libp2p.Network
	genesisHandler func(node *FlowNodeBuilder, block *flow.Block)
	postInitFns    []func(*FlowNodeBuilder)
	genesis        *flow.Block
}

func (fnb *FlowNodeBuilder) baseFlags() {
	homedir, _ := os.UserHomeDir()
	datadir := filepath.Join(homedir, ".flow", "database")
	// bind configuration parameters
	fnb.flags.StringVar(&fnb.BaseConfig.NodeID, "nodeid", notSet, "identity of our node")
	fnb.flags.StringVarP(&fnb.BaseConfig.NodeName, "nodename", "n", "node1", "identity of our node")
	fnb.flags.StringSliceVarP(&fnb.BaseConfig.Entries, "entries", "e", []string{"consensus-node1@address1=1000"}, "identity table entries for all nodes")
	fnb.flags.DurationVarP(&fnb.BaseConfig.Timeout, "timeout", "t", 1*time.Minute, "how long to try connecting to the network")
	fnb.flags.UintVarP(&fnb.BaseConfig.Connections, "connections", "c", 0, "number of connections to establish to peers")
	fnb.flags.StringVarP(&fnb.BaseConfig.datadir, "datadir", "d", datadir, "directory to store the protocol State")
	fnb.flags.StringVarP(&fnb.BaseConfig.level, "loglevel", "l", "info", "level for logging output")
	fnb.flags.UintVarP(&fnb.BaseConfig.metricsPort, "metricport", "m", 8080, "port for /metrics endpoint")
}

func (fnb *FlowNodeBuilder) enqueueNetworkInit() {
	fnb.Component("flow network", func(builder *FlowNodeBuilder) module.ReadyDoneAware {

		fnb.Logger.Info().Msg("initializing network stack")

		codec := json.NewCodec()

		mw, err := libp2p.NewMiddleware(fnb.Logger.Level(zerolog.Disabled), codec, fnb.Me.Address(), fnb.Me.NodeID())
		fnb.MustNot(err).Msg("could not initialize flow middleware")

		net, err := libp2p.NewNetwork(fnb.Logger.Level(zerolog.Disabled), codec, fnb.State, fnb.Me, mw, 10e6)
		fnb.MustNot(err).Msg("could not initialize flow network")
		fnb.Network = net
		return net
	})
}

func (fnb *FlowNodeBuilder) enqueueMetricsServerInit() {
	fnb.Component("metrics server", func(builder *FlowNodeBuilder) module.ReadyDoneAware {
		fnb.Logger.Info().Msg("initializing metrics server")
		server := metrics.NewServer(fnb.Logger, fnb.BaseConfig.metricsPort)
		return server
	})
}

func (fnb *FlowNodeBuilder) initNodeID() {
	if fnb.BaseConfig.NodeID == notSet {
		h := sha256.New()
		_, err := h.Write([]byte(fnb.BaseConfig.NodeName))
		fnb.MustNot(err).Msg("could not initialize node id")
		fnb.BaseConfig.NodeID = hex.EncodeToString(h.Sum(nil))
	}
}

func (fnb *FlowNodeBuilder) initLogger() {
	// configure logger with standard level, node ID and UTC timestamp
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(os.Stderr).With().Timestamp().Str("node_id", fnb.BaseConfig.NodeID).Logger()

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

	db, err := badger.Open(badger.DefaultOptions(fnb.BaseConfig.datadir).WithLogger(nil))
	fnb.MustNot(err).Msg("could not open key-value store")
	fnb.DB = db
}

func (fnb *FlowNodeBuilder) initTracer() {
	tracer, err := trace.NewTracer(fnb.Logger)
	fnb.MustNot(err).Msg("could not initialize tracer")
	fnb.Tracer = tracer
}

func (fnb *FlowNodeBuilder) initState() {
	state, err := protocol.NewState(fnb.DB)
	fnb.MustNot(err).Msg("could not initialize flow state")

	//check if database is initialized
	lsm, vlog := fnb.DB.Size()
	if vlog > 0 || lsm > 0 {
		fnb.Logger.Debug().Msg("using existing database")
	} else {
		//Bootstrap!

		fnb.Logger.Info().Msg("bootstrapping empty database")

		var ids flow.IdentityList
		for _, entry := range fnb.BaseConfig.Entries {
			id, err := flow.ParseIdentity(entry)
			if err != nil {
				fnb.Logger.Fatal().Err(err).Str("entry", entry).Msg("could not parse identity")
			}
			ids = append(ids, id)
		}

		fnb.genesis = flow.Genesis(ids)
		err = state.Mutate().Bootstrap(fnb.genesis)
		if err != nil {
			fnb.Logger.Fatal().Err(err).Msg("could not bootstrap protocol state")
		}

	}

	myID, err := flow.HexStringToIdentifier(fnb.BaseConfig.NodeID)
	fnb.MustNot(err).Msg("could not parse node identifier")

	allIdentities, err := state.Final().Identities()
	fnb.MustNot(err).Msg("could not retrieve finalized identities")
	fnb.Logger.Debug().Msgf("known nodes: %v", allIdentities)

	id, err := state.Final().Identity(myID)
	fnb.MustNot(err).Msg("could not get identity")

	fnb.Me, err = local.New(id)
	fnb.MustNot(err).Msg("could not initialize local")

	fnb.State = state
}

func (fnb *FlowNodeBuilder) handleReadyAware(v namedReadyFn) {

	readyAware := v.fn(fnb)

	select {
	case <-readyAware.Ready():
		fnb.Logger.Info().Msgf("%s ready", v.name)
	case <-time.After(fnb.BaseConfig.Timeout):
		fnb.Logger.Fatal().Msgf("could not start %s", v.name)
	case <-fnb.sig:
		fnb.Logger.Warn().Msgf("%s start aborted", v.name)
		os.Exit(1)
	}

	fnb.doneObject = append(fnb.doneObject, namedDoneObject{
		readyAware, v.name,
	})
}

func (fnb *FlowNodeBuilder) handleDoneObject(v namedDoneObject) {
	fnb.Logger.Info().Msgf("stopping %s", v.name)

	select {
	case <-v.ob.Done():
		fnb.Logger.Info().Msgf("%s shutdown complete", v.name)
	case <-time.After(fnb.BaseConfig.Timeout):
		fnb.Logger.Fatal().Msgf("could not stop %s", v.name)
	case <-fnb.sig:
		fnb.Logger.Warn().Msgf("%s stop aborted", v.name)
		os.Exit(1)
	}
}

// ExtraFlags enables binding additional flags beyond those defined in BaseConfig.
func (fnb *FlowNodeBuilder) ExtraFlags(f func(*pflag.FlagSet)) *FlowNodeBuilder {
	f(fnb.flags)
	return fnb
}

// Create enables setting up dependencies of the node with the context of the
// builder.
func (fnb *FlowNodeBuilder) Create(f func(builder *FlowNodeBuilder)) *FlowNodeBuilder {
	f(fnb)
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
func (fnb *FlowNodeBuilder) Component(name string, f func(*FlowNodeBuilder) module.ReadyDoneAware) *FlowNodeBuilder {
	fnb.readyDoneFns = append(fnb.readyDoneFns, namedReadyFn{
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
		BaseConfig:   BaseConfig{},
		flags:        pflag.CommandLine,
		name:         name,
		readyDoneFns: make([]namedReadyFn, 0),
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
	signal.Notify(fnb.sig, os.Interrupt)

	// parse configuration parameters
	pflag.Parse()

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	fnb.initNodeID()

	fnb.initLogger()

	fnb.initTracer()

	fnb.initDatabase()

	fnb.initState()

	for _, f := range fnb.postInitFns {
		fnb.handlePostInit(f)
	}

	if fnb.genesis != nil && fnb.genesisHandler != nil {
		fnb.genesisHandler(fnb, fnb.genesis)
	}

	for _, f := range fnb.readyDoneFns {
		fnb.handleReadyAware(f)
	}

	fnb.Logger.Info().Msgf("%s node startup complete", fnb.name)

	<-fnb.sig

	fnb.Logger.Info().Msgf("%s node shutting down", fnb.name)

	for i := len(fnb.doneObject) - 1; i >= 0; i-- {
		doneObject := fnb.doneObject[i]

		fnb.handleDoneObject(doneObject)
	}

	fnb.Logger.Info().Msgf("%s node shutdown complete", fnb.name)

	os.Exit(0)

}

func (fnb *FlowNodeBuilder) handlePostInit(f func(node *FlowNodeBuilder)) {
	f(fnb)
}
