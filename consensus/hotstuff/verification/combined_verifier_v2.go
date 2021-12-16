//go:build relic
// +build relic

package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
)

// CombinedVerifier is a verifier capable of verifying two signatures, one for each
// scheme. The first type is a signature from a staking signer,
// which verifies either a single or an aggregated signature. The second type is
// a signature from a random beacon signer, which verifies either the signature share or
// the reconstructed threshold signature.
type CombinedVerifier struct {
	committee     hotstuff.Committee
	stakingHasher hash.Hasher
	beaconHasher  hash.Hasher
	packer        hotstuff.Packer
}

// NewCombinedVerifier creates a new combined verifier with the given dependencies.
// - the hotstuff committee's state is used to retrieve the public keys for the staking signature;
// - the merger is used to combine and split staking and random beacon signatures;
// - the packer is used to unpack QC for verification;
func NewCombinedVerifier(committee hotstuff.Committee, packer hotstuff.Packer) *CombinedVerifier {
	return &CombinedVerifier{
		committee:     committee,
		stakingHasher: crypto.NewBLSKMAC(encoding.ConsensusVoteTag),
		beaconHasher:  crypto.NewBLSKMAC(encoding.RandomBeaconTag),
		packer:        packer,
	}
}

// VerifyVote verifies the validity of a combined signature from a vote.
// Usually this method is only used to verify the proposer's vote, which is
// the vote included in a block proposal.
// * signature.ErrInvalidFormat if the signature has an incompatible format.
// * model.ErrInvalidSignature is the signature is invalid
// * unexpected errors should be treated as symptoms of bugs or uncovered
//   edge cases in the logic (i.e. as fatal)
func (c *CombinedVerifier) VerifyVote(signer *flow.Identity, sigData []byte, block *model.Block) error {

	// create the to-be-signed message
	msg := MakeVoteMessage(block.View, block.BlockID)

	// split the two signatures from the vote
	stakingSig, beaconShare, err := signature.eecodeDoubleSig(sigData)
	if err != nil {
		return fmt.Errorf("could not split signature for block %v: %w", block.BlockID, err)
	}

	dkg, err := c.committee.DKG(block.BlockID)
	if err != nil {
		return fmt.Errorf("could not get dkg: %w", err)
	}

	// verify each signature against the message
	// TODO: check if using batch verification is faster (should be yes)
	stakingValid, err := signer.StakingPubKey.Verify(stakingSig, msg, c.stakingHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying staking signature of node %x at block %v: %w",
			signer.NodeID, block.BlockID, err)
	}
	if !stakingValid {
		return fmt.Errorf("invalid staking sig for block %v: %w", block.BlockID, model.ErrInvalidSignature)
	}

	// there is no beacon share, no need to verify it
	if beaconShare == nil {
		return nil
	}

	// if there is beacon share, there must be beacon public key
	beaconPubKey, err := dkg.KeyShare(signer.NodeID)
	if err != nil {
		return fmt.Errorf("could not get random beacon key share of node %x at block %v: %w",
			signer.NodeID, block.BlockID, err)
	}

	beaconValid, err := beaconPubKey.Verify(beaconShare, msg, c.beaconHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying beacon signature at block %v: %w",
			block.BlockID, err)
	}
	if !beaconValid {
		return fmt.Errorf("invalid beacon sig for block %v: %w", block.BlockID, model.ErrInvalidSignature)
	}
	return nil
}

// VerifyQC verifies the validity of a combined signature on a quorum certificate.
func (c *CombinedVerifier) VerifyQC(signers flow.IdentityList, sigData []byte, block *model.Block) error {

	dkg, err := c.committee.DKG(block.BlockID)
	if err != nil {
		return fmt.Errorf("could not get dkg data: %w", err)
	}

	// unpack sig data using packer
	blockSigData, err := c.packer.Unpack(block.BlockID, signers.NodeIDs(), sigData)
	if err != nil {
		return fmt.Errorf("could not split signature: %w", err)
	}

	msg := MakeVoteMessage(block.View, block.BlockID)

	// verify the beacon signature first since it is faster to verify (no public key aggregation needed)
	beaconValid, err := dkg.GroupKey().Verify(blockSigData.ReconstructedRandomBeaconSig, msg, c.beaconHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying beacon signature: %w", err)
	}
	if !beaconValid {
		return fmt.Errorf("invalid reconstructed random beacon sig for block (%x): %w", block.BlockID, model.ErrInvalidSignature)
	}

	pks := make([]crypto.PublicKey, 0, len(signers))
	for _, identity := range signers {
		pks = append(pks, identity.StakingPubKey)
	}

	// verify the aggregated staking signature next (more costly)
	// TODO: update to use module/signature.PublicKeyAggregator
	aggregatedKey, err := crypto.AggregateBLSPublicKeys(pks)
	if err != nil {
		return fmt.Errorf("could not compute aggregated key for block %x: %w", block.BlockID, err)
	}

	stakingValid, err := aggregatedKey.Verify(blockSigData.AggregatedStakingSig, msg, c.stakingHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying staking signature for block %x: %w", block.BlockID, err)
	}

	if !stakingValid {
		return fmt.Errorf("invalid aggregated staking sig for block %v: %w", block.BlockID, model.ErrInvalidSignature)
	}

	return nil
}
