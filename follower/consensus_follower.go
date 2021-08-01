package consensus_follower

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/onflow/flow-go/cmd"
	access "github.com/onflow/flow-go/cmd/access" // TODO: fix this
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine/common/follower"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	storage "github.com/onflow/flow-go/storage/badger"
)

// ConsensusFollower is a standalone module run by third parties which provides
// a mechanism for observing the block chain. It maintains a set of subscribers
// and delivers block proposals broadcasted by the consensus nodes to each one.
type ConsensusFollower interface {
	// Run starts the consensus follower.
	Run()
	// AddSubscriber adds a new consensus subscriber.
	AddSubscriber(Subscriber)
}

// TODO: allow per event type subscriptions

// Subscriber is the interface subscribers must implement to subscribe to the ConsensusFollower.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Subscriber interface {
	// OnReceiveProposal is called by ConsensusFollower when it receives a new block proposal.
	OnReceiveProposal(proposal *flow.Header, parentView uint64)
	// OnFinalizedBlock is called by ConsensusFollower when a block is finalized.
	OnFinalizedBlock(finalizedBlockID flow.Identifier)
}

type consensusFollowerImpl struct {
	nodeBuilder   cmd.NodeBuilder
	subscribersMu sync.RWMutex
	subscribers   map[Subscriber]struct{}

	// options
	nodeID                flow.Identifier
	upstreamAccessNodeID  flow.Identifier // node ID of the upstream access node
	bindAddr              string          // network address to bind on
	dataDir               string          // directory to store the protocol state
	bootstrapDir          string
	networkConnectTimeout time.Duration // how long to try connecting to the network
}

type ConsensusFollowerOption func(*consensusFollowerImpl)

func WithDataDir(dataDir string) ConsensusFollowerOption {
	return func(cf *consensusFollowerImpl) {
		cf.dataDir = dataDir
	}
}

func WithBootstrapDir(bootstrapDir string) ConsensusFollowerOption {
	return func(cf *consensusFollowerImpl) {
		cf.bootstrapDir = bootstrapDir
	}
}

func WithNetworkConnectTimeout(timeout time.Duration) ConsensusFollowerOption {
	return func(cf *consensusFollowerImpl) {
		cf.networkConnectTimeout = timeout
	}
}

func NewConsensusFollower(nodeID flow.Identifier, upstreamAccessNodeID flow.Identifier, bindAddr string, opts ...ConsensusFollowerOption) ConsensusFollower {
	const (
		defaultBootstrapDir          = "bootstrap"
		defaultNetworkConnectTimeout = time.Minute
	)

	homedir, _ := os.UserHomeDir() // TODO: handle error here
	defaultDataDir := filepath.Join(homedir, ".flow", "database")

	nodeBuilder := access.UnstakedAccessNode(access.FlowAccessNode())
	consensusFollower := &consensusFollowerImpl{
		nodeBuilder:           nodeBuilder,
		subscribers:           make(map[Subscriber]struct{}),
		nodeID:                nodeID,
		upstreamAccessNodeID:  upstreamAccessNodeID,
		bindAddr:              bindAddr,
		dataDir:               defaultDataDir,
		bootstrapDir:          defaultBootstrapDir,
		networkConnectTimeout: defaultNetworkConnectTimeout,
	}

	for _, opt := range opts {
		opt(consensusFollower)
	}

	finalizationDistributor := pubsub.NewFinalizationDistributor()
	finalizationDistributor.AddOnBlockFinalizedConsumer(consensusFollower.onBlockFinalized)

	var (
		followerState protocol.MutableState
		followerCore  module.HotStuffFollower
		relayer       *eventRelayer
		finalized     *flow.Header
		pending       []*flow.Header
		syncCore      *synchronization.Core
		followerEng   *follower.Engine
		committee     hotstuff.Committee
		err           error
	)

	// TODO: need some way to override  baseconfig AND stakeAccessNodIDHex

	// nodeBuilder.FlowAccessNodeBuilder
	//
	// staked                  bool
	// stakedAccessNodeIDHex   string
	// unstakedNetworkBindAddr string
	// _____ UnstakedNetwork         *p2p.Network
	// _____ unstakedMiddleware      *p2p.Middleware

	// nodeBuilder.BaseConfig
	//
	// nodeIDHex             string // pass flow.Identifier instead
	// timeout               time.Duration // component startup / shutdown timeout
	// datadir               string // "directory to store the protocol state"
	// BootstrapDir          string
	//
	// level                 string // log level. Can we use a child logger instead? // pass in logger using PreInit.
	// metricsPort           uint // port for metrics server. Can we disable this?
	//
	// unicastMessageTimeout time.Duration p2p.DefaultUnicastTimeout // TODO: need to update FlowAccessNodeBuilder.initMiddleware to actually use this
	// profilerEnabled       bool // Set this to false
	// tracerEnabled         bool // set False
	// _____(not needed) bindAddr              string
	// _____(already given) NodeRole              string
	// _____(overridden) peerUpdateInterval    time.Duration
	// _____(not needed since we set profilerEnabled false) profilerDir           string
	// _____(not needed) profilerInterval      time.Duration
	// _____(not needed) profilerDuration      time.Duration

	nodeBuilder.
		Initialize().
		Module("mutable follower state", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currenlty supported but read-only state has type %T", node.State)
			}

			followerState, err = badgerState.NewFollowerState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
				blocktimer.DefaultBlockTimer,
			)

			return err
		}).
		Module("sync core", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			syncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Module("committee", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			committee, err = committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			return err
		}).
		Module("latest header", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			finalized, pending, err = recovery.FindLatest(node.State, node.Storage.Headers)
			return err
		}).
		Component("follower core", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// create a finalizer that will handle updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, followerState)

			// initialize the staking & beacon verifiers, signature joiner
			staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
			beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
			merger := signature.NewCombiner(encodable.ConsensusVoteSigLen, encodable.RandomBeaconSigLen)

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(committee, staking, beacon, merger)

			followerCore, err = consensus.NewFollower(node.Logger, committee, node.Storage.Headers, final, verifier,
				finalizationDistributor, node.RootBlock.Header, node.RootQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not initialize follower core: %w", err)
			}

			return followerCore, nil
		}).
		Component("event relayer", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			relayer = newEventRelayer(followerCore, consensusFollower.onBlockProposal)
			return relayer, nil
		}).
		Component("follower engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// initialize cleaner for DB
			cleaner := storage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)
			conCache := buffer.NewPendingBlocks()

			followerEng, err = follower.New(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				followerState,
				conCache,
				relayer,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			return followerEng, nil
		}).
		Component("sync engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			sync, err := synceng.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				followerEng,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			finalizationDistributor.AddOnBlockFinalizedConsumer(sync.OnFinalizedBlock)

			return sync, nil
		})

	return consensusFollower
}

