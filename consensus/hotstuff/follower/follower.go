package follower

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/dkg"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// FollowerLogic runs in non-consensus nodes. It informs other components within the node
// about finalization of blocks. The consensus Follower consumes all block proposals
// broadcasts by the consensus node, verifies the block header and locally evaluates
// the finalization rules.
//
// CAUTION: Follower is NOT CONCURRENCY safe
type FollowerLogic struct {
	validator            hotstuff.Validator
	finalizationLogic    forks.Finalizer
	finalizationCallback module.Finalizer
	notifier             notifications.FinalizationConsumer
	log                  zerolog.Logger
}

func NewFollowerLogic(
	me module.Local,
	protocolState protocol.State,
	dkgState dkg.State,
	trustedRoot *forks.BlockQC,
	finalizationCallback module.Finalizer,
	notifier notifications.FinalizationConsumer,
	log zerolog.Logger,
) (*FollowerLogic, error) {
	viewState, err := viewstate.New(protocolState, dkgState, me.NodeID(), filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("initialization of consensus follower failed: %w", err)
	}
	finalizationLogic, err := finalizer.New(trustedRoot, finalizationCallback, notifier)
	if err != nil {
		return nil, fmt.Errorf("initialization of consensus follower failed: %w", err)
	}
	// TODO: initialize signer
	var signer hotstuff.Signer
	validator := validator.New(viewState, finalizationLogic, signer)

	err = validator.ValidateQC(trustedRoot.QC, trustedRoot.Block)
	if err != nil {
		return nil, fmt.Errorf("invalid root qc: %w", err)
	}

	return &FollowerLogic{
		validator:            validator,
		finalizationLogic:    finalizationLogic,
		finalizationCallback: finalizationCallback,
		notifier:             notifier,
		log:                  log.With().Str("hotstuff", "follower").Logger(),
	}, nil
}

func (f *FollowerLogic) FinalizedBlock() *model.Block {
	return f.finalizationLogic.FinalizedBlock()
}

func (f *FollowerLogic) AddBlock(blockProposal *model.Proposal) error {
	// validate the block. skip if the proposal is invalid
	err := f.validator.ValidateProposal(blockProposal)
	if errors.Is(err, model.ErrorInvalidBlock{}) {
		f.log.Warn().Hex("block_id", logging.ID(blockProposal.Block.BlockID)).
			Msg("invalid proposal")
		return nil
	}
	if errors.Is(err, model.ErrUnverifiableBlock) {
		f.log.Warn().
			Hex("block_id", logging.ID(blockProposal.Block.BlockID)).
			Hex("qc_block_id", logging.ID(blockProposal.Block.QC.BlockID)).
			Msg("unverifiable proposal")

		// even if the block is unverifiable because the QC has been
		// pruned, it still needs to be added to the forks, otherwise,
		// a new block with a QC to this block will fail to be added
		// to forks and crash the event loop.
	} else if err != nil {
		return fmt.Errorf("cannot validate block proposal %x: %w", blockProposal.Block.BlockID, err)
	}

	// as a sanity check, we run the finalization logic's internal validation on the block
	if err := f.finalizationLogic.VerifyBlock(blockProposal.Block); err != nil {
		// this should never happen: the block was found to be valid by the validator
		// if the finalization logic's internal validation errors, we have a bug
		return fmt.Errorf("invaid block passed validation: %w", err)
	}
	err = f.finalizationLogic.AddBlock(blockProposal.Block)
	if err != nil {
		return fmt.Errorf("finalization logic cannot process block proposal %x: %w", blockProposal.Block.BlockID, err)
	}

	return nil
}
