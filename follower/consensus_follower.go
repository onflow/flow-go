package follower

import (
	"context"
	"fmt"
	"sync"

	"github.com/jordanschalm/lockctx"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage"
)

// ConsensusFollower is a standalone module run by third parties which provides
// a mechanism for observing the block chain. It maintains a set of subscribers
// and delivers block proposals broadcasted by the consensus nodes to each one.
type ConsensusFollower interface {
	component.Component
	// Run starts the consensus follower.
	Run(context.Context)
	// AddOnBlockFinalizedConsumer adds a new block finalization subscriber.
	AddOnBlockFinalizedConsumer(pubsub.OnBlockFinalizedConsumer)
}

// Config contains the configurable fields for a `ConsensusFollower`.
type Config struct {
	networkPrivKey   crypto.PrivateKey   // the network private key of this node
	bootstrapNodes   []BootstrapNodeInfo // the bootstrap nodes to use
	bindAddr         string              // address to bind on
	protocolDB       storage.DB          // badger instance
	bootstrapDir     string              // path to the bootstrap directory
	logLevel         string              // log level
	exposeMetrics    bool                // whether to expose metrics
	syncConfig       *chainsync.Config   // sync core configuration
	complianceConfig *compliance.Config  // follower engine configuration
	// lock manager for the follower, allows integration tests who run mulitple followers
	// to be able to use different lock managers.
	lockManager lockctx.Manager
}

type Option func(c *Config)

// currently used by rosetta
func WithProtocolDB(db storage.DB) Option {
	return func(cf *Config) {
		cf.protocolDB = db
	}
}

func WithBootstrapDir(bootstrapDir string) Option {
	return func(cf *Config) {
		cf.bootstrapDir = bootstrapDir
	}
}

func WithLogLevel(level string) Option {
	return func(cf *Config) {
		cf.logLevel = level
	}
}

func WithExposeMetrics(expose bool) Option {
	return func(c *Config) {
		c.exposeMetrics = expose
	}
}

func WithSyncCoreConfig(config *chainsync.Config) Option {
	return func(c *Config) {
		c.syncConfig = config
	}
}

func WithComplianceConfig(config *compliance.Config) Option {
	return func(c *Config) {
		c.complianceConfig = config
	}
}

func WithLockManager(lockManager lockctx.Manager) Option {
	return func(c *Config) {
		if c.lockManager != nil {
			panic("lock manager already set, cannot overwrite")
		}
		c.lockManager = lockManager
	}
}

// BootstrapNodeInfo contains the details about the upstream bootstrap peer the consensus follower uses
type BootstrapNodeInfo struct {
	Host             string // ip or hostname
	Port             uint
	NetworkPublicKey crypto.PublicKey // the network public key of the bootstrap peer
}

func bootstrapIdentities(bootstrapNodes []BootstrapNodeInfo) flow.IdentitySkeletonList {
	ids := make(flow.IdentitySkeletonList, len(bootstrapNodes))
	for i, b := range bootstrapNodes {
		ids[i] = &flow.IdentitySkeleton{
			Role:          flow.RoleAccess,
			NetworkPubKey: b.NetworkPublicKey,
			Address:       fmt.Sprintf("%s:%d", b.Host, b.Port),
			StakingPubKey: nil,
		}
	}
	return ids
}

func getFollowerServiceOptions(config *Config) []FollowerOption {
	ids := bootstrapIdentities(config.bootstrapNodes)
	return []FollowerOption{
		WithBootStrapPeers(ids...),
		WithBaseOptions(getBaseOptions(config)),
		WithNetworkKey(config.networkPrivKey),
		WithStorageLockManager(config.lockManager),
	}
}

func getBaseOptions(config *Config) []cmd.Option {
	options := []cmd.Option{
		cmd.WithMetricsEnabled(false),
		cmd.WithSecretsDBEnabled(false),
	}
	if config.bootstrapDir != "" {
		options = append(options, cmd.WithBootstrapDir(config.bootstrapDir))
	}
	if config.bindAddr != "" {
		options = append(options, cmd.WithBindAddress(config.bindAddr))
	}
	if config.protocolDB != nil {
		options = append(options, cmd.WithProtocolDB(config.protocolDB))
	}
	if config.logLevel != "" {
		options = append(options, cmd.WithLogLevel(config.logLevel))
	}
	if config.exposeMetrics {
		options = append(options, cmd.WithMetricsEnabled(config.exposeMetrics))
	}
	if config.syncConfig != nil {
		options = append(options, cmd.WithSyncCoreConfig(*config.syncConfig))
	}
	if config.complianceConfig != nil {
		options = append(options, cmd.WithComplianceConfig(*config.complianceConfig))
	}

	return options
}

