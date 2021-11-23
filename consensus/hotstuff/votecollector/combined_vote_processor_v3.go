package votecollector

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	msig "github.com/onflow/flow-go/module/signature"
)

/* **************** Base-Factory for CombinedVoteProcessors ***************** */

// combinedVoteProcessorFactoryBaseV3 is a `votecollector.baseFactory` for creating
// CombinedVoteProcessors, holding all needed dependencies.
// combinedVoteProcessorFactoryBaseV3 is intended to be used for the main consensus.
// CAUTION:
// this base factory only creates the VerifyingVoteProcessor for the given block.
// It does _not_ check the proposer's vote for its own block, i.e. it does _not_
// implement `hotstuff.VoteProcessorFactory`. This base factory should be wrapped
// by `votecollector.VoteProcessorFactory` which adds the logic to verify
// the proposer's vote (decorator pattern).
// nolint:unused
type combinedVoteProcessorFactoryBaseV3 struct {
	log         zerolog.Logger
	committee   hotstuff.Committee
	onQCCreated hotstuff.OnQCCreated
	packer      hotstuff.Packer
}

// Create creates CombinedVoteProcessorV3 for processing votes for the given block.
// Caller must treat all errors as exceptions
// nolint:unused
func (f *combinedVoteProcessorFactoryBaseV3) Create(block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
	allParticipants, err := f.committee.Identities(block.BlockID, filter.Any)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants at block %v: %w", block.BlockID, err)
	}

	// message that has to be verified against aggregated signature
	msg := verification.MakeVoteMessage(block.View, block.BlockID)

	// prepare the staking public keys of participants
	stakingKeys := make([]crypto.PublicKey, 0, len(allParticipants))
	for _, participant := range allParticipants {
		stakingKeys = append(stakingKeys, participant.StakingPubKey)
	}

	stakingSigAggtor, err := signature.NewWeightedSignatureAggregator(allParticipants, stakingKeys, msg, encoding.ConsensusVoteTag)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregator for staking signatures: %w", err)
	}

	dkg, err := f.committee.DKG(block.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not get DKG info at block %v: %w", block.BlockID, err)
	}

	// prepare the random beacon public keys of participants
	beaconKeys := make([]crypto.PublicKey, 0, len(allParticipants))
	for _, participant := range allParticipants {
		pk, err := dkg.KeyShare(participant.NodeID)
		if err != nil {
			return nil, fmt.Errorf("could not get random beacon key share for %x: %w", participant.NodeID, err)
		}
		beaconKeys = append(beaconKeys, pk)
	}

	rbSigAggtor, err := signature.NewWeightedSignatureAggregator(allParticipants, beaconKeys, msg, encoding.RandomBeaconTag)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregator for thershold signatures: %w", err)
	}

	threshold := msig.RandomBeaconThreshold(int(dkg.Size()))
	randomBeaconInspector, err := signature.NewRandomBeaconInspector(dkg.GroupKey(), beaconKeys, threshold, msg)
	if err != nil {
		return nil, fmt.Errorf("could not create random beacon inspector: %w", err)
	}

	rbRector := signature.NewRandomBeaconReconstructor(dkg, randomBeaconInspector)
	minRequiredStake := hotstuff.ComputeStakeThresholdForBuildingQC(allParticipants.TotalStake())

	return &CombinedVoteProcessorV3{
		log:              f.log,
		block:            block,
		stakingSigAggtor: stakingSigAggtor,
		rbSigAggtor:      rbSigAggtor,
		rbRector:         rbRector,
		onQCCreated:      f.onQCCreated,
		packer:           f.packer,
		minRequiredStake: minRequiredStake,
		done:             *atomic.NewBool(false),
	}, nil
}

/* ****************** CombinedVoteProcessorV3 Implementation ****************** */

// CombinedVoteProcessorV3 implements the hotstuff.VerifyingVoteProcessor interface.
// It processes votes from the main consensus committee, where participants vote in
// favour of a block by proving either their staking key signature or their random
// beacon signature. In the former case, the participant only contributes to HotStuff
// progress; while in the latter case, the voter also contributes to running the
// random beacon. Concurrency safe.
type CombinedVoteProcessorV3 struct {
	log              zerolog.Logger
	block            *model.Block
	stakingSigAggtor hotstuff.WeightedSignatureAggregator
	rbSigAggtor      hotstuff.WeightedSignatureAggregator
	rbRector         hotstuff.RandomBeaconReconstructor
	onQCCreated      hotstuff.OnQCCreated
	packer           hotstuff.Packer
	minRequiredStake uint64
	done             atomic.Bool
}

var _ hotstuff.VoteProcessor = &CombinedVoteProcessorV3{}

// Block returns block that is part of proposal that we are processing votes for.
func (p *CombinedVoteProcessorV3) Block() *model.Block {
	return p.block
}

// Status returns status of this vote processor, it's always verifying.
func (p *CombinedVoteProcessorV3) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusVerifying
}

