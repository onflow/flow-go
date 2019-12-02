// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"bytes"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff/chain"
	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff/hasher"
	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff/round"
	"github.com/dapperlabs/flow-go/model/coldstuff"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
)

// Engine implements a simulated consensus algorithm. It does a lot of things
// that a real consensus algorithm would do, such as sending proposals, voting
// on them and finally committing blocks; however, it does so in a
// cryptographically insecure and naive way and could easily be broken.
type Engine struct {
	log       zerolog.Logger
	con       network.Conduit
	com       module.Committee
	pool      module.Mempool
	hash      Hasher
	round     Round
	chain     Chain
	interval  time.Duration
	timeout   time.Duration
	targetIDs []string
	proposals chan proposalWrap
	votes     chan voteWrap
	commits   chan commitWrap
	once      *sync.Once
	wg        *sync.WaitGroup
	stop      chan struct{}
}

// New creates a new coldstuff engine.
func New(log zerolog.Logger, net module.Network, com module.Committee, pool module.Mempool) (*Engine, error) {

	// we will round the IDs of all consensus nodes beyond ourselves
	targets := com.Select().
		Filter(identity.Role(identity.Consensus)).
		Filter(identity.Not(identity.NodeID(com.Me().NodeID)))
	if len(targets) < 3 {
		return nil, errors.Errorf("need at least 3 other consensus nodes (have: %d)", len(targets))
	}

	// define fake genesis block
	genesis := &coldstuff.BlockHeader{
		Height:    0,
		Nonce:     1337,
		Parent:    []byte{},
		Timestamp: time.Unix(1568624400, 0).UTC(),
		Payload:   []byte{},
	}

	// initialize the engine with dependencies
	e := &Engine{
		log:       log,
		com:       com,
		pool:      pool,
		hash:      hasher.NewEasy(),
		round:     round.NewSimple(),
		chain:     chain.NewDumb(genesis),
		interval:  4 * time.Second,
		timeout:   8 * time.Second,
		targetIDs: targets.NodeIDs(),
		proposals: make(chan proposalWrap, 1),
		votes:     make(chan voteWrap, 1),
		commits:   make(chan commitWrap, 1),
		once:      &sync.Once{},
		wg:        &sync.WaitGroup{},
		stop:      make(chan struct{}),
	}

	// register the engine with the network layer to get our conduit
	con, err := net.Register(engine.SimulationColdstuff, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	e.con = con

	return e, nil
}

// Ready returns a channel that will close when the coldstuff engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	ready := make(chan struct{})
	e.wg.Add(1)
	go e.consent()
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when the coldstuff engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		e.abort()
		e.wg.Wait()
		close(done)
	}()
	return done
}

// Submit will submit an event for processing in a non-blocking manner.
func (e *Engine) Submit(event interface{}) {
	err := e.Process(e.com.Me().NodeID, event)
	if err != nil {
		e.log.Error().
			Err(err).
			Msg("could not process submitted event")
	}
}

// Identify will identify the given event with a unique ID.
func (e *Engine) Identify(event interface{}) ([]byte, error) {
	switch event.(type) {
	default:
		return nil, errors.Errorf("invalid event type (%T)", event)
	}
}

// Retrieve will retrieve an event by the given unique ID.
func (e *Engine) Retrieve(eventID []byte) (interface{}, error) {
	return nil, errors.New("not implemented")
}

// Process will process an event submitted to the engine in a blocking fashion.
func (e *Engine) Process(originID string, event interface{}) error {
	var err error
	switch ev := event.(type) {
	case *coldstuff.BlockProposal:
		e.proposals <- proposalWrap{originID: originID, proposal: ev}
	case *coldstuff.BlockVote:
		e.votes <- voteWrap{originID: originID, vote: ev}
	case *coldstuff.BlockCommit:
		e.commits <- commitWrap{originID: originID, commit: ev}
	default:
		err = errors.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}
	return nil
}

// abort will shut down the coldstuff engine if it wasn't already done before.
func (e *Engine) abort() {
	e.once.Do(func() {
		close(e.stop)
	})
}

