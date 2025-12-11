package votecollector

import (
	"errors"
	"fmt"

	"github.com/onflow/crypto"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/protocol"
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
	committee   hotstuff.DynamicCommittee
	onQCCreated hotstuff.OnQCCreated
	packer      hotstuff.Packer
}

// Create creates CombinedVoteProcessorV3 for processing votes for the given block.
// Caller must treat all errors as exceptions
// nolint:unused
func (f *combinedVoteProcessorFactoryBaseV3) Create(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
	allParticipants, err := f.committee.IdentitiesByBlock(block.BlockID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants at block %v: %w", block.BlockID, err)
	}

	// message that has to be verified against aggregated signature
	msg := verification.MakeVoteMessage(block.View, block.BlockID)

	dkg, err := f.committee.DKG(block.View)
	if err != nil {
		return nil, fmt.Errorf("could not get DKG info at block %v: %w", block.BlockID, err)
	}

	// Prepare the staking public keys of participants.
	// CAUTION: while every participant must have a staking key (hence len(allParticipants) == len(stakingKeys))
	// some consensus nodes might not be part of the Random Beacon Committee.
	//   - We use 𝒫 as shorthand notation of `allParticipants`, which is the set of all nodes that are authorized to vote for `block`.
	//   - The DKG committee 𝒟 is the set of parties that were authorized to participate in the DKG (happy path; or
	//     eligible to receive a private key share from an alternative source on the fallback path).
	// With 𝓑 we denote the subset 𝓑 := (𝒟 ∩ 𝒫), i.e. all nodes that are authorized to vote for `block` _and_ are part of the
	// DKG committee. Only for nodes ρ ∈ 𝓑, the method `dkg.KeyShare(ρ.NodeID)` will return a public key. Note that there might
	// not exist a private key for ρ  (e.g. if ρ failed the DKG), but `dkg.KeyShare(ρ.NodeID)` nevertheless returns a key.
	stakingKeys := make([]crypto.PublicKey, 0, len(allParticipants))
	beaconParticipants := make(flow.IdentityList, 0, len(allParticipants))
	beaconKeys := make([]crypto.PublicKey, 0, len(allParticipants))
	for _, participant := range allParticipants {
		stakingKeys = append(stakingKeys, participant.StakingPubKey) // all nodes have staking keys
		pk, err := dkg.KeyShare(participant.NodeID)                  // but only a subset of nodes might have random beacon keys
		if err != nil {
			if protocol.IsIdentityNotFound(err) {
				continue
			}
			return nil, irrecoverable.NewException(fmt.Errorf("unexpected error retrieving random beacon key share for node %v: %w", participant.NodeID, err))
		}
		beaconParticipants = append(beaconParticipants, participant)
		beaconKeys = append(beaconKeys, pk)
	}

	stakingSigAggtor, err := signature.NewWeightedSignatureAggregator(allParticipants, stakingKeys, msg, msig.ConsensusVoteTag)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregator for staking signatures: %w", err)
	}

	beaconAggregator, err := signature.NewWeightedSignatureAggregator(beaconParticipants, beaconKeys, msg, msig.RandomBeaconTag)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregator for threshold signatures: %w", err)
	}

	threshold := msig.RandomBeaconThreshold(int(dkg.Size()))
	randomBeaconInspector, err := signature.NewRandomBeaconInspector(dkg.GroupKey(), dkg.KeyShares(), threshold, msg)
	if err != nil {
		return nil, fmt.Errorf("could not create random beacon inspector: %w", err)
	}

	rbRector := signature.NewRandomBeaconReconstructor(dkg, randomBeaconInspector)
	minRequiredWeight, err := f.committee.QuorumThresholdForView(block.View)
	if err != nil {
		return nil, fmt.Errorf("could not get weight threshold for view %d: %w", block.View, err)
	}

	return &CombinedVoteProcessorV3{
		log:               log.With().Hex("block_id", block.BlockID[:]).Logger(),
		block:             block,
		stakingSigAggtor:  stakingSigAggtor,
		rbSigAggtor:       beaconAggregator,
		rbRector:          rbRector,
		onQCCreated:       f.onQCCreated,
		packer:            f.packer,
		votesCache:        NewConcurrentIdentifierSet(),
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
	votesCache        *AppendOnlyIdentifierSet
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

// Process ingests a single vote. It can be called by multiple goroutines at the same time. While all valid
// votes must carry a staking signature, most nodes should solely provide their random beacon signatures but
// nodes can fall back to voting with their staking signature (e.g. node did not succeed the DKG).
// Design of this function is event driven: as soon as we collect enough signatures to create a QC, we will
// immediately do so and submit it via callback for further processing. However, due to concurrency, a few
// more than the minimum required signatures might be container in the QC (permitted by the protocol).
//
// IMPORTANT: The VerifyingVoteProcessor provides the final defense against any vote-equivocation attacks
// for its specific block. These attacks typically aim at multiple votes from the same node being counted
// towards the supermajority threshold. This must cover attacks by the leader concurrently utilizing
// stand-alone votes and votes embedded into the proposal.
//
// Expected error returns during normal operations:
//   - [VoteForIncompatibleBlockError] if vote is for incompatible block
//   - [VoteForIncompatibleViewError] if vote is for incompatible view
//   - [model.InvalidVoteError] if vote has invalid signature
//   - [model.DuplicatedSignerError] if the same vote from the same signer has been already added
//
// All other errors should be treated as exceptions.
//
// Impossibility of vote double-counting: All votes before being counted by the aggregator are first deduplicated using
// a dedicated votesCache which tracks votes by signerID this ensures that at most one vote will be processed from given
// signer. This means that [CombinedVoteProcessorV3] guarantees to process at most one vote per signer, everything else
// will be discarded as duplicate. We rely on the external components to detect and slash equivocation cases.
func (p *CombinedVoteProcessorV3) Process(vote *model.Vote) error {
	err := EnsureVoteForBlock(vote, p.block)
	if err != nil {
		return fmt.Errorf("received incompatible vote %v: %w", vote.ID(), err)
	}

	// Add vote to a local cache to track repeated and double votes before processing them by specific aggregators.
	// Consensus committee member can provide vote in two forms: a staking signature and a random beacon signature.
	// Therefore, we might receive two votes from one node, where the first vote gets processed by the StakingSigAggregator and
	// second one by RBSigAggregator. Since each of the aggregators tracks votes by signer ID, they cannot detect duplicated or
	// repeated votes if they were provided only one to each aggregator. It's impossible to deduplicate votes without relying on
	// external components to do the job. To increase modularity and BFT resilience of this component we are introducing a
	// votesCache which tracks votes by signer ID. Using votesCache we can guarantee that we will process at most one vote per
	// signer, which guarantees correctness of the QCs we produce without relying on external conditions.
	//
	// The way votesCache is used introduces some weak consistency between cache and aggregators; on happy path it's completely
	// straightforward approach. Adding a vote to the votesCache acts like a trapdoor for competing threads: only a single competing
	// thread for a specific voter will pass through and succeed in adding a vote to the aggregator(s).
	// It gets more interesting when any of the operations fail after we add the vote to the cache. The way this logic is structured,
	// it results in the following rule:
	// ▷ Only the very first vote that was added to the votesCache from the same signer will be processed by the component. ◁
	// It means that if the vote was invalid by any reason from the point of view of the aggregator, a second vote
	// won't be processed at all even if it might be correct from the point of view of the aggregator.
	// Formally: Let n denote the total state of the consensus committee. If we receive invalid votes from participants with a
	// combined state ≥ n/3, then we won't be able to produce a QC for this particular view. Otherwise, this component must eventually
	// always produce a QC, provided enough valid votes arrive.
	if !p.votesCache.Add(vote.SignerID) {
		return model.NewDuplicatedSignerErrorf("vote from %s has been already added", vote.SignerID)
	}

	// Vote Processing state machine
	if p.done.Load() {
		return nil
	}

	sigType, sig, err := msig.DecodeSingleSig(vote.SigData)
	if err != nil {
		if errors.Is(err, msig.ErrInvalidSignatureFormat) {
			return model.NewInvalidVoteErrorf(vote, "could not decode signature: %w", err)
		}
		return fmt.Errorf("unexpected error decoding vote %v: %w", vote.ID(), err)
	}

	switch sigType {

	case encoding.SigTypeStaking:
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

	case encoding.SigTypeRandomBeacon:
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
		return model.NewInvalidVoteErrorf(vote, "invalid signature type %d: %w", sigType, model.NewInvalidFormatErrorf(""))
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
	if !p.done.CompareAndSwap(false, true) {
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
		Hex("signers", qc.SignerIndices).
		Msg("new QC has been created")

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
	signerIndices, sigData, err := p.packer.Pack(p.block.View, blockSigData)
	if err != nil {
		return nil, fmt.Errorf("could not pack the block sig data: %w", err)
	}

	qc, err := flow.NewQuorumCertificate(flow.UntrustedQuorumCertificate{
		View:          p.block.View,
		BlockID:       p.block.BlockID,
		SignerIndices: signerIndices,
		SigData:       sigData,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build quorum certificate: %w", err)
	}

	return qc, nil
}