// TODO: need to add ctx so we can cancel things
// TODO: find a way to catch log.Fatal or panic and return error from Run()?
func (cf *consensusFollowerImpl) Run() {
	cf.nodeBuilder.Run()
}

func (cf *consensusFollowerImpl) AddSubscriber(subscriber Subscriber) {
	cf.subscribersMu.Lock()
	defer cf.subscribersMu.Unlock()

	cf.subscribers[subscriber] = struct{}{}
}

// onBlockProposal relays the given block proposal to all registered subscribers.
func (cf *consensusFollowerImpl) onBlockProposal(proposal *flow.Header, parentView uint64) {
	cf.subscribersMu.RLock()
	for subscriber := range cf.subscribers {
		cf.subscribersMu.RUnlock()

		// TODO: introduce a queueing mechanism or make this call asynchronous
		// make this entire loop async
		subscriber.OnReceiveProposal(proposal, parentView)

		cf.subscribersMu.RLock()
	}
	cf.subscribersMu.RUnlock()
}

// onBlockFinalized relays the block finalization event to all registered subscribers
func (cf *consensusFollowerImpl) onBlockFinalized(finalizedBlockID flow.Identifier) {
	cf.subscribersMu.RLock()
	for subscriber := range cf.subscribers {
		cf.subscribersMu.RUnlock()

		// TODO: introduce a queueing mechanism or make this call asynchronous
		// make this entire loop async
		subscriber.OnFinalizedBlock(finalizedBlockID)

		cf.subscribersMu.RLock()
	}
	cf.subscribersMu.RUnlock()
}

// eventRelayer is the implementation of module.HotStuffFollower that we pass in to the follower engine.
// It receives block proposals submitted from the follower engine and feeds them to both the wrapped
// follower and the callback function.
type eventRelayer struct {
	follower              module.HotStuffFollower
	blockProposalCallback func(*flow.Header, uint64)
	lm                    *lifecycle.LifecycleManager
}

func newEventRelayer(follower module.HotStuffFollower, onBlockProposal func(*flow.Header, uint64)) *eventRelayer {
	return &eventRelayer{
		follower:              follower,
		blockProposalCallback: onBlockProposal,
		lm:                    lifecycle.NewLifecycleManager(),
	}
}

func (r *eventRelayer) SubmitProposal(proposal *flow.Header, parentView uint64) {
	select {
	case <-r.lm.Started():
		r.blockProposalCallback(proposal, parentView)
		r.follower.SubmitProposal(proposal, parentView)
	default:
		// not yet started
		return
	}
}

func (r *eventRelayer) Ready() <-chan struct{} {
	r.lm.OnStart(func() {
		// wait for the wrapped follower to startup
		<-r.follower.Ready()
	})
	return r.lm.Started()
}

func (r *eventRelayer) Done() <-chan struct{} {
	r.lm.OnStop(func() {
		// wait for the wrapped follower to shutdown
		<-r.follower.Done()
	})
	return r.lm.Stopped()
}
