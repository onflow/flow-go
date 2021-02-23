package common

import (
	"bytes"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hasher"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// TODO move this to proof itself

// VerifyTrieProof verifies the proof, by constructing all the
// hash from the leaf to the root and comparing the rootHash
func VerifyTrieProof(p *ledger.TrieProof, expectedState ledger.State) bool {

	ledgerHasher := hasher.NewLedgerHasher(hasher.DefaultHasherVersion)

	treeHeight := 8 * len(p.Path)
	leafHeight := treeHeight - int(p.Steps)             // p.Steps is the number of edges we are traversing until we hit the compactified leaf.
	if !(0 <= leafHeight && leafHeight <= treeHeight) { // sanity check
		return false
	}
	// We start with the leaf and hash our way upwards towards the root
	proofIndex := len(p.Interims) - 1                                           // the index of the last non-default value furthest down the tree (-1 if there is none)
	computed := ledgerHasher.ComputeCompactValue(p.Path, p.Payload, leafHeight) // we first compute the hash of the fully-expanded leaf (at height 0)
	for h := leafHeight + 1; h <= treeHeight; h++ {                             // then, we hash our way upwards until we hit the root (at height `treeHeight`)
		// we are currently at a node n (initially the leaf). In this iteration, we want to compute the
		// parent's hash. Here, h is the height of the parent, whose hash want to compute.
		// The parent has two children: child n, whose hash we have already computed (aka `computed`);
		// and the sibling to node n, whose hash (aka `siblingHash`) must be defined by the Proof.

		var siblingHash []byte
		flagIsSet, err := utils.IsBitSet(p.Flags, treeHeight-h)
		if err != nil {
			return false
		}
		if flagIsSet { // if flag is set, siblingHash is stored in the proof
			if proofIndex < 0 { // proof invalid: too few values
				return false
			}
			siblingHash = p.Interims[proofIndex]
			proofIndex--
		} else { // otherwise, siblingHash is a default hash
			siblingHash = ledgerHasher.GetDefaultHashForHeight(h - 1)
		}

		bitIsSet, err := utils.IsBitSet(p.Path, treeHeight-h)
		if err != nil {
			return false
		}
		// hashing is order dependant
		if bitIsSet { // we hash our way up to the parent along the parent's right branch
			computed = ledgerHasher.HashInterNode(siblingHash, computed)
		} else { // we hash our way up to the parent along the parent's left branch
			computed = ledgerHasher.HashInterNode(computed, siblingHash)
		}
	}
	return bytes.Equal(computed, expectedState) == p.Inclusion
}

// VerifyTrieBatchProof verifies all the proof inside the batchproof
func VerifyTrieBatchProof(bp *ledger.TrieBatchProof, expectedState ledger.State) bool {
	for _, p := range bp.Proofs {
		// any invalid proof
		if !VerifyTrieProof(p, expectedState) {
			return false
		}
	}
	return true
}
