package cmd

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dgraph-io/badger/v2"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	bstorage "github.com/onflow/flow-go/storage/badger"
)

const NotSet = "not set"

type BuilderFunc func(nodeConfig *NodeConfig) error
type ReadyDoneFactory func(node *NodeConfig) (module.ReadyDoneAware, error)

// NodeBuilder declares the initialization methods needed to bootstrap up a Flow node
type NodeBuilder interface {
	// BaseFlags reads the command line arguments common to all nodes
	BaseFlags()

	// ExtraFlags reads the node specific command line arguments and adds it to the FlagSet
	ExtraFlags(f func(*pflag.FlagSet)) NodeBuilder

	// ParseAndPrintFlags parses and validates all the command line arguments
	ParseAndPrintFlags() error

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
	Module(name string, f BuilderFunc) NodeBuilder

	// Component adds a new component to the node that conforms to the ReadyDoneAware
	// interface, and throws a Fatal() when an irrecoverable error is encountered.
	//
	// The ReadyDoneFactory may return either a `Component` or `ReadyDoneAware` instance.
	// In both cases, the object is started according to its interface when the node is run,
	// and the node will wait for the component to exit gracefully.
	Component(name string, f ReadyDoneFactory) NodeBuilder

	// ShutdownFunc adds a callback function that is called after all components have exited.
	// All shutdown functions are called regardless of errors returned by previous callbacks. Any
	// errors returned are captured and passed to the caller.
	ShutdownFunc(fn func() error) NodeBuilder

	// AdminCommand registers a new admin command with the admin server
	AdminCommand(command string, f func(config *NodeConfig) commands.AdminCommand) NodeBuilder

	// MustNot asserts that the given error must not occur.
	// If the error is nil, returns a nil log event (which acts as a no-op).
	// If the error is not nil, returns a fatal log event containing the error.
	MustNot(err error) *zerolog.Event

	// Build finalizes the node configuration in preparation for start and returns a Node
	// object that can be run
	Build() (Node, error)

	// PreInit registers a new PreInit function.
	// PreInit functions run before the protocol state is initialized or any other modules or components are initialized
	PreInit(f BuilderFunc) NodeBuilder

	// PostInit registers a new PreInit function.
	// PostInit functions run after the protocol state has been initialized but before any other modules or components
	// are initialized
	PostInit(f BuilderFunc) NodeBuilder

	// RegisterBadgerMetrics registers all badger related metrics
	RegisterBadgerMetrics() error

	// ValidateFlags sets any custom validation rules for the command line flags,
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
	DynamicStartupANAddress         string
	DynamicStartupANPubkey          string
	DynamicStartupEpochPhase        string
	DynamicStartupEpoch             string
	DynamicStartupSleepInterval     time.Duration
	datadir                         string
	secretsdir                      string
	secretsDBEnabled                bool
	InsecureSecretsDB               bool
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
	profilerMemProfileRate          int
	tracerEnabled                   bool
	tracerSensitivity               uint
	MetricsEnabled                  bool
	guaranteesCacheSize             uint
	receiptsCacheSize               uint
	db                              *badger.DB
	PreferredUnicastProtocols       []string
	NetworkReceivedMessageCacheSize int
	topologyProtocolName            string
	topologyEdgeProbability         float64
}

// NodeConfig contains all the derived parameters such the NodeID, private keys etc. and initialized instances of
// structs such as DB, Network etc. The NodeConfig is composed of the BaseConfig and is updated in the
// NodeBuilder functions as a node is bootstrapped.
type NodeConfig struct {
	Cancel context.CancelFunc // cancel function for the context that is passed to the networking layer
	BaseConfig
	Logger            zerolog.Logger
	NodeID            flow.Identifier
	Me                module.Local
	Tracer            module.Tracer
	MetricsRegisterer prometheus.Registerer
	Metrics           Metrics
	DB                *badger.DB
	SecretsDB         *badger.DB
	Storage           Storage
	ProtocolEvents    *events.Distributor
	State             protocol.State
	Resolver          madns.BasicResolver
	Middleware        network.Middleware
	Network           network.Network
	PingService       network.PingService
	MsgValidators     []network.MessageValidator
	FvmOptions        []fvm.Option
	StakingKey        crypto.PrivateKey
	NetworkKey        crypto.PrivateKey

	// ID providers
	IdentityProvider             id.IdentityProvider
	IDTranslator                 p2p.IDTranslator
	SyncEngineIdentifierProvider id.IdentifierProvider

	// root state information
	RootSnapshot protocol.Snapshot
	// cached properties of RootSnapshot for convenience
	RootBlock   *flow.Block
	RootQC      *flow.QuorumCertificate
	RootResult  *flow.ExecutionResult
	RootSeal    *flow.Seal
	RootChainID flow.ChainID
	SporkID     flow.Identifier

	// bootstrapping options
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
		profilerMemProfileRate:          runtime.MemProfileRate,
		tracerEnabled:                   false,
		tracerSensitivity:               4,
		MetricsEnabled:                  true,
		receiptsCacheSize:               bstorage.DefaultCacheSize,
		guaranteesCacheSize:             bstorage.DefaultCacheSize,
		NetworkReceivedMessageCacheSize: p2p.DefaultCacheSize,
		topologyProtocolName:            string(topology.TopicBased),
		topologyEdgeProbability:         topology.MaximumEdgeProbability,
	}
}
