package votecollector

import (
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

//func NewWeightedSignatureAggregator(
//	signers []flow.Identity, // list of all possible signers
//	message []byte, // message to get an aggregated signature for
//	dsTag string, // domain separation tag used by the signature

// CombinedVoteProcessorFactory implements a factory for creating CombinedVoteProcessor
// holds needed dependencies to initialize CombinedVoteProcessor.
type CombinedVoteProcessorFactory struct {
	log         zerolog.Logger
	packer      hotstuff.Packer
	committee   hotstuff.Committee
	onQCCreated hotstuff.OnQCCreated
}

func NewCombinedVoteProcessorFactory(log zerolog.Logger, committee hotstuff.Committee, eventHandler hotstuff.EventHandlerV2) *CombinedVoteProcessorFactory {
	return &CombinedVoteProcessorFactory{
		log:       log,
		committee: committee,
		packer:    &signature.ConsensusSigPackerImpl{}, // TODO: initialize properly when ready
		onQCCreated: func(qc *flow.QuorumCertificate) {
			err := eventHandler.OnQCConstructed(qc)
			if err != nil {
				log.Fatal().Err(err).Msgf("failed to submit constructed QC at view %d to event handler", qc.View)
			}
		},
	}
}

// Create creates CombinedVoteProcessor for processing votes for current proposal.
// After constructing CombinedVoteProcessor validity of proposer vote is checked.
// Caller can be sure that proposal vote was verified and processed.
// Expected error returns during normal operations:
// * model.InvalidBlockError - proposal has invalid proposer vote
func (f *CombinedVoteProcessorFactory) Create(proposal *model.Proposal) (*CombinedVoteProcessor, error) {
	allParticipants, err := f.committee.Identities(proposal.Block.BlockID, filter.Any)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants: %w", err)
	}

	// message that has to be verified against aggregated signature
	_ = verification.MakeVoteMessage(proposal.Block.View, proposal.Block.BlockID)

	// TODO: initialize properly when ready
	stakingSigAggtor := &signature.WeightedSignatureAggregator{}
	rbSigAggtor := &signature.WeightedSignatureAggregator{}
	rbRector := &signature.RandomBeaconReconstructor{}

	minRequiredStake := hotstuff.ComputeStakeThresholdForBuildingQC(allParticipants.TotalStake())

	processor := &CombinedVoteProcessor{
		log:              f.log,
		block:            proposal.Block,
		stakingSigAggtor: stakingSigAggtor,
		rbSigAggtor:      rbSigAggtor,
		rbRector:         rbRector,
		onQCCreated:      f.onQCCreated,
		packer:           f.packer,
		minRequiredStake: minRequiredStake,
		done:             *atomic.NewBool(false),
	}

	err = processor.Process(proposal.ProposerVote())
	if err != nil {
		if model.IsInvalidVoteError(err) {
			return nil, model.InvalidBlockError{
				BlockID: proposal.Block.BlockID,
				View:    proposal.Block.View,
				Err:     err,
			}
		}
		return nil, fmt.Errorf("could not process proposer's vote from block %v: %w", proposal.Block.BlockID, err)
	}
	return processor, nil
}
