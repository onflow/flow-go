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
	allStakedNode, err := v.viewState.GetIdentitiesForBlockID(qc.BlockMRH)
	if err != nil {
		return fmt.Errorf("cannot get identities to validate sig at view %v, because %w", qc.View, err)
	}

	hash := qc.BytesForSig()

	// validate signatures. If valid, get back all signers
	signers, err := validateSignaturesForHash(allStakedNode, hash, qc.AggregatedSignature)
	if err != nil {
		return fmt.Errorf("qc contains invalid signature: %w", err)
	}

	// compute total stakes of all signers from the QC
	totalStakes := computeTotalStakes(signers)

	// compute the threshold of stake required for a valid QC
	threshold := ComputeStakeThresholdForBuildingQC(allStakedNode)

	// check if there are enough stake for building QC
	if totalStakes < threshold {
		return fmt.Errorf("invalid QC: not enough stake, qc BlockMRH: %v", qc.BlockMRH)
	}
	return nil
}

// ValidateBlock validates the block proposal
// bp - the block proposal to be validated.
// parent - the parent of the block proposal.
func (v *Validator) ValidateBlock(bp *types.BlockProposal, parent *types.BlockProposal) (*flow.Identity, error) {
	// validate signature
	signer, err := v.validateBlockSig(bp)
	if err != nil {
		return nil, err
	}

	// validate the signer is the leader for that block
	leaderID := v.viewState.LeaderForView(bp.View())
	if leaderID.ID() != signer.ID() {
		return nil, fmt.Errorf("invalid block. not from the leader: %v", bp.BlockMRH())
	}

	qc := bp.QC()

	// validate QC
	err = v.ValidateQC(qc)
	if err != nil {
		return nil, err
	}

	// validate block hash
	if qc.BlockMRH != parent.BlockMRH() {
		return nil, fmt.Errorf("invalid block. block must link to its parent. (qc, parent): (%v, %v)", qc.BlockMRH, parent.BlockMRH())
	}

	// validate height
	if bp.Height() != parent.Height()+1 {
		return nil, fmt.Errorf("invalid block. block height must be 1 block higher than its parent. (block, parent): (%v, %v)", bp.Height(), parent.Height())
	}

	// validate view
	if bp.View() <= qc.View {
		return nil, fmt.Errorf("invalid block. block's view must be higher than QC's view. (block, qc): (%v, %v)", bp.View(), qc.View)
	}
	return signer, nil
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

	// view must match with the block's view
	if vote.View != bp.View() {
		return nil, fmt.Errorf("invalid vote: wrong view number")
	}

	// block hash must match
	if vote.BlockMRH != bp.BlockMRH() {
		return nil, fmt.Errorf("invalid vote: wrong block hash")
	}

	return voter, nil
}

func validateSignaturesForHash(allStakedNode flow.IdentityList, hash []byte, aggsig *types.AggregatedSignature) ([]*flow.Identity, error) {
	signers := make([]*flow.Identity, 0)
	for signerIdx, signed := range aggsig.Signers {
		if signed {
			// read the signer identity
			signer, err := findSignerByIndex(allStakedNode, uint(signerIdx))
			if err != nil {
				return nil, err
			}

			// read the public key of the signer
			pubkey := readPubKey(signer)

			// verify if the signer's signature
			valid := aggsig.Verify(hash, pubkey)
			if !valid {
				return nil, fmt.Errorf("invalid aggregated signature. sig not match for pubkey: %v", pubkey)
			}

			// add valid signer to the list
			signers = append(signers, signer)
		}
	}
	return signers, nil
}

func computeTotalStakes(signers []*flow.Identity) uint64 {
	var total uint64
	for _, signer := range signers {
		total += signer.Stake
	}
	return total
}

func (v *Validator) validateVoteSig(vote *types.Vote) (*flow.Identity, error) {
	// getting all consensus identities
	identities, err := v.viewState.GetIdentitiesForBlockID(vote.BlockMRH)
	if err != nil {
		return nil, fmt.Errorf("cannot get identities to validate sig at view %v, because %w", vote.View, err)
	}

	// get the hash
	hashToSign := vote.BytesForSig()

	// verify the signature
	signer, err := validateSignatureForHash(identities, hashToSign, vote.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature for vote %v, because %w", vote.BlockMRH, err)
	}
	return signer, nil
}

func (v *Validator) validateBlockSig(bp *types.BlockProposal) (*flow.Identity, error) {
	// getting all consensus identities
	identities, err := v.viewState.GetIdentitiesForBlockID(bp.BlockMRH())
	if err != nil {
		return nil, fmt.Errorf("cannot get identities to validate sig at view %v, because %w", bp.View(), err)
	}

	// get the hash
	hashToSign := bp.BytesForSig()

	// verify the signature
	signer, err := validateSignatureForHash(identities, hashToSign, bp.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature for block %v, because %w", bp.BlockMRH(), err)
	}
	return signer, nil
}

// validateSignatureForHash validates the signature and returns an identity if the sig is valid and the signer is staked.
func validateSignatureForHash(identities flow.IdentityList, hash []byte, sig *types.Signature) (*flow.Identity, error) {
	// getting signer's public key
	signer, err := findSignerByIndex(identities, uint(sig.SignerIdx))
	if err != nil {
		return nil, fmt.Errorf("can not find signer, because %w", err)
	}

	signerPubKey := readPubKey(signer)

	// verify the signature
	err = verifySignature(sig.RawSignature, hash, signerPubKey)
	if err != nil {
		return nil, fmt.Errorf("invalid sig: %w", err)
	}

	// signer must be staked
	if signer.Stake == 0 {
		return nil, fmt.Errorf("signer has 0 stake, signer id: %v", signer.NodeID)
	}

	return signer, nil
}

func findSignerByIndex(identities flow.IdentityList, idx uint) (*flow.Identity, error) {
	// signer must exist
	if idx > identities.Count() {
		return nil, fmt.Errorf("signer not found by signerIdx: %v", idx)
	}

	signer := identities.Get(idx)
	return signer, nil
}

// readPubKey returns the public key of an identity
func readPubKey(id *flow.Identity) [32]byte {
	// TODO: confirm that the NodeID is the public key
	return id.NodeID
}

// verify the signature
func verifySignature(signature [32]byte, hashToSign []byte, pubkey [32]byte) error {
	panic("TODO")
}
