package hotstuff

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/logging"
)

// FollowerLoop implements interface module.HotStuffFollower.
// FollowerLoop buffers all incoming events to the hotstuff FollowerLogic, and feeds FollowerLogic one event at a time
// using a worker thread.
// Concurrency safe.
type FollowerLoop struct {
	*component.ComponentManager
	log           zerolog.Logger
	followerLogic FollowerLogic
	proposals     chan *model.Proposal
}

var _ component.Component = (*FollowerLoop)(nil)
var _ module.HotStuffFollower = (*FollowerLoop)(nil)

// NewFollowerLoop creates an instance of EventLoop
func NewFollowerLoop(log zerolog.Logger, followerLogic FollowerLogic) (*FollowerLoop, error) {
	// TODO(active-pacemaker) add metrics for length of inbound channels
	// we will use a buffered channel to avoid blocking of caller
	proposals := make(chan *model.Proposal, 1000)

	fl := &FollowerLoop{
		log:           log,
		followerLogic: followerLogic,
		proposals:     proposals,
	}

	fl.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(fl.loop).
		Build()

	return fl, nil
}

// SubmitProposal feeds a new block proposal (header) into the FollowerLoop.
// This method blocks until the proposal is accepted to the event queue.
//
// Block proposals must be submitted in order, i.e. a proposal's parent must
// have been previously processed by the FollowerLoop.
func (fl *FollowerLoop) SubmitProposal(proposal *model.Proposal) {
	received := time.Now()

	select {
	case fl.proposals <- proposal:
	case <-fl.ComponentManager.ShutdownSignal():
		return
	}

	// the busy duration is measured as how long it takes from a block being
	// received to a block being handled by the event handler.
	busyDuration := time.Since(received)
	fl.log.Debug().Hex("block_id", logging.ID(proposal.Block.BlockID)).
		Uint64("view", proposal.Block.View).
		Dur("busy_duration", busyDuration).
		Msg("busy duration to handle a proposal")
}

// loop will synchronously process all events.
// All errors from FollowerLogic are fatal:
//   - known critical error: some prerequisites of the HotStuff follower have been broken
//   - unknown critical error: bug-related
func (fl *FollowerLoop) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	shutdownSignal := fl.ComponentManager.ShutdownSignal()
	for {
		select { // to ensure we are not skipping over a termination signal
		case <-shutdownSignal:
			return
		default:
		}

		select {
		case p := <-fl.proposals:
			err := fl.followerLogic.AddBlock(p)
			if err != nil { // all errors are fatal
				fl.log.Error().
					Hex("block_id", logging.ID(p.Block.BlockID)).
					Uint64("view", p.Block.View).
					Err(err).
					Msg("irrecoverable follower loop error")
				ctx.Throw(err)
			}
		case <-shutdownSignal:
			return
		}
	}
}
