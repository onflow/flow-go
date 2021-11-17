package verification

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// CombinedSignerV2 creates votes for the main consensus.
// When a participant votes for a block, it _always_ provide the staking signature
// as part of their vote. Furthermore, the participant can _optionally_
// also provide a random beacon signature. Through their staking signature, a
// participant always contributes to HotStuff's progress. Participation in the random
// beacon is optional (but encouraged). This allows nodes that failed the DKG to
// still contribute only to consensus (as fallback).
// TODO: to be replaced by CombinedSignerV3 for mature V2 solution.
// The difference between V2 and V3 is that V2 will sign 2 sigs, whereas
// V3 only sign 1 sig.
type CombinedSignerV2 struct {
	staking        module.Local
	stakingHasher  hash.Hasher
	beaconKeyStore module.RandomBeaconKeyStore
	beaconHasher   hash.Hasher
	signerID       flow.Identifier
}

// NewCombinedSignerV2 creates a new combined signer with the given dependencies:
// - the staking signer is used to create and verify aggregatable signatures for Hotstuff
// - the beaconKeyStore is used to get threshold-signers by epoch/view;
// - the signer ID is used as the identity when creating signatures;
func NewCombinedSignerV2(
	staking module.Local,
	beaconKeyStore module.RandomBeaconKeyStore,
	signerID flow.Identifier,
) *CombinedSignerV2 {

	sc := &CombinedSignerV2{
		staking:        staking,
		stakingHasher:  crypto.NewBLSKMAC(encoding.ConsensusVoteTag),
		beaconKeyStore: beaconKeyStore,
		beaconHasher:   crypto.NewBLSKMAC(encoding.RandomBeaconTag),
		signerID:       signerID,
	}
	return sc
}

// CreateProposal will create a proposal with a combined signature for the given block.
func (c *CombinedSignerV2) CreateProposal(block *model.Block) (*model.Proposal, error) {

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
func (c *CombinedSignerV2) CreateVote(block *model.Block) (*model.Vote, error) {

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
// It returns:
//  - (stakingSig, nil) if there is no random beacon private key.
//  - (stakingSig+randomBeaconSig, nil) if there is a random beacon private key.
//  - (nil, error) if there is any exception
func (c *CombinedSignerV2) genSigData(block *model.Block) ([]byte, error) {

	// create the message to be signed and generate signatures
	msg := MakeVoteMessage(block.View, block.BlockID)

	stakingSig, err := c.staking.Sign(msg, c.stakingHasher)
	if err != nil {
		return nil, fmt.Errorf("could not generate staking signature: %w", err)
	}

	beaconKey, err := c.beaconKeyStore.ByView(block.View)
	if err != nil {
		if errors.Is(err, module.DKGFailError) {
			return stakingSig, nil
		}
		return nil, fmt.Errorf("could not get random beacon private key for view %d: %w", block.View, err)
	}

	// if the node is a Random Beacon participant and has succeeded DKG, then using the random beacon key
	// to sign the block
	beaconShare, err := beaconKey.Sign(msg, c.beaconHasher)
	if err != nil {
		return nil, fmt.Errorf("could not generate beacon signature: %w", err)
	}

	return signature.EncodeDoubleSig(stakingSig, beaconShare), nil
}
