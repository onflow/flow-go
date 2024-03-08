package cmd

import (
	"context"
	"time"

	"github.com/dgraph-io/badger/v2"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/profiler"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/grpcutils"
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

	// EnqueueNetworkInit enqueues the default networking layer.
	EnqueueNetworkInit()

	// EnqueueMetricsServerInit enqueues the metrics component.
	EnqueueMetricsServerInit()

	// EnqueueTracer enqueues the Tracer component.
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

	// DependableComponent adds a new component to the node that conforms to the ReadyDoneAware
	// interface. The builder will wait until all of the components in the dependencies list are ready
	// before constructing the component.
	//
	// The ReadyDoneFactory may return either a `Component` or `ReadyDoneAware` instance.
	// In both cases, the object is started when the node is run, and the node will wait for the
	// component to exit gracefully.
	//
	// IMPORTANT: Dependable components are started in parallel with no guaranteed run order, so all
	// dependencies must be initialized outside of the ReadyDoneFactory, and their `Ready()` method
	// MUST be idempotent.
	DependableComponent(name string, f ReadyDoneFactory, dependencies *DependencyList) NodeBuilder

	// RestartableComponent adds a new component to the node that conforms to the ReadyDoneAware
	// interface, and calls the provided error handler when an irrecoverable error is encountered.
	// Use RestartableComponent if the component is not critical to the node's safe operation and
	// can/should be independently restarted when an irrecoverable error is encountered.
	//
	// Any irrecoverable errors thrown by the component will be passed to the provided error handler.
	RestartableComponent(name string, f ReadyDoneFactory, errorHandler component.OnError) NodeBuilder

	// ShutdownFunc adds a callback function that is called after all components have exited.
	// All shutdown functions are called regardless of errors returned by previous callbacks. Any
	// errors returned are captured and passed to the caller.
	ShutdownFunc(fn func() error) NodeBuilder

	// AdminCommand registers a new admin command with the admin server
	AdminCommand(command string, f func(config *NodeConfig) commands.AdminCommand) NodeBuilder

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

	// ValidateRootSnapshot sets any custom validation rules for the root snapshot.
	// This check is executed after other checks but before applying any data from root snapshot.
	ValidateRootSnapshot(f func(protocol.Snapshot) error) NodeBuilder
}

// BaseConfig is the general config for the NodeBuilder and the command line params
// For a node running as a standalone process, the config fields will be populated from the command line params,
// while for a node running as a library, the config fields are expected to be initialized by the caller.
type BaseConfig struct {
	nodeIDHex                   string
	AdminAddr                   string
	AdminCert                   string
	AdminKey                    string
	AdminClientCAs              string
	AdminMaxMsgSize             uint
	BindAddr                    string
	NodeRole                    string
	ObserverMode                bool
	DynamicStartupANAddress     string
	DynamicStartupANPubkey      string
	DynamicStartupEpochPhase    string
	DynamicStartupEpoch         string
	DynamicStartupSleepInterval time.Duration
	datadir                     string
	secretsdir                  string
	secretsDBEnabled            bool
	InsecureSecretsDB           bool
	level                       string
	debugLogLimit               uint32
	metricsPort                 uint
	BootstrapDir                string
	profilerConfig              profiler.ProfilerConfig
	tracerEnabled               bool
	tracerSensitivity           uint
	MetricsEnabled              bool
	guaranteesCacheSize         uint
	receiptsCacheSize           uint
	db                          *badger.DB
	HeroCacheMetricsEnable      bool
	SyncCoreConfig              chainsync.Config
	CodecFactory                func() network.Codec
	LibP2PNode                  p2p.LibP2PNode
	// ComplianceConfig configures either the compliance engine (consensus nodes)
	// or the follower engine (all other node roles)
	ComplianceConfig compliance.Config

	// FlowConfig Flow configuration.
	FlowConfig config.FlowConfig
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
	ConfigManager     *updatable_configs.Manager
	MetricsRegisterer prometheus.Registerer
	Metrics           Metrics
	DB                *badger.DB
	SecretsDB         *badger.DB
	Storage           Storage
	ProtocolEvents    *events.Distributor
	State             protocol.State
	Resolver          madns.BasicResolver
	EngineRegistry    network.EngineRegistry
	NetworkUnderlay   network.Underlay
	ConduitFactory    network.ConduitFactory
	PingService       network.PingService
	MsgValidators     []network.MessageValidator
	FvmOptions        []fvm.Option
	StakingKey        crypto.PrivateKey
	NetworkKey        crypto.PrivateKey

	// list of dependencies for network peer manager startup
	PeerManagerDependencies *DependencyList
	// ReadyDoneAware implementation of the network middleware for DependableComponents
	networkUnderlayDependable *module.ProxiedReadyDoneAware

	// ID providers
	IdentityProvider             module.IdentityProvider
	IDTranslator                 p2p.IDTranslator
	SyncEngineIdentifierProvider module.IdentifierProvider

	// root state information
	RootSnapshot protocol.Snapshot
	// excerpt of root snapshot and latest finalized snapshot, when we boot up
	StateExcerptAtBoot

	// bootstrapping options
	SkipNwAddressBasedValidations bool

	// UnicastRateLimiterDistributor notifies consumers when a peer's unicast message is rate limited.
	UnicastRateLimiterDistributor p2p.UnicastRateLimiterDistributor
}

