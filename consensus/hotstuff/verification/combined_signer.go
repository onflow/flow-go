package verification

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/dkg"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// CombinedSigner is a signer capable of creating two signatures for each signing
// operation, combining them and adding them as the signature data on the generated
// data structures. The first type is an aggregating signer, which creates single
// signatures that can be aggregated into a single aggregated signature. The second
// type is a threshold signer, which creates threshold signature shares, which can
// be used to reconstruct a threshold signature.
type CombinedSigner struct {
	*CombinedVerifier
	state    protocol.State
	dkg      dkg.State
	staking  module.AggregatingSigner
	beacon   module.ThresholdSigner
	merger   module.Merger
	selector flow.IdentityFilter
	signerID flow.Identifier
}

// NewCombinedSigner creates a new combined signer with the given dependencies:
// - the protocol state is used to retrieve public keys for signers;
// - the selector is used to select the set of valid signers from the protocol state;
// - the signer ID is used as the identity when creating signatures;
// - the staking signer is used to create aggregatable signatures for the first signature part;
// - the threshold signer is used to create threshold signture shres for the second signature part;
// - the merger is used to join and split the two signature parts on our models;
// - the selector is used to select the set of scheme participants from the protocol state.
func NewCombinedSigner(state protocol.State, dkg dkg.State, staking module.AggregatingSigner, beacon module.ThresholdSigner, merger module.Merger, selector flow.IdentityFilter, signerID flow.Identifier) *CombinedSigner {
	sc := &CombinedSigner{
		CombinedVerifier: NewCombinedVerifier(state, dkg, staking, beacon, merger, selector),
		state:            state,
		dkg:              dkg,
		staking:          staking,
		beacon:           beacon,
		merger:           merger,
		signerID:         signerID,
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
func (c *CombinedSigner) CreateQC(votes []*model.Vote) (*model.QuorumCertificate, error) {

	// check the consistency of the votes
	err := checkVotesValidity(votes)
	if err != nil {
		return nil, fmt.Errorf("votes are not valid: %w", err)
	}

	// get the DKG group size
	groupSize, err := c.dkg.GroupSize()
	if err != nil {
		return nil, fmt.Errorf("could not get DKG group size: %w", err)
	}

	// check if we have sufficient threshold signature shares
	if !crypto.EnoughShares(int(groupSize), len(votes)) {
		return nil, ErrInsufficientShares
	}

	// collect signers, staking signatures, beacon signatures and dkg indices separately
	signerIDs := make([]flow.Identifier, 0, len(votes))
	stakingSigs := make([]crypto.Signature, 0, len(votes))
	beaconShares := make([]crypto.Signature, 0, len(votes))
	dkgIndices := make([]uint, 0, len(votes))
	for _, vote := range votes {

		// split the vote signature into its parts
		splitSigs, err := c.merger.Split(vote.SigData)
		if err != nil {
			return nil, fmt.Errorf("could not split signature (voter: %x): %w", vote.SignerID, err)
		}

		// check that we have two parts (staking & beacon)
		if len(splitSigs) != 2 {
			return nil, fmt.Errorf("wrong amount of split signatures (voter: %x, count: %d, expected: 2)", vote.SignerID, len(splitSigs))
		}

		// assign the respective parts to meaningful names
		stakingSig := splitSigs[0]
		beaconShare := splitSigs[1]

		// get the dkg index from the dkg state
		dkgIndex, err := c.dkg.ParticipantIndex(vote.SignerID)
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

	// construct the threshold signature from the shares
	beaconThresSig, err := c.beacon.Combine(groupSize, beaconShares, dkgIndices)
	if err != nil {
		return nil, fmt.Errorf("could not aggregate second signatures: %w", err)
	}

	// TODO: once true BLS signature aggregation has been implemented, the performance
	// impact of verifying the aggregated signature and the threshold signature should
	// be minor and we can consider adding a sanity check

	// combine the aggregated staking signature with the threshold beacon signature
	combinedMultiSig, err := c.merger.Join(stakingAggSig, beaconThresSig)
	if err != nil {
		return nil, fmt.Errorf("could not combine the aggregated signatures: %w", err)
	}

	// create the QC
	qc := &model.QuorumCertificate{
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
	msg := makeVoteMessage(block.View, block.BlockID)
	stakingSig, err := c.staking.Sign(msg)
	if err != nil {
		return nil, fmt.Errorf("could not generate first signature: %w", err)
	}
	beaconShare, err := c.beacon.Sign(msg)
	if err != nil {
		return nil, fmt.Errorf("could not generate second signature: %w", err)
	}

	// combine the two signatures into one byte slice
	combinedSig, err := c.merger.Join(stakingSig, beaconShare)
	if err != nil {
		return nil, fmt.Errorf("could not combine signatures: %w", err)
	}

	return combinedSig, nil
}