// consent will start the consensus algorithm on the engine.
func (e *Engine) consent() {

	// defer done on waitgroup to unblock shutdown when stopping
	defer e.wg.Done()

	myID := e.com.Me().NodeID
	log := e.log.With().Logger()

	// each iteration of the loop is one block
ConsentLoop:
	for {

		// get the last header to determine what is the earliest time to mine
		// the next valid block
		head := e.chain.Head()
		next := head.Timestamp.Add(e.interval)

		// check if we should quit
		select {
		case <-time.After(time.Until(next)):

			// reset the round for the consensus round
			e.round.Reset()

			// figure out current round leader
			round := head.Height + 1
			leader := e.com.Leader(round)

			// if we are the leader, we:
			// 1) send block proposal
			// 2) wait for block votes
			// 3) send block commit
			// 4) commit candidate block
			if leader.NodeID == myID {

				err := e.sendProposal()
				if err != nil {
					log.Error().Err(err).Msg("could not send proposal")
					continue
				}

				err = e.waitForVotes()
				if err != nil {
					log.Error().Err(err).Msg("could not get votes")
					continue
				}

				err = e.sendCommit()
				if err != nil {
					log.Error().Err(err).Msg("could not send commit")
					continue
				}

				err = e.commitCandidate()
				if err != nil {
					log.Error().Err(err).Msg("could not commit candidate")
					continue
				}

				continue
			}

			// if we are not the leader, we:
			// 1) wait for block proposal
			// 2) vote on block proposal
			// 3) wait for block commit
			// 4) commit candidate block
			err := e.waitForProposal()
			if err != nil {
				log.Error().Err(err).Msg("could not get proposal")
				continue
			}

			err = e.voteOnProposal()
			if err != nil {
				log.Error().Err(err).Msg("could not vote on proposal")
				continue
			}

			err = e.waitForCommit()
			if err != nil {
				log.Error().Err(err).Msg("could not get commit")
				continue
			}

			err = e.commitCandidate()
			if err != nil {
				log.Error().Err(err).Msg("could not commit candidate")
				continue
			}

		case <-e.stop:
			break ConsentLoop
		}
	}
}

// sendProposal will build a new block, cache it as the current candidate for
// consensus and propagate it to the other consensus nodes. It assumes that we
// are the leader for the current round.
func (e *Engine) sendProposal() error {

	// for the block proposal, we use the mempool hash as the payload, the hash
	// of the previous parent, a height increased by one, the current timestamp
	// and a random nonce
	payload := e.pool.Hash()
	parent := e.chain.Head()
	header := &coldstuff.BlockHeader{
		Height:    parent.Height + 1,
		Nonce:     rand.Uint64(),
		Timestamp: time.Now().UTC(),
		Parent:    e.hash.BlockHash(parent),
		Payload:   payload,
	}

	e.round.Propose(header)
	hash := e.hash.BlockHash(header)

	// build proposal message and send to other nodes
	proposal := &coldstuff.BlockProposal{
		Header: header,
	}
	err := e.con.Submit(proposal, e.targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not submit proposal")
	}

	e.log.Info().Hex("block_hash", hash).Uint64("block_height", header.Height).Msg("block proposal sent")

	return nil
}

// waitForVotes will wait for received votes and validate them until we have
// reached a quorum on the currently cached block candidate. It assumse we are
// the leader and will timeout after the configured timeout.
func (e *Engine) waitForVotes() error {

	for {
		select {

		// process each vote that we receive
		case w := <-e.votes:
			voterID, vote := w.originID, w.vote

			// discard votes by double voters
			voted := e.round.Voted(voterID)
			if voted {
				log.Warn().Str("voter_id", voterID).Msg("invalid double vote")
				continue
			}

			// discard votes by self
			me := e.com.Me().NodeID
			if voterID == me {
				log.Warn().Str("voter_id", voterID).Msg("invalid self-vote")
				continue
			}

			// discard votes that are not by consensus nodes
			ids := e.com.Select().
				Filter(identity.Role("consensus")).
				Filter(identity.NodeID(voterID))
			if len(ids) == 0 {
				log.Warn().Str("voter_id", voterID).Msg("invalid non-consensus vote")
				continue
			}

			// discard votes that are on the wrong candidate
			candidate := e.round.Candidate()
			hash := e.hash.BlockHash(candidate)
			if !bytes.Equal(hash, vote.Hash) {
				log.Warn().Hex("vote_hash", vote.Hash).Hex("candidate_hash", hash).Msg("invalid vote hash")
				continue
			}

			// tally the vote and check if we have enough now
			e.round.Tally(voterID)
			votes := e.round.Votes()

			e.log.Info().Hex("block_hash", hash).Uint("vote_count", votes).Msg("block vote received")

			quorum := e.com.Quorum()
			if votes >= e.com.Quorum() {
				e.log.Info().Hex("block_hash", hash).Uint("vote_quorum", quorum).Msg("vote quorum reached")
				return nil
			}

		case <-time.After(e.timeout):
			return errors.New("timed out while waiting for votes")
		}
	}
}

// sendCommit is called after we have successfully waited for a vote quorum. It
// will send a block commit message with the block hash that instructs all nodes
// to forward their blockchain and start a new consensus round.
func (e *Engine) sendCommit() error {

	// get the cached candidate
	candidate := e.round.Candidate()

	// send the commit message to the network
	hash := e.hash.BlockHash(candidate)
	commit := &coldstuff.BlockCommit{
		Hash: hash,
	}
	err := e.con.Submit(commit, e.targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not submit commit")
	}

	log.Info().Hex("block_hash", hash).Msg("block commit sent")

	return nil
}