func buildConsensusFollower(opts []FollowerOption) (*FollowerServiceBuilder, error) {
	nodeBuilder := FlowConsensusFollowerService(opts...)

	if err := nodeBuilder.Initialize(); err != nil {
		return nil, err
	}

	return nodeBuilder, nil
}

type ConsensusFollowerImpl struct {
	component.Component
	*cmd.NodeConfig
	logger      zerolog.Logger
	consumersMu sync.RWMutex
	consumers   []pubsub.OnBlockFinalizedConsumer
}

// NewConsensusFollower creates a new consensus follower.
func NewConsensusFollower(
	networkPrivKey crypto.PrivateKey,
	bindAddr string,
	bootstapIdentities []BootstrapNodeInfo,
	opts ...Option,
) (*ConsensusFollowerImpl, error) {
	config := &Config{
		networkPrivKey: networkPrivKey,
		bootstrapNodes: bootstapIdentities,
		bindAddr:       bindAddr,
		protocolDB:     nil,
		logLevel:       "info",
		exposeMetrics:  false,
		lockManager:    nil, // default to nil, can be set optionally with WithLockManager in tests
	}

	for _, opt := range opts {
		opt(config)
	}

	accessNodeOptions := getFollowerServiceOptions(config)
	anb, err := buildConsensusFollower(accessNodeOptions)
	if err != nil {
		return nil, err
	}

	cf := &ConsensusFollowerImpl{logger: anb.Logger}
	anb.BaseConfig.NodeRole = "consensus_follower"
	anb.FollowerDistributor.AddOnBlockFinalizedConsumer(cf.onBlockFinalized)
	cf.NodeConfig = anb.NodeConfig

	// Build will initialize the database
	cf.Component, err = anb.Build()
	if err != nil {
		return nil, err
	}

	return cf, nil
}

// onBlockFinalized relays the block finalization event to all registered consumers.
func (cf *ConsensusFollowerImpl) onBlockFinalized(finalizedBlock *model.Block) {
	cf.consumersMu.RLock()
	for _, consumer := range cf.consumers {
		cf.consumersMu.RUnlock()
		consumer(finalizedBlock)
		cf.consumersMu.RLock()
	}
	cf.consumersMu.RUnlock()
}

// AddOnBlockFinalizedConsumer adds a new block finalization subscriber.
func (cf *ConsensusFollowerImpl) AddOnBlockFinalizedConsumer(consumer pubsub.OnBlockFinalizedConsumer) {
	cf.consumersMu.Lock()
	defer cf.consumersMu.Unlock()
	cf.consumers = append(cf.consumers, consumer)
}

// Run starts the consensus follower.
// This may also be implemented directly in a calling library to take advantage of error recovery
// possible with the irrecoverable error handling.
func (cf *ConsensusFollowerImpl) Run(ctx context.Context) {
	if util.CheckClosed(ctx.Done()) {
		return
	}

	// Start the consensus follower with an irrecoverable signaler context. The returned error channel
	// will receive irrecoverable errors thrown by the consensus follower or any of its child components.
	// This makes it possible to listen for irrecoverable errors and restart the consensus follower. In
	// the default implementation, a fatal error is thrown.
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	cf.Start(signalerCtx)

	// log when the follower has complete startup and when it's beginning to shut down
	go func() {
		if err := util.WaitClosed(ctx, cf.Ready()); err != nil {
			return
		}
		cf.logger.Info().Msg("Consensus follower startup complete")
	}()

	go func() {
		<-ctx.Done()
		cf.logger.Info().Msg("Consensus follower shutting down")
	}()

	// Block here until all components have stopped or an irrecoverable error is received.
	if err := util.WaitError(errChan, cf.Done()); err != nil {
		cf.logger.Fatal().Err(err).Msg("A fatal error was encountered in consensus follower")
	}
	cf.logger.Info().Msg("Consensus follower shutdown complete")
}
