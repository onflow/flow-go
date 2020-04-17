package coldstuff

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/coldstuff/round"
	"github.com/dapperlabs/flow-go/engine"
	model "github.com/dapperlabs/flow-go/model/coldstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/order"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// ColdStuff implements coldstuff.
type ColdStuff struct {
	log       zerolog.Logger
	state     protocol.State
	me        module.Local
	round     *round.Round
	comms     Communicator
	builder   module.Builder
	finalizer module.Finalizer
	unit      *engine.Unit

	// round config
	participants flow.IdentityList            // the participants in consensus
	interval     time.Duration                // how often a block is proposed
	timeout      time.Duration                // how long to wait for votes
	head         func() (*flow.Header, error) // returns the head of chain state

	// incoming consensus entities
	proposals chan *flow.Header
	votes     chan *model.Vote
	commits   chan *model.Commit
}

func New(
	log zerolog.Logger,
	state protocol.State,
	me module.Local,
	comms Communicator,
	builder module.Builder,
	finalizer module.Finalizer,
	memberFilter flow.IdentityFilter,
	interval time.Duration,
	timeout time.Duration,
	head func() (*flow.Header, error),
) (*ColdStuff, error) {

	participants, err := state.Final().Identities(memberFilter)
	if err != nil {
		return nil, fmt.Errorf("could not get consensus participants: %w", err)
	}
	participants = participants.Order(order.ByNodeIDAsc)

	cold := &ColdStuff{
		log:          log,
		me:           me,
		state:        state,
		comms:        comms,
		builder:      builder,
		finalizer:    finalizer,
		unit:         engine.NewUnit(),
		participants: participants,
		interval:     interval,
		timeout:      timeout,
		head:         head,
		proposals:    make(chan *flow.Header, 1),
		votes:        make(chan *model.Vote, 1),
		commits:      make(chan *model.Commit, 1),
	}

	return cold, nil
}

func (e *ColdStuff) Ready() <-chan struct{} {
	e.unit.Launch(func() {
		err := e.loop()
		if err != nil {
			e.log.Error().Err(err).Msg("coldstuff loop exited with error")
		}
	})
	return e.unit.Ready()
}

func (e *ColdStuff) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *ColdStuff) SubmitProposal(proposal *flow.Header, parentView uint64) {
	// Ignore HotStuff-only values
	_ = parentView

	e.proposals <- proposal
}

func (e *ColdStuff) SubmitVote(originID, blockID flow.Identifier, view uint64, sigData []byte) {
	// Ignore HotStuff-only values
	_ = view
	_ = sigData

	e.votes <- &model.Vote{
		VoterID: originID,
		BlockID: blockID,
	}
}

func (e *ColdStuff) SubmitCommit(commit *model.Commit) {
	e.commits <- commit
}

