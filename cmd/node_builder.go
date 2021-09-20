package cmd

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	bstorage "github.com/onflow/flow-go/storage/badger"
)

const NotSet = "not set"

// NodeBuilder declares the initialization methods needed to bootstrap up a Flow node
type NodeBuilder interface {
	module.ReadyDoneAware

	// BaseFlags reads the command line arguments common to all nodes
	BaseFlags()

	// ExtraFlags reads the node specific command line arguments and adds it to the FlagSet
	ExtraFlags(f func(*pflag.FlagSet)) NodeBuilder

	// ParseAndPrintFlags parses all the command line arguments
	ParseAndPrintFlags()

	// Initialize performs all the initialization needed at the very start of a node
	Initialize() NodeBuilder

	// PrintBuildVersionDetails prints the node software build version
	PrintBuildVersionDetails()

	// InitIDProviders initializes the ID providers needed by various components
	InitIDProviders()

	// EnqueueNetworkInit enqueues the default network component with the given context
	EnqueueNetworkInit(ctx context.Context)

	// EnqueueMetricsServerInit enqueues the metrics component
	EnqueueMetricsServerInit()

	// Enqueues the Tracer component
	EnqueueTracer()

	// Module enables setting up dependencies of the engine with the builder context
	Module(name string, f func(builder NodeBuilder, node *NodeConfig) error) NodeBuilder

	// Component adds a new component to the node that conforms to the ReadyDone
	// interface.
	//
	// When the node is run, this component will be started with `Ready`. When the
	// node is stopped, we will wait for the component to exit gracefully with
	// `Done`.
	Component(name string, f func(builder NodeBuilder, node *NodeConfig) (module.ReadyDoneAware, error)) NodeBuilder

	// MustNot asserts that the given error must not occur.
	// If the error is nil, returns a nil log event (which acts as a no-op).
	// If the error is not nil, returns a fatal log event containing the error.
	MustNot(err error) *zerolog.Event

	// Run initiates all common components (logger, database, protocol state etc.)
	// then starts each component. It also sets up a channel to gracefully shut
	// down each component if a SIGINT is received.
	Run()

	// PreInit registers a new PreInit function.
	// PreInit functions run before the protocol state is initialized or any other modules or components are initialized
	PreInit(f func(builder NodeBuilder, node *NodeConfig)) NodeBuilder

	// PostInit registers a new PreInit function.
	// PostInit functions run after the protocol state has been initialized but before any other modules or components
	// are initialized
	PostInit(f func(builder NodeBuilder, node *NodeConfig)) NodeBuilder

	// RegisterBadgerMetrics registers all badger related metrics
	RegisterBadgerMetrics()

	// ValidateFlags is an extra method called after parsing flags, intended for extra check of flag validity
	// for example where certain combinations aren't allowed
	ValidateFlags(func() error) NodeBuilder
}

// BaseConfig is the general config for the NodeBuilder and the command line params
// For a node running as a standalone process, the config fields will be populated from the command line params,
// while for a node running as a library, the config fields are expected to be initialized by the caller.
type BaseConfig struct {
	nodeIDHex             string
	BindAddr              string
	NodeRole              string
	timeout               time.Duration
	datadir               string
	level                 string
	metricsPort           uint
	BootstrapDir          string
	PeerUpdateInterval    time.Duration
	UnicastMessageTimeout time.Duration
	DNSCacheTTL           time.Duration
	profilerEnabled       bool
	profilerDir           string
	profilerInterval      time.Duration
	profilerDuration      time.Duration
	tracerEnabled         bool
	tracerSensitivity     uint
	metricsEnabled        bool
	guaranteesCacheSize   uint
	receiptsCacheSize     uint
	db                    *badger.DB
}

// NodeConfig contains all the derived parameters such the NodeID, private keys etc. and initialized instances of
// structs such as DB, Network etc. The NodeConfig is composed of the BaseConfig and is updated in the
// NodeBuilder functions as a node is bootstrapped.
type NodeConfig struct {
	Cancel context.CancelFunc // cancel function for the context that is passed to the networking layer
	BaseConfig
	Logger            zerolog.Logger
	NodeID            flow.Identifier
	Me                *local.Local
	Tracer            module.Tracer
	MetricsRegisterer prometheus.Registerer
	Metrics           Metrics
	DB                *badger.DB
	Storage           Storage
	ProtocolEvents    *events.Distributor
	State             protocol.State
	Middleware        network.Middleware
	Network           module.ReadyDoneAwareNetwork
	MsgValidators     []network.MessageValidator
	FvmOptions        []fvm.Option
	StakingKey        crypto.PrivateKey
	NetworkKey        crypto.PrivateKey

	// ID providers
	IdentityProvider             id.IdentityProvider
	IDTranslator                 p2p.IDTranslator
	NetworkingIdentifierProvider id.IdentifierProvider
	SyncEngineIdentifierProvider id.IdentifierProvider

	// root state information
	RootBlock                     *flow.Block
	RootQC                        *flow.QuorumCertificate
	RootResult                    *flow.ExecutionResult
	RootSeal                      *flow.Seal
	RootChainID                   flow.ChainID
	SkipNwAddressBasedValidations bool
}

func DefaultBaseConfig() *BaseConfig {
	homedir, _ := os.UserHomeDir()
	datadir := filepath.Join(homedir, ".flow", "database")
	return &BaseConfig{
		nodeIDHex:             NotSet,
		BindAddr:              NotSet,
		BootstrapDir:          "bootstrap",
		timeout:               1 * time.Minute,
		datadir:               datadir,
		level:                 "info",
		PeerUpdateInterval:    p2p.DefaultPeerUpdateInterval,
		UnicastMessageTimeout: p2p.DefaultUnicastTimeout,
		metricsPort:           8080,
		profilerEnabled:       false,
		profilerDir:           "profiler",
		profilerInterval:      15 * time.Minute,
		profilerDuration:      10 * time.Second,
		tracerEnabled:         false,
		tracerSensitivity:     4,
		metricsEnabled:        true,
		receiptsCacheSize:     bstorage.DefaultCacheSize,
		guaranteesCacheSize:   bstorage.DefaultCacheSize,
	}
}
