package verification

import (
	"errors"
	"fmt"

	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	msig "github.com/onflow/flow-go/module/signature"
)

// CombinedSignerV3 creates votes for the main consensus.
// When a participant votes for a block, it _always_ provide the staking signature
// as part of their vote. Furthermore, the participant can _optionally_
// also provide a random beacon signature. Through their staking signature, a
// participant always contributes to HotStuff's progress. Participation in the random
// beacon is optional (but encouraged). This allows nodes that failed the DKG to
// still contribute only to consensus (as fallback).
// TODO: to be replaced by CombinedSignerV3 for mature V2 solution.
// The difference between V2 and V3 is that V2 will sign 2 sigs, whereas
// V3 only sign 1 sig.
type CombinedSignerV3 struct {
	staking             module.Local
	stakingHasher       hash.Hasher
	timeoutObjectHasher hash.Hasher
	beaconKeyStore      module.RandomBeaconKeyStore
	beaconHasher        hash.Hasher
}

var _ hotstuff.Signer = (*CombinedSignerV3)(nil)

// NewCombinedSignerV3 creates a new combined signer with the given dependencies:
// - the staking signer is used to create and verify aggregatable signatures for Hotstuff
// - the beaconKeyStore is used to get threshold-signers by epoch/view;
// - the signer ID is used as the identity when creating signatures;
func NewCombinedSignerV3(
	staking module.Local,
	beaconKeyStore module.RandomBeaconKeyStore,
) *CombinedSignerV3 {

	sc := &CombinedSignerV3{
		staking:             staking,
		stakingHasher:       msig.NewBLSHasher(msig.ConsensusVoteTag),
		timeoutObjectHasher: msig.NewBLSHasher(msig.ConsensusTimeoutTag),
		beaconKeyStore:      beaconKeyStore,
		beaconHasher:        msig.NewBLSHasher(msig.RandomBeaconTag),
	}
	return sc
}

// CreateProposal will create a proposal with a combined signature for the given block.
func (c *CombinedSignerV3) CreateProposal(block *model.Block) (*model.Proposal, error) {

	// check that the block is created by us
	if block.ProposerID != c.staking.NodeID() {
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
		SignerID: c.staking.NodeID(),
		SigData:  sigData,
	}

	return vote, nil
}

// CreateTimeout will create a signed timeout object for the given view.
// Timeout objects are only signed with the staking key (not beacon key).
func (c *CombinedSignerV3) CreateTimeout(curView uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) (*model.TimeoutObject, error) {
	// create timeout object specific message
	msg := MakeTimeoutMessage(curView, newestQC.View)
	sigData, err := c.staking.Sign(msg, c.timeoutObjectHasher)
	if err != nil {
		return nil, fmt.Errorf("could not generate signature for timeout object at view %d: %w", curView, err)
	}

	timeout := &model.TimeoutObject{
		View:       curView,
		NewestQC:   newestQC,
		LastViewTC: lastViewTC,
		SignerID:   c.staking.NodeID(),
		SigData:    sigData,
	}
	return timeout, nil
}

// genSigData generates the signature data for our local node for the given block.
func (c *CombinedSignerV3) genSigData(block *model.Block) ([]byte, error) {

	// create the message to be signed and generate signatures
	msg := MakeVoteMessage(block.View, block.BlockID)

	beaconKey, err := c.beaconKeyStore.ByView(block.View)
	if err != nil {
		// if the node failed DKG, then using the staking key to sign the block as a fallback
		if errors.Is(err, module.ErrNoBeaconKeyForEpoch) {
			stakingSig, err := c.staking.Sign(msg, c.stakingHasher)
			if err != nil {
				return nil, fmt.Errorf("could not generate staking signature: %w", err)
			}

			return msig.EncodeSingleSig(encoding.SigTypeStaking, stakingSig), nil
		}
		// in order to sign a block or vote, we must know the view's epoch to know the leader
		// reaching this point for an unknown epoch indicates a critical validation failure earlier on
		if errors.Is(err, model.ErrViewForUnknownEpoch) {
			return nil, fmt.Errorf("will not sign entity referencing view for unknown epoch: %v", err)
		}
		return nil, fmt.Errorf("could not get random beacon private key for view %d: %w", block.View, err)
	}

	// if the node is a Random Beacon participant and has succeeded DKG, then using the random beacon key
	// to sign the block
	beaconShare, err := beaconKey.Sign(msg, c.beaconHasher)
	if err != nil {
		return nil, fmt.Errorf("could not generate beacon signature: %w", err)
	}

	return msig.EncodeSingleSig(encoding.SigTypeRandomBeacon, beaconShare), nil
}
