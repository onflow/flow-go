package hotstuff

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Validator struct {
	// viewState provides API for querying identities for a specific view
	viewState ViewState
	// role specifies the role of the nodes
	role flow.Role
}

// QC is valid (signature is valid and has enough stake)
func (v *Validator) ValidateQC(qc *types.QuorumCertificate) error {
	// getting all consensus identities
	allStakedNode, err := v.viewState.GetIdentitiesForView(qc.View, v.role)
	if err != nil {
		return fmt.Errorf("cannot get identities to validate sig at view %v, because %w", qc.View, err)
	}

	hash := qc.BytesForSig()

	// validate signatures and get all signers
	signers, err := validateSignaturesForHash(allStakedNode, hash, qc.AggregatedSignature.Sigs())
	if err != nil {
		return fmt.Errorf("qc contains invalid signature: %w", err)
	}

	// compute total stakes of all signers from the QC
	totalStakes := computeTotalStakes(signers)

	// compute the threshold of stake required for a valid QC
	threshold := computeStakeThresholdForBuildingQC(allStakedNode)

	// check if there are enough stake for building QC
	if totalStakes < threshold {
		return fmt.Errorf("invalid QC: not enough stake, qc BlockMRH: %v", qc.BlockMRH)
	}
	return nil
}

// ValidateBlock validates the block proposal
// bp is the block proposal to be validated.
// parent is the parent of the block proposal.
func (v *Validator) ValidateBlock(bp *types.BlockProposal, parent *types.BlockProposal) error {
	// validate signature
	err := v.validateBlockSig(bp)
	if err != nil {
		return err
	}

	qc := bp.QC()

	// validate QC
	err = v.ValidateQC(qc)
	if err != nil {
		return err
	}

	// validate block hash
	if !bytes.Equal(qc.BlockMRH, parent.BlockMRH()) {
		return fmt.Errorf("invalid block. block must link to its parent. (qc, parent): (%v, %v)", qc.BlockMRH, parent.BlockMRH())
	}

	// validate height
	if bp.Height() != parent.Height()+1 {
		return fmt.Errorf("invalid block. block height must be 1 block higher than its parent. (block, parent): (%v, %v)", bp.Height(), parent.Height())
	}

	// validate view
	if bp.View() <= qc.View {
		return fmt.Errorf("invalid block. block's view must be higher than QC's view. (block, qc): (%v, %v)", bp.View(), qc.View)
	}
	return nil
}

// ValidateVote validates the vote
// vote is the vote to be validated
// bp is the voting block
func (v *Validator) ValidateVote(vote *types.Vote, bp *types.BlockProposal) error {
	// validate signature
	err := v.validateVoteSig(vote)
	if err != nil {
		return err
	}

	// view must match with the block's view
	if vote.View != bp.View() {
		return fmt.Errorf("invalid vote: wrong view number")
	}

	// block hash must match
	if !bytes.Equal(vote.BlockMRH, bp.BlockMRH()) {
		return fmt.Errorf("invalid vote: wrong block hash")
	}

	return nil
}

func validateSignaturesForHash(allStakedNode flow.IdentityList, hash []byte, sigs []*types.Signature) ([]*flow.Identity, error) {
	signers := make([]*flow.Identity, 0)
	for _, sig := range sigs {
		signer, err := validateSignatureForHash(allStakedNode, hash, sig)
		if err != nil {
			return nil, err
		}
		signers = append(signers, signer)
	}
	return signers, nil
}

func computeStakeThresholdForBuildingQC(identities flow.IdentityList) uint64 {
	// total * 2 / 3
	total := new(big.Int).SetUint64(identities.TotalStake())
	two := new(big.Int).SetUint64(2)
	three := new(big.Int).SetUint64(3)
	return new(big.Int).Div(
		new(big.Int).Mul(total, two),
		three).Uint64()
}

func computeTotalStakes(signers []*flow.Identity) uint64 {
	var total uint64
	for _, signer := range signers {
		total += signer.Stake
	}
	return total
}

func (v *Validator) validateVoteSig(vote *types.Vote) error {
	// getting all consensus identities
	identities, err := v.viewState.GetIdentitiesForView(vote.View, v.role)
	if err != nil {
		return fmt.Errorf("cannot get identities to validate sig at view %v, because %w", vote.View, err)
	}

	// get the hash
	hashToSign := vote.BytesForSig()

	// verify the signature
	_, err = validateSignatureForHash(identities, hashToSign, vote.Signature)
	return fmt.Errorf("invalid signature for vote %v, because %w", vote.BlockMRH, err)
}

func (v *Validator) validateBlockSig(bp *types.BlockProposal) error {
	// getting all consensus identities
	identities, err := v.viewState.GetIdentitiesForView(bp.View(), v.role)
	if err != nil {
		return fmt.Errorf("cannot get identities to validate sig at view %v, because %w", bp.View(), err)
	}

	// get the hash
	hashToSign := bp.BytesForSig()

	// verify the signature
	_, err = validateSignatureForHash(identities, hashToSign, bp.Signature)
	return fmt.Errorf("invalid signature for block %v, because %w", bp.BlockMRH(), err)
}

// validateSignatureForHash validates the signature and returns an identity if the sig is valid and the signer is staked.
func validateSignatureForHash(identities flow.IdentityList, hash []byte, sig *types.Signature) (*flow.Identity, error) {
	// getting signer's public key
	signer, err := findSignerByIndex(identities, sig.SignerIdx)
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

func findSignerByIndex(identities flow.IdentityList, idx uint32) (*flow.Identity, error) {
	// signer must exist
	if uint(idx) > identities.Count() {
		return nil, fmt.Errorf("signer not found by signerIdx: %v", idx)
	}

	signer := identities.Get(uint(idx))
	return signer, nil
}

// readPubKey returns the public key of an identity
func readPubKey(id *flow.Identity) [32]byte {
	// TODO: confirm that the NodeID is the public key
	return id.NodeID
}

// verify signature
func verifySignature(signature [32]byte, hashToSign []byte, pubkey [32]byte) error {
	panic("TODO")
}
