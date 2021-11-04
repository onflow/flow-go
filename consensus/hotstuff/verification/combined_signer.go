package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// CombinedSigner is a signer capable of creating two signatures for each signing
// operation, combining them and adding them as the signature data on the generated
// data structures. The first type is an aggregating signer, which creates single
// signatures that can be aggregated into a single aggregated signature. The second
// type is a threshold signer, which creates threshold signature shares, which can
// be used to reconstruct a threshold signature.
type CombinedSigner struct {
	staking           module.MsgSigner
	merger            module.Merger
	beaconSignerStore module.RandomBeaconSigner
	signerID          flow.Identifier
}

// NewCombinedSigner creates a new combined signer with the given dependencies:
// - the hotstuff committee's state is used to retrieve public keys for signers;
// - the staking signer is used to create and verify aggregatable signatures for the first signature part;
// - the merger is used to join and split the two signature parts on our models;
// - the thresholdSignerStore is used to get threshold-signers by epoch/view;
// - the signer ID is used as the identity when creating signatures;
func NewCombinedSigner(
	committee hotstuff.Committee,
	staking module.MsgSigner,
	merger module.Merger,
	beaconSignerStore module.RandomBeaconSignerStore,
	signerID flow.Identifier) *CombinedSigner {

	sc := &CombinedSigner{
		staking:  staking,
		merger:   merger,
		beacon:   randomBeaconSignerStore,
		signerID: signerID,
	}
	return sc
}

// CreateProposal will create a proposal with a combined signature for the given block.
func (c *CombinedSigner) CreateProposal(block *model.Block) (*model.Proposal, error) {

	// check that the block is created by us
	if block.ProposerID != c.signerID {
		return nil, fmt.Errorf("can't create proposal for someone else's block")
	}

	// create the signature data
	sigData, err := c.genSigData(block)
	if err != nil {
		return nil, fmt.Errorf("could not create signature: %w", err)
	}

	// create the proposal
	proposal := &model.Proposal{
		Block:   block,
		SigData: sigData,
	}

	return proposal, nil
}

// CreateVote will create a vote with a combined signature for the given block.
func (c *CombinedSigner) CreateVote(block *model.Block) (*model.Vote, error) {

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
func (c *CombinedSigner) genSigData(block *model.Block) ([]byte, error) {

	// create the message to be signed and generate signatures
	msg := MakeVoteMessage(block.View, block.BlockID)
	stakingSig, err := c.staking.Sign(msg)
	if err != nil {
		return nil, fmt.Errorf("could not generate staking signature: %w", err)
	}

	beacon, err := c.beaconSignerStore.GetSigner(block.View)
	if err != nil {
		return nil, fmt.Errorf("could not get threshold signer for view %d: %w", block.View, err)
	}
	beaconShare, err := beacon.Sign(msg)
	if err != nil {
		return nil, fmt.Errorf("could not generate beacon signature: %w", err)
	}

	// combine the two signatures into one byte slice
	combinedSig, err := c.merger.Join(stakingSig, beaconShare)
	if err != nil {
		return nil, fmt.Errorf("could not join signatures: %w", err)
	}

	return combinedSig, nil
}
