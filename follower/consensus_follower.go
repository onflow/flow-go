package follower

import (
	"context"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd"
	access "github.com/onflow/flow-go/cmd/access/node_builder"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
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
	networkPrivKey crypto.PrivateKey   // the network private key of this node
	bootstrapNodes []BootstrapNodeInfo // the bootstrap nodes to use
	bindAddr       string              // address to bind on
	db             *badger.DB          // the badger DB storage to use for the protocol state
	dataDir        string              // directory to store the protocol state (if the badger storage is not provided)
	bootstrapDir   string              // path to the bootstrap directory
	logLevel       string              // log level
	exposeMetrics  bool                // whether to expose metrics
}

type Option func(c *Config)

// WithDataDir sets the underlying directory to be used to store the database
// If a database is supplied, then data directory will be set to empty string
func WithDataDir(dataDir string) Option {
	return func(cf *Config) {
		if cf.db == nil {
			cf.dataDir = dataDir
		}
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

// WithDB sets the underlying database that will be used to store the chain state
// WithDB takes precedence over WithDataDir and datadir will be set to empty if DB is set using this option
func WithDB(db *badger.DB) Option {
	return func(cf *Config) {
		cf.db = db
		cf.dataDir = ""
	}
}

func WithExposeMetrics(expose bool) Option {
	return func(c *Config) {
		c.exposeMetrics = expose
	}
}

// BootstrapNodeInfo contains the details about the upstream bootstrap peer the consensus follower uses
type BootstrapNodeInfo struct {
	Host             string // ip or hostname
	Port             uint
	NetworkPublicKey crypto.PublicKey // the network public key of the bootstrap peer
}

func bootstrapIdentities(bootstrapNodes []BootstrapNodeInfo) flow.IdentityList {
	ids := make(flow.IdentityList, len(bootstrapNodes))
	for i, b := range bootstrapNodes {
		ids[i] = &flow.Identity{
			Role:          flow.RoleAccess,
			NetworkPubKey: b.NetworkPublicKey,
			Address:       fmt.Sprintf("%s:%d", b.Host, b.Port),
			StakingPubKey: nil,
		}
	}
	return ids
}

func getAccessNodeOptions(config *Config) []access.Option {
	ids := bootstrapIdentities(config.bootstrapNodes)
	return []access.Option{
		access.WithBootStrapPeers(ids...),
		access.WithBaseOptions(getBaseOptions(config)),
		access.WithNetworkKey(config.networkPrivKey),
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
	if config.dataDir != "" {
		options = append(options, cmd.WithDataDir(config.dataDir))
	}
	if config.bindAddr != "" {
		options = append(options, cmd.WithBindAddress(config.bindAddr))
	}
	if config.logLevel != "" {
		options = append(options, cmd.WithLogLevel(config.logLevel))
	}
	if config.db != nil {
		options = append(options, cmd.WithDB(config.db))
	}
	if config.exposeMetrics {
		options = append(options, cmd.WithMetricsEnabled(config.exposeMetrics))
	}

	return options
}

func buildAccessNode(accessNodeOptions []access.Option) (*access.UnstakedAccessNodeBuilder, error) {
	anb := access.FlowAccessNode(accessNodeOptions...)
	nodeBuilder := access.NewUnstakedAccessNodeBuilder(anb)

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
		logLevel:       "info",
		exposeMetrics:  false,
	}

	for _, opt := range opts {
		opt(config)
	}

	accessNodeOptions := getAccessNodeOptions(config)
	anb, err := buildAccessNode(accessNodeOptions)
	if err != nil {
		return nil, err
	}

	cf := &ConsensusFollowerImpl{logger: anb.Logger}
	anb.BaseConfig.NodeRole = "consensus_follower"
	anb.FinalizationDistributor.AddOnBlockFinalizedConsumer(cf.onBlockFinalized)
	cf.NodeConfig = anb.NodeConfig
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
