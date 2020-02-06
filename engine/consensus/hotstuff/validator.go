package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Validator provides functions to validate QC, Block and Vote
type Validator struct {
	// viewState provides API for querying identities for a specific view
	viewState *ViewState
}

// ValidateQC validates the QC
// It doesn't validate the block that this QC is pointing to
func (v *Validator) ValidateQC(qc *types.QuorumCertificate) error {
	// TODO: potentially can return a very long list. need to find a better way for verifying QC
	stakedNodes, err := v.viewState.GetIdentitiesForBlockID(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot get identities to validate sig at view %v: %w", qc.View, err)
	}

	signedBytes := qc.BytesForSig()

	// validate signatures. If valid, get back all signers
	signers, err := validateAggregatedSigWithSignedBytes(stakedNodes, signedBytes, qc.AggregatedSignature)
	if err != nil {
		return fmt.Errorf("qc contains invalid signature: %w", err)
	}

	// compute total stakes of all signers from the QC
	totalStakes := flow.IdentityList(signers).TotalStake()

	// compute the threshold of stake required for a valid QC
	threshold := ComputeStakeThresholdForBuildingQC(stakedNodes)

	// check if there are enough stake for building QC
	if totalStakes < threshold {
		return fmt.Errorf("invalid QC: not enough stake, qc BlockID: %v", qc.BlockID)
	}
	return nil
}

// ValidateBlock validates the block header, returns the signer and a validated block proposal
// bp - the block header to be validated.
// parent - the parent of the block proposal.
func (v *Validator) ValidateBlock(bh *types.BlockHeader, parent *types.BlockProposal) (*flow.Identity, *types.BlockProposal, error) {
	// validate signature
	signer, err := v.validateBlockSig(bh)
	if err != nil {
		return nil, nil, err
	}

	// validate the signer is the leader for that block
	leaderID := v.viewState.LeaderForView(bh.View())
	if leaderID.ID() != signer.ID() {
		return nil, nil, fmt.Errorf("invalid block. not from the leader: %v", bh.BlockID())
	}

	qc := bh.QC()

	// validate QC
	err = v.ValidateQC(qc)
	if err != nil {
		return nil, nil, err
	}

	// validate block hash
	if qc.BlockID != parent.BlockID() {
		return nil, nil, fmt.Errorf("invalid block. block must link to its parent. (qc: %x, parent: %x)", qc.BlockID, parent.BlockID())
	}

	// validate height
	if bh.Height() != parent.Height()+1 {
		return nil, nil, fmt.Errorf("invalid block. block height must be 1 block higher than its parent. (block: %v, parent: %v)", bh.Height(), parent.Height())
	}

	// validate view
	if bh.View() <= qc.View {
		return nil, nil, fmt.Errorf("invalid block. block's view must be higher than QC's view. (block: %v, qc: %v)", bh.View(), qc.View)
	}

	// only all the validation has passed, a block proposal will be constructed
	bp := toBlockProposal(bh)

	return signer, bp, nil
}

// ValidateVote validates the vote and returns the signer identity who signed the vote
// vote - the vote to be validated
// bp - the voting block
func (v *Validator) ValidateVote(vote *types.Vote, bp *types.BlockProposal) (*flow.Identity, error) {
	// validate signature
	voter, err := v.validateVoteSig(vote)
	if err != nil {
		return nil, err
	}
	// the block is missing
	if bp == nil {
		return voter, nil
	}
	// view must match with the block's view
	if vote.View != bp.View() {
		return nil, fmt.Errorf("invalid vote: wrong view number")
	}

	// block hash must match
	if vote.BlockID != bp.BlockID() {
		return nil, fmt.Errorf("invalid vote: wrong block hash")
	}

	return voter, nil
}

func validateAggregatedSigWithSignedBytes(stakedNodes flow.IdentityList, signedBytes []byte, aggsig *types.AggregatedSignature) ([]*flow.Identity, error) {
	signers := make([]*flow.Identity, 0)
	for signerIdx, signed := range aggsig.Signers {
		if signed {
			// read the signer identity
			if uint(signerIdx) > stakedNodes.Count() {
				return nil, fmt.Errorf("signer not found by signerIdx: %v", signerIdx)
			}

			signer := stakedNodes.Get(uint(signerIdx))

			// read the public key of the signer
			pubkey := readPubKey(signer)

			// verify if the signer's signature
			valid := aggsig.Verify(signedBytes, pubkey)
			if !valid {
				return nil, fmt.Errorf("invalid aggregated signature. sig not match for pubkey: %v", pubkey)
			}

			// add valid signer to the list
			signers = append(signers, signer)
		}
	}
	return signers, nil
}

func (v *Validator) validateVoteSig(vote *types.Vote) (*flow.Identity, error) {
	// getting all consensus identities
	identities, err := v.viewState.GetIdentitiesForBlockID(vote.BlockID)
	if err != nil {
		return nil, fmt.Errorf("cannot get identities to validate sig at view %v: %w", vote.View, err)
	}

	// get the signed bytes
	bytesToSign := vote.BytesForSig()

	// verify the signature
	signer, err := validateSignatureWithSignedBytes(identities, bytesToSign, vote.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature for vote: %v: %w", vote.BlockID, err)
	}
	return signer, nil
}

func (v *Validator) validateBlockSig(bh *types.BlockHeader) (*flow.Identity, error) {
	// getting all consensus identities
	identities, err := v.viewState.GetIdentitiesForBlockID(bh.BlockID())
	if err != nil {
		return nil, fmt.Errorf("cannot get identities to validate sig at view %v: %w", bh.View(), err)
	}

	// get the hash
	hashToSign := bh.ToVote().BytesForSig()

	// verify the signature
	signer, err := validateSignatureWithSignedBytes(identities, hashToSign, bh.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature for block %v: %w", bh.BlockID(), err)
	}
	return signer, nil
}

func toBlockProposal(bh *types.BlockHeader) *types.BlockProposal {
	return types.NewBlockProposal(bh.Block, bh.Signature)
}

// validateSignatureWithSignedBytes validates the signature and returns
// an identity if the sig is valid and the signer is staked.
func validateSignatureWithSignedBytes(identities flow.IdentityList, hash []byte, sig *types.Signature) (*flow.Identity, error) {
	// getting signer's public key
	if uint(sig.SignerIdx) > identities.Count() {
		return nil, fmt.Errorf("signer not found by signerIdx: %v", sig.SignerIdx)
	}
	signer := identities.Get(uint(sig.SignerIdx))
	signerPubKey := readPubKey(signer)

	// verify the signature
	err := verifySignature(sig.RawSignature, hash, signerPubKey)
	if err != nil {
		return nil, fmt.Errorf("invalid sig: %w", err)
	}

	// signer must be staked
	if signer.Stake == 0 {
		return nil, fmt.Errorf("signer has 0 stake, signer id: %v", signer.NodeID)
	}

	return signer, nil
}

// readPubKey returns the public key of an identity
func readPubKey(id *flow.Identity) [32]byte {
	// TODO: confirm that the NodeID is the public key
	return id.NodeID
}

// verify the signature
func verifySignature(signature [32]byte, hashToSign []byte, pubkey [32]byte) error {
	// TODO
	return nil
}
