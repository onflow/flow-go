package ledger

import (
	"errors"

	"github.com/dapperlabs/flow-go/storage/ledger/trie"
)

type TrieVerifier struct {
	height        int
	defaultHashes [256][]byte
}

// NewTrieVerifier creates a new trie-backed ledger verifier.
//
// The verifier is configured with a height and a default hash value for each level.
func NewTrieVerifier(height int, defaultHashes [256][]byte) *TrieVerifier {
	return &TrieVerifier{
		height:        height,
		defaultHashes: defaultHashes,
	}
}

// VerifyRegistersProof takes in an encoded proof along with registers, state, and values,
// and verifies if the proofs are correct
func (v *TrieVerifier) VerifyRegistersProof(
	registerIDs []RegisterID,
	stateCommitment StateCommitment,
	values []RegisterValue,
	proof []StorageProof,
) (verified bool, err error) {
	proofHldr := trie.DecodeProof(proof)
	length := proofHldr.GetSize()
	verified = true
	var verify bool
	for i := 0; i < length; i++ {
		flag, singleProof, inclusion, size := proofHldr.ExportProof(i)
		if inclusion {
			verify = trie.VerifyInclusionProof(registerIDs[i], values[i], flag, singleProof, size, stateCommitment, v.defaultHashes, v.height)
		} else {
			verify = trie.VerifyNonInclusionProof(registerIDs[i], values[i], flag, singleProof, size, stateCommitment, v.defaultHashes, v.height)
		}
		if !verify {
			return verify, errors.New("Incorrect Proof")
		}
	}

	return verified, nil
}
