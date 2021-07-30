package consensus_follower

import (
	"crypto"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type ConsensusFollower interface {
	Run()
	Subscribe(module.HotStuffFollower)
}

type ConsensusFollowerImpl struct {
	relayer blockProposalRelayer
	options ConsensusFollowerOptions
}

type ConsensusFollowerOptions struct {
	upstreamNodeID string
	bindAddr       string
	networkKey     crypto.PrivateKey
	datadir        string
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

func (cf *ConsensusFollowerImpl) Subscribe(subscriber module.HotStuffFollower) {
	cf.relayer.AddFollower(subscriber)
}

// blockProposalRelayer is what we pass in to the follower engine.
// It keeps track of ConsensusFollower's subscribers, and relays all block proposals
// it receives to each of the subscribers.
type blockProposalRelayer struct {
	module.HotStuffFollower

	followersMu sync.RWMutex
	followers   map[module.HotStuffFollower]struct{} // stores registered followers
}

func (r *blockProposalRelayer) AddFollower(follower module.HotStuffFollower) {
	r.followersMu.Lock()
	defer r.followersMu.Unlock()

	r.followers[follower] = struct{}{}
}

func (r *blockProposalRelayer) SubmitProposal(proposal *flow.Header, parentView uint64) {
	r.followersMu.RLock()
	for follower := range r.followers {
		r.followersMu.RUnlock()
		// TODO: introduce a queueing mechanism or make this call asynchronously
		follower.SubmitProposal(proposal, parentView)
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
