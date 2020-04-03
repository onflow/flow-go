package follower

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// HotStuffFollower implements interface HotStuffFollower
type HotStuffFollower struct {
	log           zerolog.Logger
	followerLogic *FollowerLogic
	proposals     chan *model.Proposal

	runner SingleRunner // lock for preventing concurrent state transitions
}

// New creates an instance of EventLoop
func New(
	me module.Local,
	protocolState protocol.State,
	trustedRootBlock *flow.Header,
	rootBlockQC *model.QuorumCertificate,
	verifier hotstuff.Verifier,
	finalizationCallback module.Finalizer,
	notifier notifications.FinalizationConsumer,
	log zerolog.Logger,
) (*HotStuffFollower, error) {
	trustedRoot := unsafeToBlockQC(trustedRootBlock, rootBlockQC)
	followerLogic, err := NewFollowerLogic(me, protocolState, trustedRoot, verifier, finalizationCallback, notifier, log)
	if err != nil {
		return nil, fmt.Errorf("initialization of consensus follower failed: %w", err)
	}
	return &HotStuffFollower{
		log:           log,
		followerLogic: followerLogic,
		proposals:     make(chan *model.Proposal),
		runner:        NewSingleRunner(),
	}, nil
}

// unsafeToBlockQC converts trustedRootBlock and the respective signatures into a BlockQC.
// The result is constructed blindly without verification as the FollowerLogic validates its inputs.
// The returned block does not contain a qc to its parent as, per precondition of the initialization,
// we do not consider ancestors of the trusted root block. (if trustedRootBlock is the genesis block, it
// will not even have a QC as there is no parent).
func unsafeToBlockQC(trustedRootBlock *flow.Header, rootBlockQC *model.QuorumCertificate) *forks.BlockQC {
	rootBlockID := trustedRootBlock.ID()
	block := &model.Block{
		View:        trustedRootBlock.View,
		BlockID:     rootBlockID,
		ProposerID:  trustedRootBlock.ProposerID,
		QC:          nil,
		PayloadHash: trustedRootBlock.PayloadHash,
		Timestamp:   trustedRootBlock.Timestamp,
	}
	return &forks.BlockQC{
		Block: block,
		QC:    rootBlockQC,
	}
}

// SubmitProposal feeds a new block proposal (header) into the HotStuffFollower.
// This method blocks until the proposal is accepted to the event queue.
//
// Block proposals must be submitted in order, i.e. a proposal's parent must
// have been previously processed by the HotStuffFollower.
func (fl *HotStuffFollower) SubmitProposal(proposalHeader *flow.Header, parentView uint64) {
	received := time.Now()
	proposal := model.ProposalFromFlow(proposalHeader, parentView)
	fl.proposals <- proposal
	// the busy duration is measured as how long it takes from a block being
	// received to a block being handled by the event handler.
	busyDuration := time.Since(received)
	fl.log.Debug().Hex("block_ID", logging.ID(proposal.Block.BlockID)).
		Uint64("view", proposal.Block.View).
		Dur("busy_duration", busyDuration).
		Msg("busy duration to handle a proposal")
}

// loop will synchronously processes all events.
// All errors from FollowerLogic are fatal:
//   * known critical error: some prerequisites of the HotStuff follower have been broken
//   * unknown critical error: bug-related
func (fl *HotStuffFollower) loop() {
	shutdownSignal := fl.runner.ShutdownSignal()
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
				fl.log.Error().Hex("block_ID", logging.ID(p.Block.BlockID)).
					Uint64("view", p.Block.View).
					Msg("fatal error processing proposal")
				fl.log.Error().Msgf("terminating HotStuffFollower: %s", err.Error())
				return
			}
		case <-shutdownSignal:
			return
		}
	}
}

// Ready implements interface module.ReadyDoneAware
// Method call will starts the HotStuffFollower's internal processing loop.
// Multiple calls are handled gracefully and the follower will only start once.
func (fl *HotStuffFollower) Ready() <-chan struct{} {
	return fl.runner.Start(fl.loop)
}

// Done implements interface module.ReadyDoneAware
func (fl *HotStuffFollower) Done() <-chan struct{} {
	return fl.runner.Abort()
}
