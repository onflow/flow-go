package ledger

import (
	"errors"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
)

type TrieVerifier struct {
	trieHeight int
}

// NewTrieVerifier creates a new trie-backed ledger verifier.
//
// The verifier is configured with a height and a default hash value for each level.
func NewTrieVerifier(trieHeight int) *TrieVerifier {
	return &TrieVerifier{
		trieHeight: trieHeight,
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
	proofHldr, err := trie.DecodeProof(proof)
	if err != nil {
		return false, err
	}
	length := proofHldr.GetSize()
	verified = true
	var verify bool
	for i := 0; i < length; i++ {
		flag, singleProof, inclusion, size := proofHldr.ExportProof(i)
		if inclusion {
			verify = trie.VerifyInclusionProof(registerIDs[i], values[i], flag, singleProof, size, stateCommitment, v.trieHeight)
		} else {
			verify = trie.VerifyNonInclusionProof(registerIDs[i], values[i], flag, singleProof, size, stateCommitment, v.trieHeight)
		}
		if !verify {
			return verify, errors.New("Incorrect Proof")
		}
	}

	return verified, nil
}
