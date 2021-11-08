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
	"github.com/onflow/flow-go/model/flow"
	modulesig "github.com/onflow/flow-go/module/signature"
)

// CombinedVerifier is a verifier capable of verifying two signatures for each
// verifying operation. The first type is a signature from an aggregating signer,
// which verifies either the single or the aggregated signature. The second type is
// a signature from a threshold signer, which verifies either the signature share or
// the reconstructed threshold signature.
type CombinedVerifierV2 struct {
	committee      hotstuff.Committee
	staking        hash.Hasher
	beacon         hash.Hasher
	keysAggregator *stakingKeysAggregator
	packer         hotstuff.Packer
}

// NewCombinedVerifier creates a new combined verifier with the given dependencies.
// - the hotstuff committee's state is used to retrieve the public keys for the staking signature;
// - the staking tag is used to create hasher to verify staking signatures;
// - the beacon tag is used to create hasher to verify random beacon signatures;
// - the packer is used to unpack QC for verification;
func NewCombinedVerifierV2(committee hotstuff.Committee, stakingTag string, beaconTag string, packer hotstuff.Packer) *CombinedVerifierV2 {
	return &CombinedVerifierV2{
		committee:      committee,
		staking:        crypto.NewBLSKMAC(stakingTag),
		beacon:         crypto.NewBLSKMAC(beaconTag),
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

	// get the signer dkg key share
	beaconPubKey, err := dkg.KeyShare(signer.NodeID)
	if err != nil {
		return false, fmt.Errorf("could not get random beacon key share for %x: %w", signer.NodeID, err)
	}

	// verify each signature against the message
	// TODO: check if using batch verification is faster (should be yes)
	stakingValid, err := signer.StakingPubKey.Verify(stakingSig, msg, c.staking)
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

	beaconValid, err := beaconPubKey.Verify(beaconShare, msg, c.beacon)
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
		return false, fmt.Errorf("could not get dkg: %w", err)
	}

	// unpack sig data using packer
	blockSigData, err := c.packer.Unpack(block.BlockID, signers.NodeIDs(), sigData)
	if err != nil {
		return false, fmt.Errorf("could not split signature: %w", modulesig.ErrInvalidFormat)
	}

	msg := MakeVoteMessage(block.View, block.BlockID)
	// TODO: verify if batch verification is faster

	// verify the beacon signature first
	beaconValid, err := dkg.GroupKey().Verify(blockSigData.ReconstructedRandomBeaconSig, msg, c.beacon)
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

	aggregatedKey, err := c.keysAggregator.aggregatedStakingKey(signers)
	if err != nil {
		return false, fmt.Errorf("could not compute aggregated key: %w", err)
	}
	stakingValid, err := aggregatedKey.Verify(blockSigData.AggregatedStakingSig, msg, c.staking)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	return stakingValid, nil
}
