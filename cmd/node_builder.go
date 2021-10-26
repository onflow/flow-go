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

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
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
	// BaseFlags reads the command line arguments common to all nodes
	BaseFlags()

	// ExtraFlags reads the node specific command line arguments and adds it to the FlagSet
	ExtraFlags(f func(*pflag.FlagSet)) NodeBuilder

	// ParseAndPrintFlags parses all the command line arguments
	ParseAndPrintFlags()

	// Initialize performs all the initialization needed at the very start of a node
	Initialize() error

	// PrintBuildVersionDetails prints the node software build version
	PrintBuildVersionDetails()

	// InitIDProviders initializes the ID providers needed by various components
	InitIDProviders()

	// EnqueueNetworkInit enqueues the default network component with the given context
	EnqueueNetworkInit()

	// EnqueueMetricsServerInit enqueues the metrics component
	EnqueueMetricsServerInit()

	// Enqueues the Tracer component
	EnqueueTracer()

	// Module enables setting up dependencies of the engine with the builder context
	Module(name string, f func(ctx irrecoverable.SignalerContext, node *NodeConfig) error) NodeBuilder

	// Component adds a new component to the node that conforms to the ReadyDoneAware
	// interface, and throws a Fatal() when an irrecoverable error is encountered
	// Use Component if the component cannot be restarted when an irrecoverable error is encountered
	// and the node should crash.
	//
	// When the node is run, this component will be started with `Ready`. When the
	// node is stopped, we will wait for the component to exit gracefully with
	// `Done`.
	Component(name string, f func(ctx irrecoverable.SignalerContext, node *NodeConfig, lookup component.LookupFunc) (module.ReadyDoneAware, error)) NodeBuilder

	// BackgroundComponent adds a new component to the node that conforms to the ReadyDoneAware
	// interface, and calls the provided error handler when an irrecoverable error is encountered.
	// Use BackgroundComponent if the component is not critical to the node's safe operation and
	// can/should be independently restarted when an irrecoverable error is encountered.
	//
	// Any irrecoverable errors thrown by the component will be passed to the provided error handler.
	BackgroundComponent(name string, f func(ctx irrecoverable.SignalerContext, node *NodeConfig, lookup component.LookupFunc) (component.Component, error), errHandler func(err error) component.ErrorHandlingResult) NodeBuilder

	// AdminCommand registers a new admin command with the admin server
	AdminCommand(command string, handler admin.CommandHandler, validator admin.CommandValidator) NodeBuilder

	// MustNot asserts that the given error must not occur.
	// If the error is nil, returns a nil log event (which acts as a no-op).
	// If the error is not nil, returns a fatal log event containing the error.
	MustNot(err error) *zerolog.Event

	// SerialStart configures the node to start components serially
	// By default, components are started in parallel and must have explicit dependency checks.
	// This allows using the order components were added to manage dependencies implicitly
	SerialStart() NodeBuilder

	// Build finalizes the node configuration in preparation for start and returns a Node
	// object that can be run
	Build() Node

	// PreInit registers a new PreInit function.
	// PreInit functions run before the protocol state is initialized or any other modules or components are initialized
	PreInit(f func(ctx irrecoverable.SignalerContext, node *NodeConfig)) NodeBuilder

	// PostInit registers a new PreInit function.
	// PostInit functions run after the protocol state has been initialized but before any other modules or components
	// are initialized
	PostInit(f func(ctx irrecoverable.SignalerContext, node *NodeConfig)) NodeBuilder

	// RegisterBadgerMetrics registers all badger related metrics
	RegisterBadgerMetrics() error

	// ValidateFlags is an extra method called after parsing flags, intended for extra check of flag validity
	// for example where certain combinations aren't allowed
	ValidateFlags(func() error) NodeBuilder
}

// BaseConfig is the general config for the NodeBuilder and the command line params
// For a node running as a standalone process, the config fields will be populated from the command line params,
// while for a node running as a library, the config fields are expected to be initialized by the caller.
type BaseConfig struct {
	nodeIDHex                       string
	AdminAddr                       string
	AdminCert                       string
	AdminKey                        string
	AdminClientCAs                  string
	BindAddr                        string
	NodeRole                        string
	datadir                         string
	secretsdir                      string
	secretsDBEnabled                bool
	level                           string
	metricsPort                     uint
	BootstrapDir                    string
	PeerUpdateInterval              time.Duration
	UnicastMessageTimeout           time.Duration
	DNSCacheTTL                     time.Duration
	profilerEnabled                 bool
	profilerDir                     string
	profilerInterval                time.Duration
	profilerDuration                time.Duration
	tracerEnabled                   bool
	tracerSensitivity               uint
	metricsEnabled                  bool
	guaranteesCacheSize             uint
	receiptsCacheSize               uint
	db                              *badger.DB
	LibP2PStreamCompression         string
	NetworkReceivedMessageCacheSize int
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
	SecretsDB         *badger.DB
	Storage           Storage
	ProtocolEvents    *events.Distributor
	State             protocol.State
	Middleware        network.Middleware
	Network           network.Network
	MsgValidators     []network.MessageValidator
	FvmOptions        []fvm.Option
	StakingKey        crypto.PrivateKey
	NetworkKey        crypto.PrivateKey

	// ID providers
	IdentityProvider             id.IdentityProvider
	IDTranslator                 p2p.IDTranslator
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
		nodeIDHex:                       NotSet,
		AdminAddr:                       NotSet,
		AdminCert:                       NotSet,
		AdminKey:                        NotSet,
		AdminClientCAs:                  NotSet,
		BindAddr:                        NotSet,
		BootstrapDir:                    "bootstrap",
		datadir:                         datadir,
		secretsdir:                      NotSet,
		secretsDBEnabled:                true,
		level:                           "info",
		PeerUpdateInterval:              p2p.DefaultPeerUpdateInterval,
		UnicastMessageTimeout:           p2p.DefaultUnicastTimeout,
		metricsPort:                     8080,
		profilerEnabled:                 false,
		profilerDir:                     "profiler",
		profilerInterval:                15 * time.Minute,
		profilerDuration:                10 * time.Second,
		tracerEnabled:                   false,
		tracerSensitivity:               4,
		metricsEnabled:                  true,
		receiptsCacheSize:               bstorage.DefaultCacheSize,
		guaranteesCacheSize:             bstorage.DefaultCacheSize,
		LibP2PStreamCompression:         p2p.NoCompression,
		NetworkReceivedMessageCacheSize: p2p.DefaultCacheSize,
	}
}
