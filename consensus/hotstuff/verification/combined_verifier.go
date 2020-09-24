package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module"
)

// CombinedVerifier is a verifier capable of verifying two signatures for each
// verifying operation. The first type is a signature from an aggregating signer,
// which verifies either the single or the aggregated signature. The second type is
// a signature from a threshold signer, which verifies either the signature share or
// the reconstructed threshold signature.
type CombinedVerifier struct {
	committee hotstuff.Committee
	dkg       hotstuff.DKG
	staking   module.AggregatingVerifier
	beacon    module.ThresholdVerifier
	merger    module.Merger
}

// NewCombinedVerifier creates a new combined verifier with the given dependencies.
// - the hotstuff committee's state is used to retrieve the public keys for the staking signature;
// - the DKG state is used to retrieve DKG data necessary to verify beacon signatures;
// - the staking verifier is used to verify single & aggregated staking signatures;
// - the beacon verifier is used to verify signature shares & threshold signatures;
// - the merger is used to combined & split staking & random beacon signatures; and
func NewCombinedVerifier(committee hotstuff.Committee, staking module.AggregatingVerifier, beacon module.ThresholdVerifier, merger module.Merger) *CombinedVerifier {
	c := &CombinedVerifier{
		committee: committee,
		staking:   staking,
		beacon:    beacon,
		merger:    merger,
	}
	return c
}

// VerifyVote verifies the validity of a combined signature on a vote.
func (c *CombinedVerifier) VerifyVote(voterID flow.Identifier, sigData []byte, block *model.Block) (bool, error) {

	// create the to-be-signed message
	msg := makeVoteMessage(block.View, block.BlockID)

	// get the set of signing participants
	participants, err := c.committee.Identities(block.BlockID, filter.Any)
	if err != nil {
		return false, fmt.Errorf("could not get participants: %w", err)
	}

	// get the specific identity
	signer, ok := participants.ByNodeID(voterID)
	if !ok {
		return false, fmt.Errorf("voter %x is not a valid consensus participant at block %x: %w", voterID, block.BlockID, model.ErrInvalidSigner)
	}

	// split the two signatures from the vote
	splitSigs, err := c.merger.Split(sigData)
	if err != nil {
		return false, fmt.Errorf("could not split signature: %w", ErrInvalidFormat)
	}

	// check if we have two signature
	if len(splitSigs) != 2 {
		return false, fmt.Errorf("wrong number of combined signatures: %w", ErrInvalidFormat)
	}

	// assign the signtures
	stakingSig := splitSigs[0]
	beaconShare := splitSigs[1]

	dkg, err := c.committee.DKG(block.BlockID)
	if err != nil {
		return false, fmt.Errorf("could not get dkg: %w", err)
	}

	// get the signer dkg key share
	beaconPubKey, err := dkg.KeyShare(voterID)
	if err != nil {
		return false, fmt.Errorf("could not get random beacon key share for %x: %w", voterID, err)
	}

	// verify each signature against the message
	stakingValid, err := c.staking.Verify(msg, stakingSig, signer.StakingPubKey)
	if err != nil {
		return false, fmt.Errorf("could not verify staking signature: %w", err)
	}
	beaconValid, err := c.beacon.Verify(msg, beaconShare, beaconPubKey)
	if err != nil {
		return false, fmt.Errorf("could not verify beacon signature: %w", err)
	}

	return stakingValid && beaconValid, nil
}

// VerifyQC verifies the validity of a combined signature on a quorum certificate.
func (c *CombinedVerifier) VerifyQC(voterIDs []flow.Identifier, sigData []byte, block *model.Block) (bool, error) {

	// get the full Identities of the signers
	signers, err := c.committee.Identities(block.BlockID, filter.HasNodeID(voterIDs...))
	if err != nil {
		return false, fmt.Errorf("could not get signer identities: %w", err)
	}
	if len(signers) < len(voterIDs) { // check we have valid consensus member Identities for all signers
		return false, fmt.Errorf("some signers are not valid consensus participants at block %x: %w", block.BlockID, model.ErrInvalidSigner)
	}
	signers = signers.Order(order.ByReferenceOrder(voterIDs)) // re-arrange Identities into the same order as in voterIDs

	dkg, err := c.committee.DKG(block.BlockID)
	if err != nil {
		return false, fmt.Errorf("could not get dkg: %w", err)
	}

	// split the aggregated staking & beacon signatures
	splitSigs, err := c.merger.Split(sigData)
	if err != nil {
		return false, fmt.Errorf("could not split signature: %w", ErrInvalidFormat)
	}

	// check we have the right amount of split sigs
	if len(splitSigs) != 2 {
		return false, fmt.Errorf("invalid number of split signatures: %w", ErrInvalidFormat)
	}

	// assign the signatures
	stakingAggSig := splitSigs[0]
	beaconThresSig := splitSigs[1]

	// verify the aggregated staking signature first
	msg := makeVoteMessage(block.View, block.BlockID)
	stakingValid, err := c.staking.VerifyMany(msg, stakingAggSig, signers.StakingKeys())
	if err != nil {
		return false, fmt.Errorf("could not verify staking signature: %w", err)
	}
	beaconValid, err := c.beacon.VerifyThreshold(msg, beaconThresSig, dkg.GroupKey())
	if err != nil {
		return false, fmt.Errorf("could not verify beacon signature: %w", err)
	}

	return stakingValid && beaconValid, nil
}
