package cmd

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

type NodeBuilder interface {
	BaseFlags()
	ExtraFlags(f func(*pflag.FlagSet)) NodeBuilder
	Initialize() NodeBuilder
	EnqueueNetworkInit()
	EnqueueMetricsServerInit()
	ParseAndPrintFlags()
	PrintBuildVersionDetails()
	InitLocal()
	Module(name string, f func(builder NodeBuilder) error) NodeBuilder
	MustNot(err error) *zerolog.Event
	Component(name string, f func(NodeBuilder) (module.ReadyDoneAware, error)) NodeBuilder
	Run()
	PostInit(f func(node NodeBuilder)) NodeBuilder

	// getters
	Config() BaseConfig
	NodeID() flow.Identifier
	Logger() *zerolog.Logger
	Me() *local.Local
	Tracer() module.Tracer
	MetricsRegisterer() prometheus.Registerer
	Metrics() Metrics
	DB() *badger.DB
	Storage() Storage
	ProtocolEvents() *events.Distributor
	ProtocolState() protocol.State
	Middleware() *p2p.Middleware
	Network() *p2p.Network
	MsgValidators() []network.MessageValidator
	FvmOptions() []fvm.Option
	RootBlock() *flow.Block
	RootQC() *flow.QuorumCertificate
	RootSeal() *flow.Seal
	RootChainID() flow.ChainID
}

// BaseConfig is the general config for the NodeBuilder
type BaseConfig struct {
	nodeIDHex             string
	bindAddr              string
	NodeRole              string
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