// Process performs processing of single vote in concurrent safe way. This function is implemented to be
// called by multiple goroutines at the same time. Supports processing of both staking and random beacon signatures.
// Design of this function is event driven: as soon as we collect enough signatures to create a QC we will immediately do so
// and submit it via callback for further processing.
// Expected error returns during normal operations:
// * VoteForIncompatibleBlockError - submitted vote for incompatible block
// * VoteForIncompatibleViewError - submitted vote for incompatible view
// * model.InvalidVoteError - submitted vote with invalid signature
// All other errors should be treated as exceptions.
func (p *CombinedVoteProcessorV3) Process(vote *model.Vote) error {
	err := EnsureVoteForBlock(vote, p.block)
	if err != nil {
		return fmt.Errorf("received incompatible vote %v: %w", vote.ID(), err)
	}

	// Vote Processing state machine
	if p.done.Load() {
		return nil
	}
	sigType, sig, err := signature.DecodeSingleSig(vote.SigData)
	if err != nil {
		if errors.Is(err, msig.ErrInvalidFormat) {
			return model.NewInvalidVoteErrorf(vote, "could not decode signature: %w", err)
		}
		return fmt.Errorf("unexpected error decoding vote %v: %w", vote.ID(), err)
	}

	switch sigType {

	case hotstuff.SigTypeStaking:
		err := p.stakingSigAggtor.Verify(vote.SignerID, sig)
		if err != nil {
			if errors.Is(err, msig.ErrInvalidFormat) {
				return model.NewInvalidVoteErrorf(vote, "vote %x for view %d has an invalid staking signature: %w",
					vote.ID(), vote.View, err)
			}
			return fmt.Errorf("internal error checking signature validity for vote %v: %w", vote.ID(), err)
		}
		if p.done.Load() {
			return nil
		}
		_, err = p.stakingSigAggtor.TrustedAdd(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("adding the signature to staking aggregator failed for vote %v: %w", vote.ID(), err)
		}

	case hotstuff.SigTypeRandomBeacon:
		err := p.rbSigAggtor.Verify(vote.SignerID, sig)
		if err != nil {
			if errors.Is(err, msig.ErrInvalidFormat) {
				return model.NewInvalidVoteErrorf(vote, "vote %x for view %d has an invalid random beacon signature: %w",
					vote.ID(), vote.View, err)
			}
			return fmt.Errorf("internal error checking signature validity for vote %v: %w", vote.ID(), err)
		}

		if p.done.Load() {
			return nil
		}
		_, err = p.rbSigAggtor.TrustedAdd(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("adding the signature to staking aggregator failed for vote %v: %w", vote.ID(), err)
		}
		_, err = p.rbRector.TrustedAdd(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("adding the signature to random beacon reconstructor failed for vote %v: %w", vote.ID(), err)
		}

	default:
		return model.NewInvalidVoteErrorf(vote, "invalid signature type %d: %w", sigType, msig.ErrInvalidFormat)
	}

	// checking of conditions for building QC are satisfied
	if p.stakingSigAggtor.TotalWeight()+p.rbSigAggtor.TotalWeight() < p.minRequiredStake {
		return nil
	}
	if !p.rbRector.EnoughShares() {
		return nil
	}

	// At this point, we have enough signatures to build a QC. Another routine
	// might just be at this point. To avoid duplicate work, only one routine can pass:
	if !p.done.CAS(false, true) {
		return nil
	}

	// Our algorithm for checking votes and adding them to the aggregators should
	// guarantee that we are _always_ able to successfully construct a QC when we
	// reach this point. A failure implies that the VoteProcessor's internal state is corrupted.
	qc, err := p.buildQC()
	if err != nil {
		return fmt.Errorf("internal error constructing QC from votes: %w", err)
	}

	p.onQCCreated(qc)

	return nil
}

// buildQC performs aggregation and reconstruction of signatures when we have collected enough
// signatures for building a QC. This function is run only once by a single worker.
// Any error should be treated as exception.
func (p *CombinedVoteProcessorV3) buildQC() (*flow.QuorumCertificate, error) {
	stakingSigners, aggregatedStakingSig, err := p.stakingSigAggtor.Aggregate()
	if err != nil {
		return nil, fmt.Errorf("could not aggregate staking signature: %w", err)
	}
	beaconSigners, aggregatedRandomBeaconSig, err := p.rbSigAggtor.Aggregate()
	if err != nil {
		return nil, fmt.Errorf("could not aggregate random beacon signatures: %w", err)
	}
	reconstructedBeaconSig, err := p.rbRector.Reconstruct()
	if err != nil {
		return nil, fmt.Errorf("could not reconstruct random beacon group signature: %w", err)
	}

	blockSigData := &hotstuff.BlockSignatureData{
		StakingSigners:               stakingSigners,
		RandomBeaconSigners:          beaconSigners,
		AggregatedStakingSig:         aggregatedStakingSig,
		AggregatedRandomBeaconSig:    aggregatedRandomBeaconSig,
		ReconstructedRandomBeaconSig: reconstructedBeaconSig,
	}

	signerIDs, sigData, err := p.packer.Pack(p.block.BlockID, blockSigData)
	if err != nil {
		return nil, fmt.Errorf("could not pack the block sig data: %w", err)
	}

	return &flow.QuorumCertificate{
		View:      p.block.View,
		BlockID:   p.block.BlockID,
		SignerIDs: signerIDs,
		SigData:   sigData,
	}, nil
}
