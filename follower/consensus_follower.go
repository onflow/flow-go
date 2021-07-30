package consensus_follower

import (
	"crypto"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// ConsensusFollower is a standalone module run by third parties which provides
// a mechanism for observing the block chain. It maintains a set of subscribers
// and delivers block proposals broadcasted by the consensus nodes to each one.
type ConsensusFollower interface {
	// Run starts the consensus follower.
	Run()

	// Subscribe adds a new consensus subscriber.
	Subscribe(ConsensusFollowerSubscriber)
}

// ConsensusFollowerSubscriber is the interface subscribers must implement to subscribe to the ConsensusFollower.
type ConsensusFollowerSubscriber interface {
	// NewBlockProposal is called by ConsensusFollower upon each new block proposal it receives from consensus.
	NewBlockProposal(*flow.Header, uint64)
}

type ConsensusFollowerImpl struct {
	ConsensusFollower
	relayer blockProposalRelayer
	options ConsensusFollowerOptions
}

type ConsensusFollowerOptions struct {
	upstreamNodeID string            // node ID of the upstream access node
	bindAddr       string            // network address to bind on
	networkKey     crypto.PrivateKey // network private key
	datadir        string            // directory to store the protocol state
}

func NewConsensusFollower(opts ConsensusFollowerOptions) ConsensusFollower {
	return &ConsensusFollowerImpl{
		options: opts,
	}
}

func (cf *ConsensusFollowerImpl) Run() {
	// TODO:
	// init network
	// init db
	// create block proposal relayer
	// startup follower engine and pass in relayer
	// startup sync engine
}

func (cf *ConsensusFollowerImpl) Subscribe(subscriber ConsensusFollowerSubscriber) {
	cf.relayer.AddFollower(subscriber)
}

// blockProposalRelayer is what we pass in to the follower engine.
// It keeps track of ConsensusFollower's subscribers, and relays all block proposals
// it receives to each of the subscribers.
type blockProposalRelayer struct {
	module.HotStuffFollower

	followersMu sync.RWMutex
	followers   map[ConsensusFollowerSubscriber]struct{} // stores registered followers
}

// AddFollower registers a new follower with the block proposal relayer.
func (r *blockProposalRelayer) AddFollower(follower ConsensusFollowerSubscriber) {
	r.followersMu.Lock()
	defer r.followersMu.Unlock()

	r.followers[follower] = struct{}{}
}

// SubmitProposal feeds a new block proposal into the block proposal relayer.
// The relayer will forward this block proposal to all registered followers.
func (r *blockProposalRelayer) SubmitProposal(proposal *flow.Header, parentView uint64) {
	r.followersMu.RLock()
	for follower := range r.followers {
		r.followersMu.RUnlock()

		// TODO: introduce a queueing mechanism or make this call asynchronous
		follower.NewBlockProposal(proposal, parentView)

		r.followersMu.RLock()
	}
	r.followersMu.RUnlock()
}

func (r *blockProposalRelayer) Ready() <-chan struct{} {
	// TODO: only start relaying proposals after this is called
	ready := make(chan struct{})
	close(ready)
	return ready
}

func (r *blockProposalRelayer) Done() <-chan struct{} {
	// TODO: stop start relaying proposals after this is called
	done := make(chan struct{})
	close(done)
	return done
}
