package verification

import (
	"errors"
	"fmt"

	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/protocol"
)

// CombinedVerifier is a verifier capable of verifying two signatures, one for each
// scheme. The first type is a signature from a staking signer,
// which verifies either a single or an aggregated signature. The second type is
// a signature from a random beacon signer, which verifies either the signature share or
// the reconstructed threshold signature.
type CombinedVerifier struct {
	committee           hotstuff.Replicas
	stakingHasher       hash.Hasher
	timeoutObjectHasher hash.Hasher
	beaconHasher        hash.Hasher
	packer              hotstuff.Packer
}

var _ hotstuff.Verifier = (*CombinedVerifier)(nil)

// NewCombinedVerifier creates a new combined verifier with the given dependencies.
// - the hotstuff committee's state is used to retrieve the public keys for the staking signature;
// - the merger is used to combine and split staking and random beacon signatures;
// - the packer is used to unpack QC for verification;
func NewCombinedVerifier(committee hotstuff.Replicas, packer hotstuff.Packer) *CombinedVerifier {
	return &CombinedVerifier{
		committee:           committee,
		stakingHasher:       msig.NewBLSHasher(msig.ConsensusVoteTag),
		timeoutObjectHasher: msig.NewBLSHasher(msig.ConsensusTimeoutTag),
		beaconHasher:        msig.NewBLSHasher(msig.RandomBeaconTag),
		packer:              packer,
	}
}

// VerifyVote verifies the validity of a combined signature from a vote.
// Usually this method is only used to verify the proposer's vote, which is
// the vote included in a block proposal.
//   - model.InvalidFormatError if the signature has an incompatible format.
//   - model.ErrInvalidSignature is the signature is invalid
//   - model.InvalidSignerError if signer is _not_ part of the random beacon committee
//   - model.ErrViewForUnknownEpoch if no epoch containing the given view is known
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
func (c *CombinedVerifier) VerifyVote(signer *flow.IdentitySkeleton, sigData []byte, view uint64, blockID flow.Identifier) error {

	// create the to-be-signed message
	msg := MakeVoteMessage(view, blockID)

	// split the two signatures from the vote
	stakingSig, beaconShare, err := msig.DecodeDoubleSig(sigData)
	if err != nil {
		if errors.Is(err, msig.ErrInvalidSignatureFormat) {
			return model.NewInvalidFormatErrorf("could not split signature for block %v: %w", blockID, err)
		}
		return fmt.Errorf("unexpected internal error while splitting signature for block %v: %w", blockID, err)
	}

	dkg, err := c.committee.DKG(view)
	if err != nil {
		return fmt.Errorf("could not get dkg: %w", err)
	}

	// verify each signature against the message
	// TODO: check if using batch verification is faster (should be yes)
	stakingValid, err := signer.StakingPubKey.Verify(stakingSig, msg, c.stakingHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying staking signature of node %x at block %v: %w",
			signer.NodeID, blockID, err)
	}
	if !stakingValid {
		return fmt.Errorf("invalid staking sig for block %v: %w", blockID, model.ErrInvalidSignature)
	}

	// there is no beacon share, no need to verify it
	if beaconShare == nil {
		return nil
	}

	// if there is beacon share, there should be beacon public key
	beaconPubKey, err := dkg.KeyShare(signer.NodeID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return model.NewInvalidSignerErrorf("%v is not a random beacon participant: %w", signer.NodeID, err)
		}
		return fmt.Errorf("unexpected error retrieving random beacon key share for node %x at block %v: %w",
			signer.NodeID, blockID, err)
	}

	beaconValid, err := beaconPubKey.Verify(beaconShare, msg, c.beaconHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying beacon signature at block %v: %w",
			blockID, err)
	}
	if !beaconValid {
		return fmt.Errorf("invalid beacon sig for block %v: %w", blockID, model.ErrInvalidSignature)
	}
	return nil
}

// VerifyQC checks the cryptographic validity of the QC's `sigData` for the
// given block. It is the responsibility of the calling code to ensure
// that all `signers` are authorized, without duplicates. Return values:
//   - nil if `sigData` is cryptographically valid
//   - model.InsufficientSignaturesError if `signers` is empty.
//     Depending on the order of checks in the higher-level logic this error might
//     be an indicator of an external byzantine input or an internal bug.
//   - model.InvalidFormatError if `sigData` has an incompatible format
//   - model.ErrInvalidSignature if a signature is invalid
//   - model.ErrViewForUnknownEpoch if no epoch containing the given view is known
//   - error if running into any unexpected exception (i.e. fatal error)
func (c *CombinedVerifier) VerifyQC(signers flow.IdentitySkeletonList, sigData []byte, view uint64, blockID flow.Identifier) error {
	dkg, err := c.committee.DKG(view)
	if err != nil {
		return fmt.Errorf("could not get dkg data: %w", err)
	}

	// unpack sig data using packer
	blockSigData, err := c.packer.Unpack(signers, sigData)
	if err != nil {
		return fmt.Errorf("could not split signature: %w", err)
	}

	msg := MakeVoteMessage(view, blockID)

	// verify the beacon signature first since it is faster to verify (no public key aggregation needed)
	beaconValid, err := dkg.GroupKey().Verify(blockSigData.ReconstructedRandomBeaconSig, msg, c.beaconHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying beacon signature: %w", err)
	}
	if !beaconValid {
		return fmt.Errorf("invalid reconstructed random beacon sig for block (%x): %w", blockID, model.ErrInvalidSignature)
	}

	err = verifyAggregatedSignatureOneMessage(signers.PublicStakingKeys(), blockSigData.AggregatedStakingSig, c.stakingHasher, msg)
	if err != nil {
		return fmt.Errorf("verifying aggregated staking signature failed for block %v: %w", blockID, err)
	}

	return nil
}

// VerifyTC checks cryptographic validity of the TC's `sigData` w.r.t. the
// given view. It is the responsibility of the calling code to ensure
// that all `signers` are authorized, without duplicates. Return values:
//   - nil if `sigData` is cryptographically valid
//   - model.InsufficientSignaturesError if `signers is empty.
//   - model.InvalidFormatError if `signers`/`highQCViews` have differing lengths
//   - model.ErrInvalidSignature if a signature is invalid
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
func (c *CombinedVerifier) VerifyTC(signers flow.IdentitySkeletonList, sigData []byte, view uint64, highQCViews []uint64) error {
	stakingPks := signers.PublicStakingKeys()
	return verifyTCSignatureManyMessages(stakingPks, sigData, view, highQCViews, c.timeoutObjectHasher)
}
