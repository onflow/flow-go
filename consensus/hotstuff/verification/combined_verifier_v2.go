//go:build relic
// +build relic

package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	modulesig "github.com/onflow/flow-go/module/signature"
)

// CombinedVerifierV2 is a verifier capable of verifying two signatures, one for each
// scheme. The first type is a signature from a staking signer,
// which verifies either a single or an aggregated signature. The second type is
// a signature from a random beacon signer, which verifies either the signature share or
// the reconstructed threshold signature.
type CombinedVerifierV2 struct {
	committee      hotstuff.Committee
	stakingHasher  hash.Hasher
	beaconHasher   hash.Hasher
	keysAggregator *stakingKeysAggregator
	packer         hotstuff.Packer
}

// NewCombinedVerifierV2 creates a new combined verifier with the given dependencies.
// - the hotstuff committee's state is used to retrieve the public keys for the staking signature;
// - the merger is used to combine and split staking and random beacon signatures;
// - the packer is used to unpack QC for verification;
func NewCombinedVerifierV2(committee hotstuff.Committee, packer hotstuff.Packer) *CombinedVerifierV2 {
	return &CombinedVerifierV2{
		committee:      committee,
		stakingHasher:  crypto.NewBLSKMAC(encoding.ConsensusVoteTag),
		beaconHasher:   crypto.NewBLSKMAC(encoding.RandomBeaconTag),
		keysAggregator: newStakingKeysAggregator(),
		packer:         packer,
	}
}

// VerifyVote verifies the validity of a combined signature from a vote.
// Usually this method is only used to verify the proposer's vote, which is
// the vote included in a block proposal.
// TODO: return error only, because when the sig is invalid, the returned bool
// can't indicate whether it's staking sig was invalid, or beacon sig was invalid.
func (c *CombinedVerifierV2) VerifyVote(signer *flow.Identity, sigData []byte, block *model.Block) (bool, error) {

	// create the to-be-signed message
	msg := MakeVoteMessage(block.View, block.BlockID)

	// split the two signatures from the vote
	// TODO: move DecodeDoubleSig to merger.Split
	stakingSig, beaconShare, err := signature.DecodeDoubleSig(sigData)
	if err != nil {
		return false, fmt.Errorf("could not split signature: %w", modulesig.ErrInvalidFormat)
	}

	dkg, err := c.committee.DKG(block.BlockID)
	if err != nil {
		return false, fmt.Errorf("could not get dkg: %w", err)
	}

	// verify each signature against the message
	// TODO: check if using batch verification is faster (should be yes)
	stakingValid, err := signer.StakingPubKey.Verify(stakingSig, msg, c.stakingHasher)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	if !stakingValid {
		return false, fmt.Errorf("invalid staking sig")
	}

	// there is no beacon share, no need to verify it
	if beaconShare == nil {
		return true, nil
	}

	// if there is beacon share, there must be beacon public key
	beaconPubKey, err := dkg.KeyShare(signer.NodeID)
	if err != nil {
		return false, fmt.Errorf("could not get random beacon key share for %x: %w", signer.NodeID, err)
	}

	beaconValid, err := beaconPubKey.Verify(beaconShare, msg, c.beaconHasher)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying beacon signature: %w", err)
	}

	if !beaconValid {
		return false, fmt.Errorf("invalid beacon sig")
	}
	return true, nil
}

// VerifyQC verifies the validity of a combined signature on a quorum certificate.
func (c *CombinedVerifierV2) VerifyQC(signers flow.IdentityList, sigData []byte, block *model.Block) (bool, error) {

	dkg, err := c.committee.DKG(block.BlockID)
	if err != nil {
		return false, fmt.Errorf("could not get dkg data: %w", err)
	}

	// unpack sig data using packer
	blockSigData, err := c.packer.Unpack(block.BlockID, signers.NodeIDs(), sigData)
	if err != nil {
		return false, fmt.Errorf("could not split signature: %w", modulesig.ErrInvalidFormat)
	}

	msg := MakeVoteMessage(block.View, block.BlockID)

	// verify the beacon signature first since it is faster to verify (no public key aggregation needed)
	beaconValid, err := dkg.GroupKey().Verify(blockSigData.ReconstructedRandomBeaconSig, msg, c.beaconHasher)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying beacon signature: %w", err)
	}
	if !beaconValid {
		return false, nil
	}
	// verify the aggregated staking signature next (more costly)
	// TODO: eventually VerifyMany will be a method of a stateful struct. The struct would
	// hold the message, all the participants keys, the latest verification aggregated public key,
	// as well as the latest list of signers (preferably a bit vector, using indices).
	// VerifyMany would only take the signature and the new list of signers (a bit vector preferably)
	// as inputs. A new struct needs to be used for each epoch since the list of participants is upadted.

	// TODO: update to use module/signature.PublicKeyAggregator
	aggregatedKey, err := c.keysAggregator.aggregatedStakingKey(signers)
	if err != nil {
		return false, fmt.Errorf("could not compute aggregated key: %w", err)
	}
	stakingValid, err := aggregatedKey.Verify(blockSigData.AggregatedStakingSig, msg, c.stakingHasher)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	return stakingValid, nil
}
