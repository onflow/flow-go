// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"bytes"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff/round"
	"github.com/dapperlabs/flow-go/model/coldstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
)

// Engine implements a simulated consensus algorithm. It's similar to a
// one-chain BFT consensus algorithm, finalizing blocks immediately upon
// collecting the first quorum. In order to keep nodes in sync, the quorum is
// set at the totality of the stake in the network.
type Engine struct {
	log       zerolog.Logger
	con       network.Conduit
	state     protocol.State
	me        module.Local
	pool      module.CollectionPool
	round     Round
	interval  time.Duration
	timeout   time.Duration
	proposals chan proposalWrap
	votes     chan voteWrap
	commits   chan commitWrap
	once      *sync.Once
	wg        *sync.WaitGroup
	stop      chan struct{}
}

// New initializes a new coldstuff consensus engine, using the injected network
// and the injected memory pool to forward the injected protocol state.
func New(log zerolog.Logger, net module.Network, state protocol.State, me module.Local, pool module.CollectionPool) (*Engine, error) {

	// initialize the engine with dependencies
	e := &Engine{
		log:       log.With().Str("engine", "coldstuff").Logger(),
		state:     state,
		me:        me,
		pool:      pool,
		round:     nil, // initialized for each consensus round
		interval:  4 * time.Second,
		timeout:   1 * time.Second,
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

// Submit can be used to submit events for processing locally. It logs errors
// internally and doesn't return them, so it can be used by other engines to
// forward events in a non-blocking manner by using a goroutine.
func (e *Engine) Submit(event interface{}) {
	err := e.Process(e.me.NodeID(), event)
	if err != nil {
		e.log.Error().Err(err).Msg("could not process submitted event")
	}
}

// Process will process coldstuff consensus events from the given origin.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	var err error
	switch ev := event.(type) {
	case *coldstuff.BlockProposal:
		e.proposals <- proposalWrap{originID: originID, block: ev.Block}
	case *coldstuff.BlockVote:
		e.votes <- voteWrap{originID: originID, hash: ev.Hash}
	case *coldstuff.BlockCommit:
		e.commits <- commitWrap{originID: originID, hash: ev.Hash}
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

// consent will start the consensus algorithm on the engine. As we need to
// process events sequentially, all submissions are queued in channels and then
// processed here.
func (e *Engine) consent() {

	// defer done on waitgroup to unblock shutdown when stopping
	defer e.wg.Done()

	localID := e.me.NodeID()
	log := e.log.With().Hex("local_id", localID[:]).Logger()

	// each iteration of the loop represents one (successful or failed) round of
	// the consensus algorithm
ConsentLoop:
	for {

		// initialize and cache immutable parameters for the current round
		var err error
		e.round, err = round.New(e.state, e.me)
		if err != nil {
			log.Error().Err(err).Msg("could not initialize round")
			break
		}

		// calculate the time at which we can generate the next valid block
		limit := e.round.Parent().Timestamp.Add(e.interval)

		select {

		// break the loop and shut down
		case <-e.stop:
			break ConsentLoop

		// start the next consensus round
		case <-time.After(time.Until(limit)):

			if e.round.Leader().NodeID == localID {
				// if we are the leader, we:
				// 1) send a block proposal
				// 2) wait for sufficient block votes
				// 3) send a block commit

				err = e.sendProposal()
				if err != nil {
					log.Error().Err(err).Msg("could not send proposal")
					continue ConsentLoop
				}

				err = e.waitForVotes()
				if err != nil {
					log.Error().Err(err).Msg("could not receive votes")
					continue ConsentLoop
				}

				err = e.sendCommit()
				if err != nil {
					log.Error().Err(err).Msg("could not send commit")
					continue ConsentLoop
				}

			} else {
				// if we are not the leader, we:
				// 1) wait for a block proposal
				// 2) vote on the block proposal
				// 3) wait for a block commit

				err = e.waitForProposal()
				if err != nil {
					log.Error().Err(err).Msg("could not receive proposal")
					continue ConsentLoop
				}

				err = e.voteOnProposal()
				if err != nil {
					log.Error().Err(err).Msg("could not vote on proposal")
					continue ConsentLoop
				}

				err = e.waitForCommit()
				if err != nil {
					log.Error().Err(err).Msg("could not receive commit")
					continue ConsentLoop
				}
			}

			// regardless of path, if we successfully reach here, we finished a
			// full successful consensus round and can commit the current
			// block candidate
			err = e.commitCandidate()
			if err != nil {
				log.Error().Err(err).Msg("could not commit candidate")
				continue
			}
		}
	}
}

// sendProposal will build a new block, cache it as the current candidate for
// consensus and propagate it to the other consensus nodes. It assumes that we
// are the leader for the current round.
func (e *Engine) sendProposal() error {

	log := e.log.With().
		Str("action", "send_proposal").
		Logger()

	// get our own ID to tally our stake
	id, err := e.state.Final().Identity(e.me.NodeID())
	if err != nil {
		return errors.Wrap(err, "could not get own current ID")
	}

	// create the next header
	header := flow.Header{
		Number:    e.round.Parent().Number + 1,
		Parent:    e.round.Parent().Hash(),
		Timestamp: time.Now().UTC(),
	}

	// create a block with the current mempool collections as payload
	collections := e.pool.All()
	candidate := &flow.Block{
		Header:                header,
		NewIdentities:         flow.IdentityList{},
		GuaranteedCollections: collections,
	}

	// fill in the header payload hash
	candidate.Header.Payload = candidate.Payload()

	log = log.With().
		Uint64("number", candidate.Number).
		Int("collections", len(collections)).
		Hex("hash", candidate.Hash()).
		Logger()

	// cache the candidate block
	e.round.Propose(candidate)

	// send the block proposal
	proposal := &coldstuff.BlockProposal{
		Block: candidate,
	}
	err = e.con.Submit(proposal, e.round.Participants().NodeIDs()...)
	if err != nil {
		return errors.Wrap(err, "could not submit proposal")
	}

	// add our own vote to the engine
	e.round.Tally(id.NodeID, id.Stake)

	log.Info().Msg("block proposal sent")

	return nil
}

// waitForVotes will wait for received votes and validate them until we have
// reached a quorum on the currently cached block candidate. It assumse we are
// the leader and will timeout after the configured timeout.
func (e *Engine) waitForVotes() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Number).
		Hex("hash", candidate.Hash()).
		Int("collections", len(candidate.GuaranteedCollections)).
		Str("action", "wait_votes").
		Logger()

	for {
		select {

		// process each vote that we receive
		case w := <-e.votes:
			voterID, voteHash := w.originID, w.hash

			// discard votes by double voters
			voted := e.round.Voted(voterID)
			if voted {
				log.Warn().Hex("voter_id", voterID[:]).Msg("invalid double vote")
				continue
			}

			// discard votes by self
			if voterID == e.me.NodeID() {
				log.Warn().Hex("voter_id", voterID[:]).Msg("invalid self-vote")
				continue
			}

			// discard votes that are not by staked consensus nodes
			id, err := e.state.Final().Identity(voterID)
			if errors.Cause(err) == badger.ErrKeyNotFound {
				log.Warn().Hex("voter_id", voterID[:]).Msg("vote by unknown node")
				continue
			}
			if err != nil {
				log.Error().Err(err).Hex("voter_id", voterID[:]).Msg("could not verify voter ID")
				break
			}
			if id.Role != flow.RoleConsensus {
				log.Warn().Str("role", id.Role.String()).Msg("vote by non-consensus node")
				continue
			}

			// discard votes that are on the wrong candidate
			if !bytes.Equal(voteHash, candidate.Hash()) {
				log.Warn().Hex("vote_hash", voteHash).Msg("invalid candidate vote")
				continue
			}

			// tally the voting stake of the voter ID
			e.round.Tally(voterID, id.Stake)
			votes := e.round.Votes()

			log.Info().Uint64("vote_quorum", e.round.Quorum()).Uint64("vote_count", votes).Msg("block vote received")

			// if we reached the quorum, continue to next step
			if votes >= e.round.Quorum() {
				log.Info().Msg("sufficient votes received")
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

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Number).
		Hex("hash", candidate.Hash()).
		Int("collections", len(candidate.GuaranteedCollections)).
		Str("action", "send_commit").
		Logger()

	// send a commit for the cached block hash
	commit := &coldstuff.BlockCommit{
		Hash: candidate.Hash(),
	}
	err := e.con.Submit(commit, e.round.Participants().NodeIDs()...)
	if err != nil {
		return errors.Wrap(err, "could not submit commit")
	}

	log.Info().Msg("block commit sent")

	return nil
}

// waitForProposal waits for a block proposal to be received and validates it in
// a number of ways. It should be called at the beginning of a round if we are
// not the leader. It will timeout if no proposal was received by the leader
// after the configured timeout.
func (e *Engine) waitForProposal() error {

	log := e.log.With().
		Str("action", "wait_proposal").
		Logger()

	for {
		select {

		// process each proposal we receive
		case w := <-e.proposals:
			proposerID, block := w.originID, w.block

			// discard proposals by non-leaders
			leaderID := e.round.Leader().NodeID
			if proposerID != leaderID {
				log.Warn().Hex("candidate_leader", proposerID[:]).Hex("expected_leader", leaderID[:]).Msg("invalid leader")
				continue
			}

			// discard proposals with the wrong height
			number := e.round.Parent().Number + 1
			if block.Number != e.round.Parent().Number+1 {
				log.Warn().Uint64("candidate_height", block.Number).Uint64("expected_height", number).Msg("invalid height")
				continue
			}

			// discard proposals with the wrong parent
			parent := e.round.Parent().Hash()
			if !bytes.Equal(block.Parent, parent) {
				log.Warn().Hex("candidate_parent", block.Parent).Hex("expected_parent", parent).Msg("invalid parent")
				continue
			}

			// discard proposals with invalid timestamp
			limit := e.round.Parent().Timestamp.Add(e.interval)
			if block.Timestamp.Before(limit) {
				log.Warn().Time("candidate_timestamp", block.Timestamp).Time("candidate_limit", limit).Msg("invalid timestamp")
				continue
			}

			// cache the candidate for the round
			e.round.Propose(block)

			log.Info().
				Uint64("number", block.Number).
				Int("collections", len(block.GuaranteedCollections)).
				Hex("hash", block.Hash()).
				Msg("block proposal received")

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

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Number).
		Hex("hash", candidate.Hash()).
		Int("collections", len(candidate.GuaranteedCollections)).
		Str("action", "send_vote").
		Logger()

	// send vote for proposal to leader
	hash := candidate.Hash()
	vote := &coldstuff.BlockVote{
		Hash: hash,
	}
	err := e.con.Submit(vote, e.round.Leader().NodeID)
	if err != nil {
		return errors.Wrap(err, "could not submit vote")
	}

	log.Info().Msg("block vote sent")

	return nil
}

// waitForCommit is called after we have submitted our vote for the leader and
// awaits his confirmation that we can commit the block. The confirmation is
// only sent once a quorum of votes was received by the leader.
func (e *Engine) waitForCommit() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Number).
		Hex("hash", candidate.Hash()).
		Int("collections", len(candidate.GuaranteedCollections)).
		Str("action", "wait_commit").
		Logger()

	for {
		select {
		case w := <-e.commits:
			committerID, commitHash := w.originID, w.hash

			// discard commits not from leader
			leaderID := e.round.Leader().NodeID
			if committerID != leaderID {
				log.Warn().Hex("commit_leader", committerID[:]).Hex("expected_leader", leaderID[:]).Msg("invalid commit leader")
				continue
			}

			// discard commits not for candidate hash
			if !bytes.Equal(commitHash, candidate.Hash()) {
				log.Warn().Hex("commit_hash", commitHash).Msg("invalid commit hash")
				continue
			}

			log.Info().Msg("block commit received")

			return nil

		case <-time.After(e.timeout):
			return errors.New("timed out while waiting for commit")
		}
	}
}

// commitCandidate commits the current block candidate to the blockchain and
// starts the next consensus round.
func (e *Engine) commitCandidate() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Number).
		Hex("hash", candidate.Hash()).
		Int("collections", len(candidate.GuaranteedCollections)).
		Str("action", "exec_commit").
		Logger()

	// commit the block to our chain state
	err := e.state.Mutate().Extend(candidate)
	if err != nil {
		return errors.Wrap(err, "could not extend state")
	}

	// finalize the state
	err = e.state.Mutate().Finalize(candidate.Hash())
	if err != nil {
		return errors.Wrap(err, "could not finalize state")
	}

	// remove all collections from the block from the mempool
	removed := uint(0)
	for _, gc := range candidate.GuaranteedCollections {
		ok := e.pool.Rem(gc.Hash())
		if ok {
			removed++
		}
	}

	log.Info().Uint("removed_collections", removed).Msg("block candidate committed")

	return nil
}
