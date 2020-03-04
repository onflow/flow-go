package follower

import (
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/model/flow"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
)

// HotStuffFollower implements interface HotStuffFollower
type HotStuffFollower struct {
	log           zerolog.Logger
	followerLogic *FollowerLogic
	proposals     chan *model.Proposal
	started       *atomic.Bool
}

// HotStuffFollower creates an instance of EventLoop
func New(
	me module.Local,
	protocolState protocol.State,
	dkgPubData *hotstuff.DKGPublicData,
	trustedRootBlock *flow.Header,
	rootBlockSigs *model.AggregatedSignature,
	finalizationCallback module.Finalizer,
	notifier finalizer.FinalizationConsumer,
	log zerolog.Logger,
) (*HotStuffFollower, error) {
	trustedRoot := unsafeToBlockQC(trustedRootBlock, rootBlockSigs)
	followerLogic, err := NewFollowerLogic(me, protocolState, dkgPubData, trustedRoot, finalizationCallback, notifier, log)
	if err != nil {
		return nil, fmt.Errorf("initialization of consensus follower failed: %w", err)
	}
	proposals := make(chan *model.Proposal)

	return &HotStuffFollower{
		log:           log,
		followerLogic: followerLogic,
		proposals:     proposals,
		started:       atomic.NewBool(false),
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

func (el *HotStuffFollower) loop() error {
	for {
		err := el.processEvent()
		// hotstuff will run in an event loop to process all events synchronously. And this is what will happen when hitting errors:
		// if hotstuff hits a known critical error, it will exit the loop (for instance, there is a conflicting block with a QC against finalized blocks
		// if hotstuff hits a known error indicates some assumption between components is broken, it will exit the loop (for instance, hotstuff receives a block whose parent is missing)
		// if hotstuff hits a known error that is safe to be ignored, it will not exit the loop (for instance, double voting/invalid vote)
		// if hotstuff hits any unknown error, it will exit the loop
		if err != nil {
			return err
		}
	}
}

// processEvent processes one event at a time.
// This function should only be called within the `loop` function
func (el *HotStuffFollower) processEvent() error {
	// Giving timeout events the priority to be processed first
	// This is to prevent attacks from malicious nodes that attempt
	// to block honest nodes' pacemaker from progressing by sending
	// other events.
	timeoutChannel := el.eventHandler.TimeoutChannel()

	idleStart := time.Now()

	var err error
	select {
	case t := <-timeoutChannel:
		// measure how long it takes for a timeout event to go through
		// eventloop and get handled
		busyDuration := time.Now().Sub(t)
		el.log.Debug().Dur("busy_duration", busyDuration).
			Msg("busy duration to handle local timeout")

		// meansure how long the event loop was idle waiting for an
		// incoming event
		idleDuration := time.Now().Sub(idleStart)
		el.log.Debug().Dur("idle_duration", idleDuration)

		err = el.eventHandler.OnLocalTimeout()
	default:
	}

	if err != nil {
		return err
	}

	// select for block headers/votes here
	select {
	case t := <-timeoutChannel:
		busyDuration := time.Now().Sub(t)
		el.log.Debug().Dur("busy_duration", busyDuration).
			Msg("busy duration to handle local timeout")

		idleDuration := time.Now().Sub(idleStart)
		el.log.Debug().Dur("idle_duration", idleDuration)

		err = el.eventHandler.OnLocalTimeout()
	case p := <-el.proposals:
		idleDuration := time.Now().Sub(idleStart)
		el.log.Debug().Dur("idle_duration", idleDuration)

		err = el.eventHandler.OnReceiveProposal(p)
	case v := <-el.votes:
		idleDuration := time.Now().Sub(idleStart)
		el.log.Debug().Dur("idle_duration", idleDuration)

		err = el.eventHandler.OnReceiveVote(v)
	}
	return err
}

// OnReceiveProposal pushes the received block to the blockheader channel
func (el *HotStuffFollower) OnReceiveProposal(proposal *hotstuff.Proposal) {
	received := time.Now()

	el.proposals <- proposal

	// the busy duration is measured as how long it takes from a block being
	// received to a block being handled by the event handler.
	busyDuration := time.Now().Sub(received)
	el.log.Debug().Hex("block_ID", logging.ID(proposal.Block.BlockID)).
		Uint64("view", proposal.Block.View).
		Dur("busy_duration", busyDuration).
		Msg("busy duration to handle a proposal")
}

// Start will start the event handler then enter the loop
func (el *HotStuffFollower) Start() error {
	if el.started.Swap(true) {
		return nil
	}
	err := el.eventHandler.Start()
	if err != nil {
		return fmt.Errorf("can not start the eventloop: %w", err)
	}
	return el.loop()
}