// waitForProposal waits for a block proposal to be received and validates it in
// a number of ways. It should be called at the beginning of a round if we are
// not the leader. It will timeout if no proposal was received by the leader
// after the configured timeout.
func (e *Engine) waitForProposal() error {

	log := e.log.With().Str("action", "wait_for_proposal").Logger()

	// get current head to calculate round height and leader
	parent := e.chain.Head()
	height := parent.Height + 1
	phash := e.hash.BlockHash(parent)
	leaderID := e.com.Leader(height).NodeID

	for {
		select {

		// process each proposal we receive
		case w := <-e.proposals:
			proposerID, proposal := w.originID, w.proposal
			candidate := proposal.Header

			// discard proposals by non-leaders
			if proposerID != leaderID {
				log.Warn().Str("candidate_leader", proposerID).Str("expected_leader", leaderID).Msg("invalid leader")
				continue
			}

			// discard proposals with the wrong height
			if candidate.Height != height {
				log.Warn().Uint64("candidate_height", candidate.Height).Uint64("expected_height", height).Msg("invalid height")
				continue
			}

			// discard proposals with the wrong parent
			if !bytes.Equal(candidate.Parent, phash) {
				log.Warn().Hex("candidate_parent", candidate.Parent).Hex("expected_parent", phash).Msg("invalid parent")
				continue
			}

			// discard proposals with invalid timestamp
			timestamp := candidate.Timestamp
			cutoff := parent.Timestamp.Add(e.interval)
			if timestamp.Before(cutoff) {
				log.Warn().Time("candidate_timestamp", timestamp).Time("candidate_cutoff", cutoff).Msg("invalid timestamp")
				continue
			}

			// store the candidate in round
			e.round.Propose(candidate)
			hash := e.hash.BlockHash(candidate)

			e.log.Info().Hex("block_hash", hash).Uint64("block_height", candidate.Height).Msg("block proposal received")

			return nil

		case <-time.After(e.timeout):
			return errors.New("timed out while waiting for proposal")
		}
	}
}

// voteOnProposal is called after we have received a new block proposal as
// non-leader. It assumes that all checks were already done and simply sends a
// vote to the leader of the current round that accepts the candidate block.
func (e *Engine) voteOnProposal() error {

	// get current leader
	parent := e.chain.Head()
	height := parent.Height + 1
	leader := e.com.Leader(height)

	// send vote for proposal to leader
	candidate := e.round.Candidate()
	hash := e.hash.BlockHash(candidate)
	vote := &coldstuff.BlockVote{
		Hash: hash,
	}
	err := e.con.Submit(vote, leader.NodeID)
	if err != nil {
		return errors.Wrap(err, "could not submit vote")
	}

	e.log.Info().Hex("block_hash", hash).Msg("block vote sent")

	return nil
}

// waitForCommit is called after we have submitted our vote for the leader and
// awaits his confirmation that we can commit the block. The confirmation is
// only sent once a quorum of votes was received by the leader.
func (e *Engine) waitForCommit() error {

	// get parent hash and leader ID
	parent := e.chain.Head()
	height := parent.Height + 1
	leaderID := e.com.Leader(height).NodeID
	candidate := e.round.Candidate()
	hash := e.hash.BlockHash(candidate)

	for {
		select {
		case w := <-e.commits:
			proposerID, commit := w.originID, w.commit

			// discard commits not from leader
			if proposerID != leaderID {
				log.Warn().Str("commit_leader", proposerID).Str("expected_leader", leaderID).Msg("invalid commit leader")
				continue
			}

			// discard commits not for candidate hash
			if !bytes.Equal(commit.Hash, hash) {
				log.Warn().Hex("commit_hash", commit.Hash).Hex("expected_hash", hash).Msg("invalid commit hash")
				continue
			}

			e.log.Info().Hex("block_hash", commit.Hash).Msg("block commit received")

			return nil

		case <-time.After(e.timeout):
			return errors.New("timed out while waiting for commit")
		}
	}
}

// commitCandidate commits the current block candidate to the blockchain and
// starts the next consensus round.
func (e *Engine) commitCandidate() error {

	// commit the block to our chain state
	candidate := e.round.Candidate()
	err := e.chain.Commit(candidate)
	if err != nil {
		return errors.Wrap(err, "could not commit candidate")
	}

	// for now, just drop all collections
	hash := e.hash.BlockHash(candidate)
	e.pool.Drop()

	log.Info().Hex("block_hash", hash).Uint64("new_height", e.chain.Head().Height).Msg("block candidate committed")

	return nil
}