// StateExcerptAtBoot stores information about the root snapshot and latest finalized block for use in bootstrapping.
type StateExcerptAtBoot struct {
	// properties of RootSnapshot for convenience
	// For node bootstrapped with a root snapshot for the first block of a spork,
	// 		FinalizedRootBlock and SealedRootBlock are the same block (special case of self-sealing block)
	// For node bootstrapped with a root snapshot for a block above the first block of a spork (dynamically bootstrapped),
	// 		FinalizedRootBlock.Height > SealedRootBlock.Height
	FinalizedRootBlock  *flow.Block             // The last finalized block when bootstrapped.
	SealedRootBlock     *flow.Block             // The last sealed block when bootstrapped.
	RootQC              *flow.QuorumCertificate // QC for Finalized Root Block
	RootResult          *flow.ExecutionResult   // Result for SealedRootBlock
	RootSeal            *flow.Seal              //Seal for RootResult
	RootChainID         flow.ChainID
	SporkID             flow.Identifier
	LastFinalizedHeader *flow.Header // last finalized header when the node boots up
}

func DefaultBaseConfig() *BaseConfig {
	datadir := "/data/protocol"

	// NOTE: if the codec used in the network component is ever changed any code relying on
	// the message format specific to the codec must be updated. i.e: the AuthorizedSenderValidator.
	codecFactory := func() network.Codec { return cbor.NewCodec() }

	return &BaseConfig{
		nodeIDHex:        NotSet,
		AdminAddr:        NotSet,
		AdminCert:        NotSet,
		AdminKey:         NotSet,
		AdminClientCAs:   NotSet,
		AdminMaxMsgSize:  grpcutils.DefaultMaxMsgSize,
		BindAddr:         NotSet,
		ObserverMode:     false,
		BootstrapDir:     "bootstrap",
		datadir:          datadir,
		secretsdir:       NotSet,
		secretsDBEnabled: true,
		level:            "info",
		debugLogLimit:    2000,

		metricsPort:         8080,
		tracerEnabled:       false,
		tracerSensitivity:   4,
		MetricsEnabled:      true,
		receiptsCacheSize:   bstorage.DefaultCacheSize,
		guaranteesCacheSize: bstorage.DefaultCacheSize,

		profilerConfig: profiler.ProfilerConfig{
			Enabled:         false,
			UploaderEnabled: false,

			Dir:      "profiler",
			Interval: 15 * time.Minute,
			Duration: 10 * time.Second,
		},

		HeroCacheMetricsEnable: false,
		SyncCoreConfig:         chainsync.DefaultConfig(),
		CodecFactory:           codecFactory,
		ComplianceConfig:       compliance.DefaultConfig(),
	}
}

// DependencyList is a slice of ReadyDoneAware implementations that are used by DependableComponent
// to define the list of dependencies that must be ready before starting the component.
type DependencyList struct {
	components []module.ReadyDoneAware
}

func NewDependencyList(components ...module.ReadyDoneAware) *DependencyList {
	return &DependencyList{
		components: components,
	}
}

// Add adds a new ReadyDoneAware implementation to the list of dependencies.
func (d *DependencyList) Add(component module.ReadyDoneAware) {
	d.components = append(d.components, component)
}
