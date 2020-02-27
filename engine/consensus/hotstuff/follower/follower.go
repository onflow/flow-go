package follower

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	models "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
	"github.com/rs/zerolog"
)

// Follower runs in non-consensus nodes. It informs other components within the node
// about finalization of blocks. The consensus Follower consumes all block proposals
// broadcasts by the consensus node, verifies the block header and locally evaluates
// the finalization rules.
//
// CAUTION: Follower is NOT CONCURRENCY safe
type Follower struct {
	log zerolog.Logger
	validator            *hotstuff.Validator
	finalizationLogic    forks.Finalizer
	finalizationCallback module.Finalizer
	notifier             finalizer.FinalizationConsumer
}

func New(
	me module.Local,
	protocolState protocol.State,
	dkgPubData *signature.DKGPublicData,
	trustedRoot *forks.BlockQC,
	finalizationCallback module.Finalizer,
	notifier finalizer.FinalizationConsumer,
	log zerolog.Logger,
) (*Follower, error) {
	finalizationLogic, err := finalizer.New(trustedRoot, finalizationCallback, notifier)
	if err != nil {
		return nil, fmt.Errorf("initialization of consensus follower failed: %w", err)
	}
	viewState, err := hotstuff.NewViewState(protocolState, me.NodeID(), filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("initialization of consensus follower failed: %w", err)
	}
	sigVerifier := initConsensusSigVerifier(dkgPubData)
	return &Follower{
		validator:            hotstuff.NewValidator(viewState, finalizationLogic, sigVerifier),
		finalizationLogic:    finalizationLogic,
		finalizationCallback: finalizationCallback,
		notifier:             notifier,
		log:            log.With().Str("hotstuff", "follower").Logger(),
	}, nil
}

func initConsensusSigVerifier(dkgPubData *signature.DKGPublicData) hotstuff.SigVerifier {
	sigVerifier := struct {
		signature.StakingSigVerifier
		signature.RandomBeaconSigVerifier
	}{
		signature.NewStakingSigVerifier(messages.ConsensusVoteTag),
		signature.NewRandomBeaconSigVerifier(dkgPubData),
	}
	return &sigVerifier
}

func (f *Follower) FinalizedBlock() *models.Block {
	return f.finalizationLogic.FinalizedBlock()
}

func (f *Follower) FinalizedView() uint64 {
	return f.finalizationLogic.FinalizedView()
}

func (f *Follower) AddBlock(blockProposal *models.Proposal) error {
	// validate the block. exit if the proposal is invalid
	err := f.validator.ValidateProposal(blockProposal)
	if errors.Is(err, models.ErrorInvalidBlock{}) {
		f.log.Warn().Hex("block_id", logging.ID(blockProposal.Block.BlockID)).
					 Msg("invalid proposal")
		return nil
	}

	if errors.Is(err, models.ErrUnverifiableBlock) {
		f.log.Warn().
			Hex("block_id", logging.ID(blockProposal.Block.BlockID)).
			Hex("qc_block_id", logging.ID(blockProposal.Block.QC.BlockID)).
			Msg("unverifiable proposal")

		// even if the block is unverifiable because the QC has been
		// pruned, it still needs to be added to the forks, otherwise,
		// a new block with a QC to this block will fail to be added
		// to forks and crash the event loop.
	} else {
		return fmt.Errorf("cannot validate block proposal (%x): %w", blockProposal.Block.BlockID, err)
	}

	// as a sanity check, we run the finalization logic's internal validation on the block
	if err := f.finalizationLogic.VerifyBlock(blockProposal.Block); err != nil {
		// this should never happen: the block was found to be valid by the validator
		// if the finalization logic's internal validation errors, we have a bug
		return fmt.Errorf("invaid block passed validation: %w", err)
	}
	err = f.finalizationLogic.AddBlock(blockProposal.Block)
	if err != nil {
		return fmt.Errorf("hotstuff follower cannot process block (%x): %w", blockProposal.Block.BlockID, err)
	}

	return nil
}
