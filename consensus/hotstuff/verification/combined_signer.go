package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/signature"
)

// CombinedSigner is a signer capable of creating two signatures for each signing
// operation, combining them and adding them as the signature data on the generated
// data structures. The first type is an aggregating signer, which creates single
// signatures that can be aggregated into a single aggregated signature. The second
// type is a threshold signer, which creates threshold signature shares, which can
// be used to reconstruct a threshold signature.
type CombinedSigner struct {
	*CombinedVerifier
	staking              module.AggregatingSigner
	merger               module.Merger
	thresholdSignerStore module.ThresholdSignerStore
	signerID             flow.Identifier
}

// NewCombinedSigner creates a new combined signer with the given dependencies:
// - the hotstuff committee's state is used to retrieve public keys for signers;
// - the staking signer is used to create and verify aggregatable signatures for the first signature part;
// - the thresholdVerifier is used to verify threshold signatures
// - the merger is used to join and split the two signature parts on our models;
// - the thresholdSignerStore is used to get threshold-signers by epoch/view;
// - the signer ID is used as the identity when creating signatures;
func NewCombinedSigner(
	committee hotstuff.Committee,
	staking module.AggregatingSigner,
	thresholdVerifier module.ThresholdVerifier,
	merger module.Merger,
	thresholdSignerStore module.ThresholdSignerStore,
	signerID flow.Identifier) *CombinedSigner {

	sc := &CombinedSigner{
		CombinedVerifier:     NewCombinedVerifier(committee, staking, thresholdVerifier, merger),
		staking:              staking,
		merger:               merger,
		thresholdSignerStore: thresholdSignerStore,
		signerID:             signerID,
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

// CreateQC will create a quorum certificate with a combined aggregated signature and
// threshold signature for the given votes.
func (c *CombinedSigner) CreateQC(votes []*model.Vote) (*flow.QuorumCertificate, error) {

	// check the consistency of the votes
	// TODO: is checking the view and block id needed? (single votes are supposed to be already checked)
	err := checkVotesValidity(votes)
	if err != nil {
		return nil, fmt.Errorf("votes are not valid: %w", err)
	}

	// get the DKG group size
	blockID := votes[0].BlockID
	view := votes[0].View
	dkg, err := c.committee.DKG(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get DKG: %w", err)
	}

	// check if we have sufficient threshold signature shares
	enoughShares, err := signature.EnoughThresholdShares(int(dkg.Size()), len(votes))
	if err != nil {
		return nil, fmt.Errorf("failed to check if shares are enough: %w", err)
	}
	if !enoughShares {
		return nil, signature.ErrInsufficientShares
	}

	// collect signers, staking signatures, beacon signatures and dkg indices separately
	signerIDs := make([]flow.Identifier, 0, len(votes))
	stakingSigs := make([]crypto.Signature, 0, len(votes))
	beaconShares := make([]crypto.Signature, 0, len(votes))
	dkgIndices := make([]uint, 0, len(votes))
	for _, vote := range votes {

		// split the vote signature into its parts
		stakingSig, beaconShare, err := c.merger.Split(vote.SigData)
		if err != nil {
			return nil, fmt.Errorf("could not split signature (voter: %x): %w", vote.SignerID, err)
		}

		// get the dkg index from the dkg state
		dkgIndex, err := dkg.Index(vote.SignerID)
		if err != nil {
			return nil, fmt.Errorf("could not get dkg index (signer: %x): %w", vote.SignerID, err)
		}

		// collect each element in its respective slice
		signerIDs = append(signerIDs, vote.SignerID)
		stakingSigs = append(stakingSigs, stakingSig)
		beaconShares = append(beaconShares, beaconShare)
		dkgIndices = append(dkgIndices, dkgIndex)
	}

	// aggregate all staking signatures into one aggregated signature
	stakingAggSig, err := c.staking.Aggregate(stakingSigs)
	if err != nil {
		return nil, fmt.Errorf("could not aggregate staking signatures: %w", err)
	}

	beaconSigner, err := c.thresholdSignerStore.GetThresholdSigner(view)
	if err != nil {
		return nil, fmt.Errorf("could not get threshold signer for view (%d): %w", view, err)
	}
	// construct the threshold signature from the shares
	beaconThresSig, err := beaconSigner.Reconstruct(dkg.Size(), beaconShares, dkgIndices)
	if err != nil {
		return nil, fmt.Errorf("could not reconstruct beacon signatures: %w", err)
	}

	// combine the aggregated staking signature with the threshold beacon signature
	combinedMultiSig, err := c.merger.Join(stakingAggSig, beaconThresSig)
	if err != nil {
		return nil, fmt.Errorf("could not join signatures: %w", err)
	}

	// create the QC
	qc := &flow.QuorumCertificate{
		View:      votes[0].View,
		BlockID:   votes[0].BlockID,
		SignerIDs: signerIDs,
		SigData:   combinedMultiSig,
	}

	return qc, nil
}

// genSigData generates the signature data for our local node for the given block.
func (c *CombinedSigner) genSigData(block *model.Block) ([]byte, error) {

	// create the message to be signed and generate signatures
	msg := MakeVoteMessage(block.View, block.BlockID)
	stakingSig, err := c.staking.Sign(msg)
	if err != nil {
		return nil, fmt.Errorf("could not generate staking signature: %w", err)
	}

	beacon, err := c.thresholdSignerStore.GetThresholdSigner(block.View)
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
