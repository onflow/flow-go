package hotstuff

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/utils/logging"
)

// FollowerLoop implements interface FollowerLoop
type FollowerLoop struct {
	log           zerolog.Logger
	followerLogic FollowerLogic
	proposals     chan *model.Proposal

	lm *lifecycle.LifecycleManager // lock for preventing concurrent state transitions
}

// NewFollowerLoop creates an instance of EventLoop
func NewFollowerLoop(log zerolog.Logger, followerLogic FollowerLogic) (*FollowerLoop, error) {
	return &FollowerLoop{
		log:           log,
		followerLogic: followerLogic,
		proposals:     make(chan *model.Proposal),
		lm:            lifecycle.NewLifecycleManager(),
	}, nil
}

// SubmitProposal feeds a new block proposal (header) into the FollowerLoop.
// This method blocks until the proposal is accepted to the event queue.
//
// Block proposals must be submitted in order, i.e. a proposal's parent must
// have been previously processed by the FollowerLoop.
func (fl *FollowerLoop) SubmitProposal(proposalHeader *flow.Header, parentView uint64) {
	received := time.Now()
	proposal := model.ProposalFromFlow(proposalHeader, parentView)
	fl.proposals <- proposal
	// the busy duration is measured as how long it takes from a block being
	// received to a block being handled by the event handler.
	busyDuration := time.Since(received)
	fl.log.Debug().Hex("block_id", logging.ID(proposal.Block.BlockID)).
		Uint64("view", proposal.Block.View).
		Dur("busy_duration", busyDuration).
		Msg("busy duration to handle a proposal")
}

// loop will synchronously processes all events.
// All errors from FollowerLogic are fatal:
//   * known critical error: some prerequisites of the HotStuff follower have been broken
//   * unknown critical error: bug-related
func (fl *FollowerLoop) loop() {
	shutdownSignal := fl.lm.ShutdownSignal()
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
	fl.lm.OnStart(func() {
		go func() {
			fl.loop()
			// there are two cases f() would exit:
			// (a) f exited on its own without Done() being called (this is generally an internal error)
			// (b) f exited as a reaction to Done() being called
			// In either case, we want to abort and stop the lifecycle manager
			fl.lm.OnStop()
		}()
	})
	return fl.lm.Started()
}

// Done implements interface module.ReadyDoneAware
func (fl *FollowerLoop) Done() <-chan struct{} {
	fl.lm.OnStop()
	return fl.lm.Stopped()
}
