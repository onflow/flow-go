package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/signature"
)

// CombinedVerifier is a verifier capable of verifying two signatures for each
// verifying operation. The first type is a signature from an aggregating signer,
// which verifies either the single or the aggregated signature. The second type is
// a signature from a threshold signer, which verifies either the signature share or
// the reconstructed threshold signature.
type CombinedVerifier struct {
	committee      hotstuff.Committee
	staking        module.AggregatingVerifier
	keysAggregator *stakingKeysAggregator
	beacon         module.ThresholdVerifier
	merger         module.Merger
	packer         hotstuff.Packer
}

// NewCombinedVerifier creates a new combined verifier with the given dependencies.
// - the hotstuff committee's state is used to retrieve the public keys for the staking signature;
// - the DKG state is used to retrieve DKG data necessary to verify beacon signatures;
// - the staking verifier is used to verify single & aggregated staking signatures;
// - the beacon verifier is used to verify signature shares & threshold signatures;
// - the merger is used to combined & split staking & random beacon signatures; and
func NewCombinedVerifier(committee hotstuff.Committee, staking module.AggregatingVerifier, beacon module.ThresholdVerifier, merger module.Merger) *CombinedVerifier {
	c := &CombinedVerifier{
		committee:      committee,
		staking:        staking,
		keysAggregator: newStakingKeysAggregator(),
		beacon:         beacon,
		merger:         merger,
	}
	return c
}

// VerifyVote verifies the validity of a combined signature from a vote.
func (c *CombinedVerifier) VerifyVote(signer *flow.Identity, sigData []byte, block *model.Block) (bool, error) {

	// create the to-be-signed message
	msg := MakeVoteMessage(block.View, block.BlockID)

	// split the two signatures from the vote
	stakingSig, beaconShare, err := c.merger.Split(sigData)
	if err != nil {
		return false, fmt.Errorf("could not split signature: %w", signature.ErrInvalidFormat)
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
	stakingValid, err := c.staking.Verify(msg, stakingSig, signer.StakingPubKey)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	if !stakingValid {
		return false, nil
	}
	beaconValid, err := c.beacon.Verify(msg, beaconShare, beaconPubKey)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying beacon signature: %w", err)
	}

	return beaconValid, nil
}

// VerifyQC verifies the validity of a combined signature on a quorum certificate.
func (c *CombinedVerifier) VerifyQC(signers flow.IdentityList, sigData []byte, block *model.Block) (bool, error) {

	dkg, err := c.committee.DKG(block.BlockID)
	if err != nil {
		return false, fmt.Errorf("could not get dkg: %w", err)
	}

	// split the aggregated staking & beacon signatures
	stakingAggSig, beaconThresSig, err := c.merger.Split(sigData)
	if err != nil {
		return false, fmt.Errorf("could not split signature: %w", signature.ErrInvalidFormat)
	}

	msg := MakeVoteMessage(block.View, block.BlockID)
	// TODO: verify if batch verification is faster

	// verify the beacon signature first
	beaconValid, err := c.beacon.VerifyThreshold(msg, beaconThresSig, dkg.GroupKey())
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
	stakingValid, err := c.staking.Verify(msg, stakingAggSig, aggregatedKey)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	return stakingValid, nil
}

// VerifyConsensusQC verifies the validity of a combined signature on a quorum certificate.
func (c *CombinedVerifier) VerifyConsensusQC(signers flow.IdentityList, sigData []byte, block *model.Block) error {

	dkg, err := c.committee.DKG(block.BlockID)
	if err != nil {
		return fmt.Errorf("could not get dkg: %w", err)
	}

	aggregatedSigData, err := c.packer.Unpack(block.BlockID, signers, sigData)
	if err != nil {
		return fmt.Errorf("could not unpack sig data for block (%v): %w", block.BlockID, sigData)
	}

	msg := MakeVoteMessage(block.View, block.BlockID)
	// TODO: verify if batch verification is faster

	// verify the beacon signature first
	beaconValid, err := c.beacon.VerifyThreshold(msg, aggregatedSigData.ReconstructedRandomBeaconSig, dkg.GroupKey())
	if err != nil {
		return fmt.Errorf("internal error while verifying beacon signature: %w", err)
	}
	if !beaconValid {
		return fmt.Errorf("invalid random beacon signature: %w", signature.ErrInvalidFormat)
	}
	// verify the aggregated staking signature next (more costly)
	// TODO: eventually VerifyMany will be a method of a stateful struct. The struct would
	// hold the message, all the participants keys, the latest verification aggregated public key,
	// as well as the latest list of signers (preferably a bit vector, using indices).
	// VerifyMany would only take the signature and the new list of signers (a bit vector preferably)
	// as inputs. A new struct needs to be used for each epoch since the list of participants is upadted.

	aggregatedKey, err := c.keysAggregator.aggregatedStakingKey(aggregatedSigData.StakingSigners)
	if err != nil {
		return fmt.Errorf("could not compute aggregated key: %w", err)
	}

	aggregatedStakingValid, err := c.staking.Verify(msg, aggregatedSigData.AggregatedStakingSig, aggregatedKey)
	if err != nil {
		return fmt.Errorf("internal error while verifying aggregated staking signature: %w", err)
	}

	if !aggregatedStakingValid {
		return fmt.Errorf("invalid aggregated staking sig: %w", signature.ErrInvalidFormat)
	}

	aggregatedBeaconKey, err := c.keysAggregator.aggregatedStakingKey(aggregatedSigData.RandomBeaconSigners)
	if err != nil {
		return fmt.Errorf("could not compute aggregated key: %w", err)
	}

	aggregatedBeaconValid, err := c.staking.Verify(msg, aggregatedSigData.AggregatedRandomBeaconSig, aggregatedBeaconKey)
	if err != nil {
		return fmt.Errorf("internal error while verifying aggregated random beacon signature: %w", err)
	}

	if !aggregatedBeaconValid {
		return fmt.Errorf("invalid aggregated random beacon sig error: %w", signature.ErrInvalidFormat)
	}

	return nil
}