func (e *ColdStuff) loop() error {

	localID := e.me.NodeID()
	log := e.log.With().Hex("local_id", logging.ID(localID)).Logger()

ConsentLoop:
	for {

		head, err := e.head()
		if err != nil {
			return fmt.Errorf("could not get head: %w", err)
		}

		e.round, err = round.New(head, e.me, e.participants)
		if err != nil {
			return fmt.Errorf("could not initialize round: %w", err)
		}

		// calculate the time at which we can generate the next valid block
		limit := e.round.Parent().Timestamp.Add(e.interval)

		log.Debug().
			Str("leader", e.round.Leader().NodeID.String()).
			Str("participants", fmt.Sprintf("%v", e.participants.NodeIDs())).
			Hex("head_id", logging.ID(head.ID())).
			Uint64("head_height", head.Height).
			Msg("starting round")

		select {
		case <-e.unit.Quit():
			return nil
		case <-time.After(time.Until(limit)):
			if e.round.Leader().NodeID == localID {
				// if we are the leader, we:
				// 1) send a block proposal
				// 2) wait for sufficient block votes
				// 3) send a block commit

				e.log.Debug().Msg("sending proposal")
				err = e.sendProposal()
				if err != nil {
					log.Warn().Err(err).Msg("could not send proposal")
					continue ConsentLoop
				}

				e.log.Debug().Msg("waiting for votes")
				err = e.waitForVotes()
				if err != nil {
					log.Warn().Err(err).Msg("could not receive votes")
					continue ConsentLoop
				}

				e.log.Debug().Msg("sending commit")
				err = e.sendCommit()
				if err != nil {
					log.Warn().Err(err).Msg("could not send commit")
					continue ConsentLoop
				}

			} else {
				// if we are not the leader, we:
				// 1) wait for a block proposal
				// 2) vote on the block proposal
				// 3) wait for a block commit

				e.log.Debug().Msg("waiting for proposal")
				err = e.waitForProposal()
				if err != nil {
					log.Warn().Err(err).Msg("could not receive proposal")
					continue ConsentLoop
				}

				e.log.Debug().Msg("voting for proposal")
				err = e.voteOnProposal()
				if err != nil {
					log.Warn().Err(err).Msg("could not vote on proposal")
					continue ConsentLoop
				}

				e.log.Debug().Msg("waiting for commit")
				err = e.waitForCommit()
				if err != nil {
					log.Warn().Err(err).Msg("could not receive commit")
					continue ConsentLoop
				}
			}

			// regardless of path, if we successfully reach here, we finished a
			// full successful consensus round and can commit the current
			// block candidate
			e.log.Debug().Msg("committing candidate")
			err = e.commitCandidate()
			if err != nil {
				log.Error().Err(err).Msg("could not commit candidate")
				continue
			}
		}
	}
}

func (e *ColdStuff) sendProposal() error {
	log := e.log.With().
		Str("action", "send_proposal").
		Logger()

	// get our own ID to tally our stake
	myIdentity, err := e.state.Final().Identity(e.me.NodeID())
	if err != nil {
		return fmt.Errorf("could not get own current ID: %w", err)
	}

	// define the block header build function
	setProposer := func(header *flow.Header) {
		header.ProposerID = myIdentity.NodeID
		header.View = e.round.Parent().View + 1
		header.ParentVoterIDs = e.participants.NodeIDs()
	}

	// define payload and build next block
	candidate, err := e.builder.BuildOn(e.round.Parent().ID(), setProposer)
	if err != nil {
		return fmt.Errorf("could not build on parent: %w", err)
	}

	log = log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Logger()

	// cache the candidate block
	e.round.Propose(candidate)

	// send the block proposal
	err = e.comms.BroadcastProposal(candidate)
	if err != nil {
		return fmt.Errorf("could not submit proposal: %w", err)
	}

	// add our own vote to the engine
	e.round.Tally(myIdentity.NodeID, myIdentity.Stake)

	log.Info().Msg("block proposal sent")

	return nil

}

// waitForVotes will wait for received votes and validate them until we have
// reached a quorum on the currently cached block candidate. It assumes we are
// the leader and will timeout after the configured timeout.
func (e *ColdStuff) waitForVotes() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "wait_votes").
		Logger()

	if id, err := e.state.Final().Identity(e.me.NodeID()); err == nil && e.round.Quorum() == id.Stake {
		log.Info().Msg("sufficient votes received")
		return nil
	}

	for {
		select {

		// process each vote that we receive
		case vote := <-e.votes:
			voterID, voteID := vote.VoterID, vote.BlockID

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

			// discard votes that are not by staked consensus participants
			voter, err := e.state.Final().Identity(voterID)
			if errors.Is(err, badger.ErrKeyNotFound) {
				log.Warn().Hex("voter_id", voterID[:]).Msg("vote by unknown node")
				continue
			}
			if err != nil {
				log.Error().Err(err).Hex("voter_id", voterID[:]).Msg("could not verify voter ID")
				break
			}
			_, isParticipant := e.participants.ByNodeID(voterID)
			if !isParticipant {
				log.Warn().Hex("voter_id", logging.ID(voterID)).Msg("vote by non-participant")
				continue
			}

			// discard votes that are on the wrong candidate
			if voteID != candidate.ID() {
				log.Warn().Hex("vote_id", voteID[:]).Msg("invalid candidate vote")
				continue
			}

			// tally the voting stake of the voter ID
			e.round.Tally(voterID, voter.Stake)
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
func (e *ColdStuff) sendCommit() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "send_commit").
		Logger()

	// send a commit for the cached block hash
	commit := &model.Commit{
		BlockID:     candidate.ID(),
		CommitterID: e.me.NodeID(),
	}
	err := e.comms.BroadcastCommit(commit)
	if err != nil {
		return fmt.Errorf("could not submit commit: %w", err)
	}

	log.Info().Msg("block commit sent")

	return nil
}

