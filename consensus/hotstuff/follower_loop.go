package hotstuff

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/runner"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/logging"
)

// proposalTask struct used to send a proposal and done channel in one message
type proposalTask struct {
	*model.Proposal
	done chan struct{} // closed when the proposal has finished being processed
}

// FollowerLoop implements interface FollowerLoop
type FollowerLoop struct {
	log           zerolog.Logger
	followerLogic FollowerLogic
	proposals     chan *proposalTask

	runner runner.SingleRunner // lock for preventing concurrent state transitions
}

// NewFollowerLoop creates an instance of EventLoop
func NewFollowerLoop(log zerolog.Logger, followerLogic FollowerLogic) (*FollowerLoop, error) {
	return &FollowerLoop{
		log:           log,
		followerLogic: followerLogic,
		proposals:     make(chan *proposalTask),
		runner:        runner.NewSingleRunner(),
	}, nil
}

// SubmitProposal feeds a new block proposal (header) into the FollowerLoop.
// This method blocks until the proposal is accepted to the event queue.
//
// Block proposals must be submitted in order, i.e. a proposal's parent must
// have been previously processed by the FollowerLoop.
func (fl *FollowerLoop) SubmitProposal(proposalHeader *flow.Header, parentView uint64) <-chan struct{} {
	received := time.Now()
	proposal := &proposalTask{
		Proposal: model.ProposalFromFlow(proposalHeader, parentView),
		done:     make(chan struct{}),
	}

	fl.proposals <- proposal

	// the busy duration is measured as how long it takes from a block being
	// received to a block being handled by the event handler.
	busyDuration := time.Since(received)
	fl.log.Debug().Hex("block_id", logging.ID(proposal.Block.BlockID)).
		Uint64("view", proposal.Block.View).
		Dur("busy_duration", busyDuration).
		Msg("busy duration to handle a proposal")

	return proposal.done
}

// loop will synchronously processes all events.
// All errors from FollowerLogic are fatal:
//   * known critical error: some prerequisites of the HotStuff follower have been broken
//   * unknown critical error: bug-related
func (fl *FollowerLoop) loop() {
	shutdownSignal := fl.runner.ShutdownSignal()
	for {
		select { // to ensure we are not skipping over a termination signal
		case <-shutdownSignal:
			return
		default:
		}

		select {
		case p := <-fl.proposals:
			err := fl.followerLogic.AddBlock(p.Proposal)
			close(p.done)

			if err != nil { // all errors are fatal
				fl.log.Error().
					Hex("block_id", logging.ID(p.Block.BlockID)).
					Uint64("view", p.Block.View).
					Err(err).
					Msg("terminating FollowerLoop")
				return
			}
		case <-shutdownSignal:
			return
		}
	}
}

// Ready implements interface module.ReadyDoneAware
// Method call will starts the FollowerLoop's internal processing loop.
// Multiple calls are handled gracefully and the follower will only start once.
func (fl *FollowerLoop) Ready() <-chan struct{} {
	return fl.runner.Start(fl.loop)
}

// Done implements interface module.ReadyDoneAware
func (fl *FollowerLoop) Done() <-chan struct{} {
	return fl.runner.Abort()
}
