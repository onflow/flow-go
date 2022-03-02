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

// combinedVoteProcessorFactoryBaseV2 is a `votecollector.baseFactory` for creating
// CombinedVoteProcessors, holding all needed dependencies.
// combinedVoteProcessorFactoryBaseV2 is intended to be used for the main consensus.
// CAUTION:
// this base factory only creates the VerifyingVoteProcessor for the given block.
// It does _not_ check the proposer's vote for its own block, i.e. it does _not_
// implement `hotstuff.VoteProcessorFactory`. This base factory should be wrapped
// by `votecollector.VoteProcessorFactory` which adds the logic to verify
// the proposer's vote (decorator pattern).
type combinedVoteProcessorFactoryBaseV2 struct {
	committee   hotstuff.Committee
	onQCCreated hotstuff.OnQCCreated
	packer      hotstuff.Packer
}

// Create creates CombinedVoteProcessorV2 for processing votes for the given block.
// Caller must treat all errors as exceptions
func (f *combinedVoteProcessorFactoryBaseV2) Create(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
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
		return nil, fmt.Errorf("could not create aggregator for staking signatures at block %v: %w", block.BlockID, err)
	}

	publicKeyShares := make([]crypto.PublicKey, 0, len(allParticipants))
	dkg, err := f.committee.DKG(block.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not get DKG info at block %v: %w", block.BlockID, err)
	}
	for _, participant := range allParticipants {
		pk, err := dkg.KeyShare(participant.NodeID)
		if err != nil {
			return nil, fmt.Errorf("could not get random beacon key share for %x at block %v: %w", participant.NodeID, block.BlockID, err)
		}
		publicKeyShares = append(publicKeyShares, pk)
	}

	threshold := msig.RandomBeaconThreshold(int(dkg.Size()))
	randomBeaconInspector, err := signature.NewRandomBeaconInspector(dkg.GroupKey(), publicKeyShares, threshold, msg)
	if err != nil {
		return nil, fmt.Errorf("could not create random beacon inspector at block %v: %w", block.BlockID, err)
	}

	rbRector := signature.NewRandomBeaconReconstructor(dkg, randomBeaconInspector)
	minRequiredWeight := hotstuff.ComputeWeightThresholdForBuildingQC(allParticipants.TotalWeight())

	return NewCombinedVoteProcessor(
		log,
		block,
		stakingSigAggtor,
		rbRector,
		f.onQCCreated,
		f.packer,
		minRequiredWeight,
	), nil
}

/* ****************** CombinedVoteProcessorV2 Implementation ****************** */

// CombinedVoteProcessorV2 implements the hotstuff.VerifyingVoteProcessor interface.
// It processes votes from the main consensus committee, where participants must
// _always_ provide the staking signature as part of their vote and can _optionally_
// also provide a random beacon signature. Through their staking signature, a
// participant always contributes to HotStuff's progress. Participation in the random
// beacon is optional (but encouraged). This allows nodes that failed the DKG to
// still contribute only to consensus (as fallback).
// CombinedVoteProcessorV2 is Concurrency safe.
type CombinedVoteProcessorV2 struct {
	log               zerolog.Logger
	block             *model.Block
	stakingSigAggtor  hotstuff.WeightedSignatureAggregator
	rbRector          hotstuff.RandomBeaconReconstructor
	onQCCreated       hotstuff.OnQCCreated
	packer            hotstuff.Packer
	minRequiredWeight uint64
	done              atomic.Bool
}

var _ hotstuff.VerifyingVoteProcessor = (*CombinedVoteProcessorV2)(nil)

func NewCombinedVoteProcessor(log zerolog.Logger,
	block *model.Block,
	stakingSigAggtor hotstuff.WeightedSignatureAggregator,
	rbRector hotstuff.RandomBeaconReconstructor,
	onQCCreated hotstuff.OnQCCreated,
	packer hotstuff.Packer,
	minRequiredWeight uint64,
) *CombinedVoteProcessorV2 {
	return &CombinedVoteProcessorV2{
		log:               log.With().Hex("block_id", block.BlockID[:]).Logger(),
		block:             block,
		stakingSigAggtor:  stakingSigAggtor,
		rbRector:          rbRector,
		onQCCreated:       onQCCreated,
		packer:            packer,
		minRequiredWeight: minRequiredWeight,
		done:              *atomic.NewBool(false),
	}
}

// Block returns block that is part of proposal that we are processing votes for.
func (p *CombinedVoteProcessorV2) Block() *model.Block {
	return p.block
}

