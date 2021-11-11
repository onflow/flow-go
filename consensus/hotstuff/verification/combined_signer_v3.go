package verification

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// CombinedSignerV3 creates votes for the main consensus. Per protocol specification
// a consensus participant can vote for a block by _either_ signing the block with
// its staking key _or_ with its random beacon key.
// If the participant provides signs with its random beacon key, it contributes to
// hotstuff _and_ the random beacon. A staking signature _only_ contributes to
// hotstuff. The latter is a useful fallback for a node that failed to properly
// participate in the DKG.
// The default behaviour of a node is to contribute to both the random beacon and
// hotstuff, as it will yield higher rewards. Therefore, we always sign with the
// random beacon key if it is available.
type CombinedSignerV3 struct {
	staking           module.MsgSigner
	beaconSignerStore module.RandomBeaconKeyStore
	hasher            hash.Hasher
	signerID          flow.Identifier
}

// NewCombinedSignerV3 creates a new combined signer with the given dependencies:
// - the hotstuff committee's state is used to retrieve public keys for signers;
// - the staking signer is used to create and verify aggregatable signatures for Hotstuff
// - the thresholdVerifier is used to verify threshold signatures
// - the thresholdSignerStore is used to get threshold-signers by epoch/view;
// - the signer ID is used as the identity when creating signatures;
func NewCombinedSignerV3(
	committee hotstuff.Committee,
	staking module.MsgSigner,
	beaconSignerStore module.RandomBeaconKeyStore,
	signerID flow.Identifier) *CombinedSignerV3 {

	sc := &CombinedSignerV3{
		staking:           staking,
		beaconSignerStore: beaconSignerStore,
		hasher:            crypto.NewBLSKMAC(encoding.RandomBeaconTag),
		signerID:          signerID,
	}
	return sc
}

// CreateProposal will create a proposal with a combined signature for the given block.
func (c *CombinedSignerV3) CreateProposal(block *model.Block) (*model.Proposal, error) {

	// check that the block is created by us
	if block.ProposerID != c.signerID {
		return nil, fmt.Errorf("can't create proposal for someone else's block")
	}

	// create the signature data
	sigData, err := c.genSigData(block)
	if err != nil {
		return nil, fmt.Errorf("signing my proposal failed: %w", err)
	}

	// create the proposal
	proposal := &model.Proposal{
		Block:   block,
		SigData: sigData,
	}

	return proposal, nil
}

// CreateVote will create a vote with a combined signature for the given block.
func (c *CombinedSignerV3) CreateVote(block *model.Block) (*model.Vote, error) {

	// create the signature data
	sigData, err := c.genSigData(block)
	if err != nil {
		return nil, fmt.Errorf("could not create signature: %w", err)
	}

	// create the vote
	vote := &model.Vote{
		View:     block.View,
		BlockID:  block.BlockID,
		SignerID: c.signerID,
		SigData:  sigData,
	}

	return vote, nil
}

// genSigData generates the signature data for our local node for the given block.
func (c *CombinedSignerV3) genSigData(block *model.Block) ([]byte, error) {

	// create the message to be signed and generate signatures
	msg := MakeVoteMessage(block.View, block.BlockID)

	beaconKey, err := c.beaconSignerStore.ByView(block.View)
	if err != nil {
		if errors.Is(err, module.DKGIncompleteError) {
			// if the node didn't complete DKG, then using the staking key to sign the block as a
			// fallback
			stakingSig, err := c.staking.Sign(msg)
			if err != nil {
				return nil, fmt.Errorf("could not generate staking signature: %w", err)
			}

			return signature.EncodeSingleSig(hotstuff.SigTypeStaking, stakingSig), nil
		}
		return nil, fmt.Errorf("could not get threshold signer for view %d: %w", block.View, err)
	}

	// if the node is a Random Beacon participant and has completed its DKG, then using the random beacon key
	// to sign the block
	beaconShare, err := beaconKey.Sign(msg, c.hasher)
	if err != nil {
		return nil, fmt.Errorf("could not generate beacon signature: %w", err)
	}

	return signature.EncodeSingleSig(hotstuff.SigTypeRandomBeacon, beaconShare), nil
}
