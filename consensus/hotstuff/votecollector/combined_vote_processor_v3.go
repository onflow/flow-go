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
	committee   hotstuff.Committee
	onQCCreated hotstuff.OnQCCreated
	packer      hotstuff.Packer
}

// Create creates CombinedVoteProcessorV3 for processing votes for the given block.
// Caller must treat all errors as exceptions
// nolint:unused
func (f *combinedVoteProcessorFactoryBaseV3) Create(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
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
	minRequiredWeight := hotstuff.ComputeWeightThresholdForBuildingQC(allParticipants.TotalWeight())

	return &CombinedVoteProcessorV3{
		log:               log.With().Hex("block_id", block.BlockID[:]).Logger(),
		block:             block,
		stakingSigAggtor:  stakingSigAggtor,
		rbSigAggtor:       rbSigAggtor,
		rbRector:          rbRector,
		onQCCreated:       f.onQCCreated,
		packer:            f.packer,
		minRequiredWeight: minRequiredWeight,
		done:              *atomic.NewBool(false),
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
	log               zerolog.Logger
	block             *model.Block
	stakingSigAggtor  hotstuff.WeightedSignatureAggregator
	rbSigAggtor       hotstuff.WeightedSignatureAggregator
	rbRector          hotstuff.RandomBeaconReconstructor
	onQCCreated       hotstuff.OnQCCreated
	packer            hotstuff.Packer
	minRequiredWeight uint64
	done              atomic.Bool
}

var _ hotstuff.VerifyingVoteProcessor = (*CombinedVoteProcessorV3)(nil)

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
// * model.DuplicatedSignerError - vote from a signer whose vote was previously already processed
// All other errors should be treated as exceptions.
//
// CAUTION: implementation is NOT (yet) BFT
// Explanation: for correctness, we require that no voter can be counted repeatedly. However,
// CombinedVoteProcessorV3 relies on the `VoteCollector`'s `votesCache` filter out all votes but the first for
// every signerID. However, we have the edge case, where we still feed the proposers vote twice into the
// `VerifyingVoteProcessor` (once as part of a cached vote, once as an individual vote). This can be exploited
// by a byzantine proposer to be erroneously counted twice, which would lead to a safety fault.
// TODO: (suggestion) I think it would be worth-while to include a second `votesCache` into the `CombinedVoteProcessorV3`.
//       Thereby,  `CombinedVoteProcessorV3` inherently guarantees correctness of the QCs it produces without relying on
//       external conditions (making the code more modular, less interdependent and thereby easier to maintain). The
//       runtime overhead is marginal: For `votesCache` to add 500 votes (concurrently with 20 threads) takes about
//       0.25ms. This runtime overhead is neglectable and a good tradeoff for the gain in maintainability and code clarity.
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
		if errors.Is(err, model.ErrInvalidFormat) {
			return model.NewInvalidVoteErrorf(vote, "could not decode signature: %w", err)
		}
		return fmt.Errorf("unexpected error decoding vote %v: %w", vote.ID(), err)
	}

	switch sigType {

	case hotstuff.SigTypeStaking:
		err := p.stakingSigAggtor.Verify(vote.SignerID, sig)
		if err != nil {
			if model.IsInvalidSignerError(err) {
				return model.NewInvalidVoteErrorf(vote, "vote %x for view %d is not signed by an authorized consensus participant: %w",
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
		_, err = p.stakingSigAggtor.TrustedAdd(vote.SignerID, sig)
		if err != nil {
			// we don't expect any errors here during normal operation, as we previously checked
			// for duplicated votes from the same signer and verified the signer+signature
			return fmt.Errorf("adding the signature to staking aggregator failed for vote %v: %w", vote.ID(), err)
		}

	case hotstuff.SigTypeRandomBeacon:
		err := p.rbSigAggtor.Verify(vote.SignerID, sig)
		if err != nil {
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

		if p.done.Load() {
			return nil
		}
		// Add signatures to `rbSigAggtor` and `rbRector`: we don't expect any errors during normal operation,
		// as we previously checked for duplicated votes from the same signer and verified the signer+signature
		_, err = p.rbSigAggtor.TrustedAdd(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("unexpected exception adding signature from vote %v to random beacon aggregator: %w", vote.ID(), err)
		}
		_, err = p.rbRector.TrustedAdd(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("unexpected exception adding signature from vote %v to random beacon reconstructor: %w", vote.ID(), err)
		}

	default:
		return model.NewInvalidVoteErrorf(vote, "invalid signature type %d: %w", sigType, model.ErrInvalidFormat)
	}

	// checking of conditions for building QC are satisfied
	if p.stakingSigAggtor.TotalWeight()+p.rbSigAggtor.TotalWeight() < p.minRequiredWeight {
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
func (p *CombinedVoteProcessorV3) buildQC() (*flow.QuorumCertificate, error) {
	// STEP 1: aggregate staking signatures (if there are any)
	// * It is possible that all replicas signed with their random beacon keys.
	//   Per Convention, we represent an empty set of staking signers as
	//   `stakingSigners` and `aggregatedStakingSig` both being zero-length
	//   (here, we use `nil`).
	// * If it has _not collected any_ signatures, `stakingSigAggtor.Aggregate()`
	//   errors with a `model.InsufficientSignaturesError`. We shortcut this case,
	//   and only call `Aggregate`, if the `stakingSigAggtor` has collected signatures
	//   with non-zero weight (i.e. at least one signature was collected).
	var stakingSigners []flow.Identifier // nil (zero value) represents empty set of staking signers
	var aggregatedStakingSig []byte      // nil (zero value) for empty set of staking signers
	if p.stakingSigAggtor.TotalWeight() > 0 {
		var err error
		stakingSigners, aggregatedStakingSig, err = p.stakingSigAggtor.Aggregate()
		if err != nil {
			return nil, fmt.Errorf("unexpected error aggregating staking signatures: %w", err)
		}
	}

	// STEP 2: reconstruct random beacon group sig and aggregate random beacon sig shares
	// Note: A valid random beacon group sig is required for QC validity. Our logic guarantees
	// that we always collect the minimally required number (non-zero) of signature shares.
	beaconSigners, aggregatedRandomBeaconSig, err := p.rbSigAggtor.Aggregate()
	if err != nil {
		return nil, fmt.Errorf("could not aggregate random beacon signatures: %w", err)
	}
	reconstructedBeaconSig, err := p.rbRector.Reconstruct()
	if err != nil {
		return nil, fmt.Errorf("could not reconstruct random beacon group signature: %w", err)
	}

	// STEP 3: generate BlockSignatureData and serialize it
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
