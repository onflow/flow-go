package follower

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/model/flow"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// HotStuffFollower implements interface HotStuffFollower
type HotStuffFollower struct {
	log           zerolog.Logger
	followerLogic *FollowerLogic
	proposals     chan *model.Proposal
	started       *atomic.Bool
	readyChan     chan struct{}
	doneChan      chan struct{}
}

// HotStuffFollower creates an instance of EventLoop
func New(
	me module.Local,
	protocolState protocol.State,
	dkgPubData *hotstuff.DKGPublicData,
	trustedRootBlock *flow.Header,
	rootBlockSigs *model.AggregatedSignature,
	finalizationCallback module.Finalizer,
	notifier notifications.FinalizationConsumer,
	log zerolog.Logger,
) (*HotStuffFollower, error) {
	trustedRoot := unsafeToBlockQC(trustedRootBlock, rootBlockSigs)
	followerLogic, err := NewFollowerLogic(me, protocolState, dkgPubData, trustedRoot, finalizationCallback, notifier, log)
	if err != nil {
		return nil, fmt.Errorf("initialization of consensus follower failed: %w", err)
	}
	return &HotStuffFollower{
		log:           log,
		followerLogic: followerLogic,
		proposals:     make(chan *model.Proposal),
		started:       atomic.NewBool(false),
		readyChan:     make(chan struct{}),
		doneChan:      make(chan struct{}),
	}, nil
}

// unsafeToBlockQC converts trustedRootBlock and the respective signatures into a BlockQC.
// The result is constructed blindly without verification as the FollowerLogic validates its inputs.
// The returned block does not contain a qc to its parent as, per precondition of the initialization,
// we do not consider ancestors of the trusted root block. (if trustedRootBlock is the genesis block, it
// will not even have a QC as there is no parent).
func unsafeToBlockQC(trustedRootBlock *flow.Header, rootBlockSigs *model.AggregatedSignature) *forks.BlockQC {
	rootBlockID := trustedRootBlock.ID()
	block := &model.Block{
		BlockID:     rootBlockID,
		View:        trustedRootBlock.View,
		ProposerID:  trustedRootBlock.ProposerID,
		QC:          nil,
		PayloadHash: trustedRootBlock.PayloadHash,
		Timestamp:   trustedRootBlock.Timestamp,
	}
	qc := &model.QuorumCertificate{
		BlockID:             rootBlockID,
		View:                trustedRootBlock.View,
		AggregatedSignature: rootBlockSigs,
	}
	return &forks.BlockQC{
		Block: block,
		QC:    qc,
	}
}

// SubmitProposal feeds a new block proposal (header) into the HotStuffFollower.
// This method blocks until the proposal is accepted to the event queue.
//
// Block proposals must be submitted in order, i.e. a proposal's parent must
// have been previously processed by the HotStuffFollower.
func (el *HotStuffFollower) SubmitProposal(proposalHeader *flow.Header, parentView uint64) {
	received := time.Now()
	proposal := model.ProposalFromFlow(proposalHeader, parentView)
	el.proposals <- proposal
	// the busy duration is measured as how long it takes from a block being
	// received to a block being handled by the event handler.
	busyDuration := time.Now().Sub(received)
	el.log.Debug().Hex("block_ID", logging.ID(proposal.Block.BlockID)).
		Uint64("view", proposal.Block.View).
		Dur("busy_duration", busyDuration).
		Msg("busy duration to handle a proposal")
}

// Start will starts the HotStuffFollower's internal processing loop.
// Method blocks until the loop exists with a fatal error.
func (el *HotStuffFollower) Start() error {
	if el.started.Swap(true) {
		return nil
	}
	close(el.readyChan)
	err := el.loop()
	close(el.doneChan)
	return fmt.Errorf("follower's event loop crashed: %w", err)
}

func (el *HotStuffFollower) loop() error {
	for {
		idleStart := time.Now()
		p := <-el.proposals
		idleDuration := time.Now().Sub(idleStart)
		el.log.Debug().Dur("idle_duration", idleDuration)

		err := el.followerLogic.AddBlock(p)
		// HotStuffFollower will run in an event loop to process all events synchronously.
		// And this point, we are only expecting fatal errors:
		//   * known critical error: exit the loop (some assumption between components is broken)
		//   * unknown critical error: it will exit the loop
		if err != nil {
			el.log.Error().Hex("block_ID", logging.ID(p.Block.BlockID)).
				Uint64("view", p.Block.View).
				Msg("fatal error processing proposal")
			return err
		}
	}
}

// Ready implements interface module.ReadyDoneAware
func (el *HotStuffFollower) Ready() <-chan struct{} {
	return el.readyChan
}

// Done implements interface module.ReadyDoneAware
func (el *HotStuffFollower) Done() <-chan struct{} {
	return el.doneChan
}
