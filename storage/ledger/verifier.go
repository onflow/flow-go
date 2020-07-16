package ledger

import (
	"github.com/dapperlabs/flow-go/model/flow"
	proofs "github.com/dapperlabs/flow-go/storage/ledger/mtrie/proof"
)

type TrieVerifier struct {
	keyByteSize int
}

// NewTrieVerifier creates a new trie-backed ledger verifier.
//
// The verifier is configured with a height and a default hash value for each level.
func NewTrieVerifier(keyByteSize int) *TrieVerifier {
	return &TrieVerifier{
		keyByteSize: keyByteSize,
	}
}

// VerifyRegistersProof takes in an encoded proof along with registers, state, and values,
// and verifies if the proofs are correct
func (v *TrieVerifier) VerifyRegistersProof(
	registerIDs []flow.RegisterID,
	values []flow.RegisterValue,
	proof []flow.StorageProof,
	stateCommitment flow.StateCommitment,
) (verified bool, err error) {
	bp, err := proofs.DecodeBatchProof(proof)
	if err != nil {
		return false, err
	}
	return bp.Verify(registerIDs, values, stateCommitment, v.keyByteSize), nil
}