// Status returns status of this vote processor, it's always verifying.
func (p *CombinedVoteProcessorV2) Status() hotstuff.VoteCollectorStatus {
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
// * model.DuplicatedSignerError if the signer has been already added
// All other errors should be treated as exceptions.
//
// Impossibility of vote double-counting: Our signature scheme requires _every_ vote to supply a
// staking signature. Therefore, the `stakingSigAggtor` has the set of _all_ signerIDs that have
// provided a valid vote. Hence, the `stakingSigAggtor` guarantees that only a single vote can
// be successfully added for each `signerID`, i.e. double-counting votes is impossible.
func (p *CombinedVoteProcessorV2) Process(vote *model.Vote) error {
	err := EnsureVoteForBlock(vote, p.block)
	if err != nil {
		return fmt.Errorf("received incompatible vote %v: %w", vote.ID(), err)
	}

	// Vote Processing state machine
	if p.done.Load() {
		return nil
	}
	stakingSig, randomBeaconSig, err := signature.DecodeDoubleSig(vote.SigData)
	if err != nil {
		if errors.Is(err, model.ErrInvalidFormat) {
			return model.NewInvalidVoteErrorf(vote, "could not decode signature: %w", err)
		}
		return fmt.Errorf("unexpected error decoding vote %v: %w", vote.ID(), err)
	}

	// Verify staking sig.
	err = p.stakingSigAggtor.Verify(vote.SignerID, stakingSig)
	if err != nil {
		if model.IsInvalidSignerError(err) {
			return model.NewInvalidVoteErrorf(vote, "vote %x for view %d is not from an authorized consensus participant: %w",
				vote.ID(), vote.View, err)
		}
		if errors.Is(err, model.ErrInvalidSignature) {
			return model.NewInvalidVoteErrorf(vote, "vote %x for view %d has an invalid staking signature: %w",
				vote.ID(), vote.View, err)
		}
		return fmt.Errorf("internal error checking signature validity for vote %v: %w", vote.ID(), err)
	}

	if p.done.Load() {
		return nil
	}

	// Verify random beacon sig
	if randomBeaconSig != nil {
		err = p.rbRector.Verify(vote.SignerID, randomBeaconSig)
		if err != nil {
			// InvalidSignerError is possible in case we have consensus participants that are _not_ part of the random beacon committee.
			if model.IsInvalidSignerError(err) {
				return model.NewInvalidVoteErrorf(vote, "vote %x for view %d is not from an authorized random beacon participant: %w",
					vote.ID(), vote.View, err)
			}
			if errors.Is(err, model.ErrInvalidSignature) {
				return model.NewInvalidVoteErrorf(vote, "vote %x for view %d has an invalid random beacon signature: %w",
					vote.ID(), vote.View, err)
			}
			return fmt.Errorf("internal error checking signature validity for vote %v: %w", vote.ID(), err)
		}
	}

	if p.done.Load() {
		return nil
	}

	// Add staking sig to aggregator.
	_, err = p.stakingSigAggtor.TrustedAdd(vote.SignerID, stakingSig)
	if err != nil {
		// we don't expect any errors here during normal operation, as we previously checked
		// for duplicated votes from the same signer and verified the signer+signature
		return fmt.Errorf("unexpected exception adding signature from vote %v to staking aggregator: %w", vote.ID(), err)
	}
	// Add random beacon sig to threshold sig reconstructor
	if randomBeaconSig != nil {
		_, err = p.rbRector.TrustedAdd(vote.SignerID, randomBeaconSig)
		if err != nil {
			// we don't expect any errors here during normal operation, as we previously checked
			// for duplicated votes from the same signer and verified the signer+signature
			return fmt.Errorf("unexpected exception adding signature from vote %v to random beacon reconstructor: %w", vote.ID(), err)
		}
	}

	// checking of conditions for building QC are satisfied
	if p.stakingSigAggtor.TotalWeight() < p.minRequiredWeight {
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

	p.log.Info().
		Uint64("view", qc.View).
		Int("num_signers", len(qc.SignerIDs)).
		Msg("new qc has been created")

	p.onQCCreated(qc)

	return nil
}

// buildQC performs aggregation and reconstruction of signatures when we have collected enough
// signatures for building a QC. This function is run only once by a single worker.
// Any error should be treated as exception.
func (p *CombinedVoteProcessorV2) buildQC() (*flow.QuorumCertificate, error) {
	stakingSigners, aggregatedStakingSig, err := p.stakingSigAggtor.Aggregate()
	if err != nil {
		return nil, fmt.Errorf("could not aggregate staking signature: %w", err)
	}
	reconstructedBeaconSig, err := p.rbRector.Reconstruct()
	if err != nil {
		return nil, fmt.Errorf("could not reconstruct random beacon group signature: %w", err)
	}

	blockSigData := buildBlockSignatureDataForV2(stakingSigners, aggregatedStakingSig, reconstructedBeaconSig)
	qc, err := buildQCWithPackerAndSigData(p.packer, p.block, blockSigData)
	if err != nil {
		return nil, err
	}

	return qc, nil
}

// buildBlockSignatureDataForV2 build a block sig data for V2
// It reuses the hotstuff.BlockSignatureData type to create the sig data without filling the RandomBeaconSigners field and
// the AggregatedRandomBeaconSig field, so that the packer can be reused by both V2 and V3 to pack QC's sig data.
func buildBlockSignatureDataForV2(
	stakingSigners []flow.Identifier,
	aggregatedStakingSig []byte,
	reconstructedBeaconSig crypto.Signature,
) *hotstuff.BlockSignatureData {
	return &hotstuff.BlockSignatureData{
		StakingSigners:               stakingSigners,
		AggregatedStakingSig:         aggregatedStakingSig,
		ReconstructedRandomBeaconSig: reconstructedBeaconSig,
	}
}

// buildQCWithPackerAndSigData builds the QC with the given packer and blockSigData
func buildQCWithPackerAndSigData(
	packer hotstuff.Packer,
	block *model.Block,
	blockSigData *hotstuff.BlockSignatureData,
) (*flow.QuorumCertificate, error) {
	signerIDs, sigData, err := packer.Pack(block.BlockID, blockSigData)
	if err != nil {
		return nil, fmt.Errorf("could not pack the block sig data: %w", err)
	}

	return &flow.QuorumCertificate{
		View:      block.View,
		BlockID:   block.BlockID,
		SignerIDs: signerIDs,
		SigData:   sigData,
	}, nil
}