// waitForProposal waits for a block proposal to be received and validates it in
// a number of ways. It should be called at the beginning of a round if we are
// not the leader. It will timeout if no proposal was received by the leader
// after the configured timeout.
func (e *ColdStuff) waitForProposal() error {
	log := e.log.With().
		Str("action", "wait_proposal").
		Logger()

	for {
		select {

		// process each proposal we receive
		case candidate := <-e.proposals:
			proposerID := candidate.ProposerID

			// discard proposals by non-leaders
			leaderID := e.round.Leader().NodeID
			if proposerID != leaderID {
				log.Warn().Hex("candidate_leader", proposerID[:]).Hex("expected_leader", leaderID[:]).Msg("invalid leader")
				continue
			}

			// discard proposals with the wrong height
			number := e.round.Parent().Height + 1
			if candidate.Height != e.round.Parent().Height+1 {
				log.Warn().Uint64("candidate_height", candidate.Height).Uint64("expected_height", number).Msg("invalid height")
				continue
			}

			// discard proposals with the wrong parent
			parentID := e.round.Parent().ID()
			if candidate.ParentID != parentID {
				log.Warn().Hex("candidate_parent", candidate.ParentID[:]).Hex("expected_parent", parentID[:]).Msg("invalid parent")
				continue
			}

			// cache the candidate for the round
			e.round.Propose(candidate)

			log.Info().
				Uint64("number", candidate.Height).
				Hex("candidate_id", logging.Entity(candidate)).
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
func (e *ColdStuff) voteOnProposal() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "send_vote").
		Logger()

	// create a fake signature to avoid network message de-duplication
	sig := make([]byte, 32)
	_, err := rand.Read(sig)
	if err != nil {
		return fmt.Errorf("could not create fake signature: %w", err)
	}

	// send vote for proposal to leader
	err = e.comms.SendVote(candidate.ID(), candidate.View, sig, e.round.Leader().NodeID)
	if err != nil {
		return fmt.Errorf("could not submit vote: %w", err)
	}

	log.Info().Msg("block vote sent")

	return nil
}

// waitForCommit is called after we have submitted our vote for the leader and
// awaits his confirmation that we can commit the block. The confirmation is
// only sent once a quorum of votes was received by the leader.
func (e *ColdStuff) waitForCommit() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "wait_commit").
		Logger()

	for {
		select {
		case w := <-e.commits:
			committerID, commitID := w.CommitterID, w.BlockID

			// discard commits not from leader
			leaderID := e.round.Leader().NodeID
			if committerID != leaderID {
				log.Warn().Hex("commit_leader", committerID[:]).Hex("expected_leader", leaderID[:]).Msg("invalid commit leader")
				continue
			}

			// discard commits not for candidate hash
			if commitID != candidate.ID() {
				log.Warn().Hex("commit_id", commitID[:]).Msg("invalid commit hash")
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
func (e *ColdStuff) commitCandidate() error {

	candidate := e.round.Candidate()

	log := e.log.With().
		Uint64("number", candidate.Height).
		Hex("candidate_id", logging.Entity(candidate)).
		Str("action", "exec_commit").
		Logger()

	err := e.finalizer.MakeFinal(candidate.ID())
	if err != nil {
		return fmt.Errorf("could not finalize committed block: %w", err)
	}

	log.Info().Msg("block candidate committed")

	return nil
}
