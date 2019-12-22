package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/trickle"
	"github.com/dapperlabs/flow-go/network/trickle/middleware"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
)

type BaseConfig struct {
	NodeID      string
	Entries     []string
	Timeout     time.Duration
	Connections uint
	datadir     string
	level       string
}

type MaksioConfig struct {
	BaseConfig
}

func (fnb *FlowNodeBuilder) baseFlags() {
	// bind configuration parameters
	fnb.flags.StringVarP(&fnb.BaseConfig.NodeID, "nodeid", "n", "node1", "identity of our node")
	fnb.flags.StringSliceVarP(&fnb.BaseConfig.Entries, "entries", "e", []string{"consensus-node1@address1=1000"}, "identity table entries for all nodes")
	fnb.flags.DurationVarP(&fnb.BaseConfig.Timeout, "timeout", "t", 1*time.Minute, "how long to try connecting to the network")
	fnb.flags.UintVarP(&fnb.BaseConfig.Connections, "connections", "c", 0, "number of connections to establish to peers")
	fnb.flags.StringVarP(&fnb.BaseConfig.datadir, "datadir", "d", "data", "directory to store the protocol State")
	fnb.flags.StringVarP(&fnb.BaseConfig.level, "loglevel", "l", "info", "level for logging output")
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
	db, err := badger.Open(badger.DefaultOptions(fnb.BaseConfig.datadir).WithLogger(nil))
	if err != nil {
		fnb.MustNot(err).Msg("could not open key-value store")
	}
	fnb.DB = db
}

func (fnb *FlowNodeBuilder) initState() {
	state, err := protocol.NewState(fnb.DB)
	fnb.MustNot(err).Msg("could not initialize flow committee")

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

		err = state.Mutate().Bootstrap(flow.Genesis(ids))
		if err != nil {
			fnb.Logger.Fatal().Err(err).Msg("could not bootstrap protocol State")
		}
	}

	trueID, err := flow.HexStringToIdentifier(fnb.BaseConfig.NodeID)
	fnb.MustNot(err).Msg("could not parse local node ID")
	allIdentities, err := state.Final().Identities()
	fnb.MustNot(err).Msg("could not retrieve finalized identities")
	fnb.Logger.Debug().Msg(fmt.Sprintf("%v", allIdentities))

	id, err := state.Final().Identity(trueID)
	fnb.MustNot(err).Msg("could not get identity")

	fnb.Me, err = local.New(id)
	fnb.MustNot(err).Msg("could not initialize local")

	fnb.State = state
}

func (fnb *FlowNodeBuilder) handleReadyAware(v namedReadyFn) {

	readyAware := v.fn(fnb)

	select {
	case <-readyAware.Ready():
		fnb.Logger.Info().Msg(v.name + " ready")
	case <-time.After(fnb.BaseConfig.Timeout):
		fnb.Logger.Fatal().Msg("could not start " + v.name)
	case <-fnb.sig:
		fnb.Logger.Warn().Msg(v.name + " start aborted")
		os.Exit(1)
	}

	fnb.doneObject = append(fnb.doneObject, namedDoneObject{
		readyAware, v.name,
	})
}

func (fnb *FlowNodeBuilder) handleDoneObject(v namedDoneObject) {
	fnb.Logger.Info().Msg("stopping " + v.name)

	select {
	case <-v.ob.Done():
		fnb.Logger.Info().Msg(v.name + " shutdown complete")
	case <-time.After(fnb.BaseConfig.Timeout):
		fnb.Logger.Fatal().Msg("could not stop " + v.name)
	case <-fnb.sig:
		fnb.Logger.Warn().Msg(v.name + " stop aborted")
		os.Exit(1)
	}
}

func (fnb *FlowNodeBuilder) enqueueNetworkInit() {
	fnb.CreateReadDoneAware("trickle network", func(builder *FlowNodeBuilder) module.ReadyDoneAware {

		fnb.Logger.Info().Msg("initializing network stack")

		codec := json.NewCodec()

		mw, err := middleware.New(fnb.Logger, codec, fnb.BaseConfig.Connections, fnb.Me.Address())
		fnb.MustNot(err).Msg("could not initialize trickle middleware")

		net, err := trickle.NewNetwork(fnb.Logger, codec, fnb.State, fnb.Me, mw)
		fnb.MustNot(err).Msg("could not initialize trickle network")
		fnb.Network = net
		return net
	})
}

type namedReadyFn struct {
	fn   func(*FlowNodeBuilder) module.ReadyDoneAware
	name string
}

type namedDoneObject struct {
	ob   module.ReadyDoneAware
	name string
}

type FlowNodeBuilder struct {
	BaseConfig   BaseConfig
	flags        *pflag.FlagSet
	name         string
	Logger       zerolog.Logger
	DB           *badger.DB
	Me           *local.Local
	State        *protocol.State
	readyDoneFns []namedReadyFn
	doneObject   []namedDoneObject
	sig          chan os.Signal
	Network      *trickle.Network
}

func (fnb *FlowNodeBuilder) ExtraFlags(f func(*pflag.FlagSet)) *FlowNodeBuilder {
	f(fnb.flags)
	return fnb
}

func (fnb *FlowNodeBuilder) Create(f func(builder *FlowNodeBuilder)) *FlowNodeBuilder {
	f(fnb)
	return fnb
}

func (fnb *FlowNodeBuilder) MustNot(err error) *zerolog.Event {
	if err != nil {
		return fnb.Logger.Fatal().Err(err)
	}
	return nil
}

func (fnb *FlowNodeBuilder) CreateReadDoneAware(name string, f func(*FlowNodeBuilder) module.ReadyDoneAware) *FlowNodeBuilder {
	fnb.readyDoneFns = append(fnb.readyDoneFns, namedReadyFn{
		fn:   f,
		name: name,
	})

	return fnb
}

func FlowNode(name string) *FlowNodeBuilder {

	builder := &FlowNodeBuilder{
		BaseConfig:   BaseConfig{},
		flags:        pflag.CommandLine,
		name:         name,
		readyDoneFns: make([]namedReadyFn, 0),
	}

	builder.baseFlags()

	builder.enqueueNetworkInit()

	return builder
}

func (fnb *FlowNodeBuilder) Run() {

	// initialize signal catcher
	fnb.sig = make(chan os.Signal, 1)
	signal.Notify(fnb.sig, os.Interrupt)

	// parse configuration parameters
	pflag.Parse()

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	fnb.initLogger()

	fnb.initDatabase()

	fnb.initState()

	for _, f := range fnb.readyDoneFns {
		fnb.handleReadyAware(f)
	}

	fnb.Logger.Info().Msg(fnb.name + " node startup complete")

	<-fnb.sig

	fnb.Logger.Info().Msg(fnb.name + " node shutting down")
	// whatever

	for i := len(fnb.doneObject) - 1; i >= 0; i-- {
		doneObject := fnb.doneObject[i]

		fnb.handleDoneObject(doneObject)
	}

	fnb.Logger.Info().Msg(fnb.name + " node shutdown complete")

	os.Exit(0)

}
