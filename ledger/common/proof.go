package common

import (
	"bytes"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/utils"
)

// VerifyProof verifies the proof, by constructing all the
// hash from the leaf to the root and comparing the rootHash
func VerifyProof(p *ledger.Proof, expectedStateCommitment ledger.StateCommitment, expectedKeySize int) bool {
	treeHeight := 8 * expectedKeySize
	leafHeight := treeHeight - int(p.Steps)             // p.Steps is the number of edges we are traversing until we hit the compactified leaf.
	if !(0 <= leafHeight && leafHeight <= treeHeight) { // sanity check
		return false
	}
	// We start with the leaf and hash our way upwards towards the root
	proofIndex := len(p.Interims) - 1                              // the index of the last non-default value furthest down the tree (-1 if there is none)
	computed := ComputeCompactValue(p.Path, p.Payload, leafHeight) // we first compute the hash of the fully-expanded leaf (at height 0)
	for h := leafHeight + 1; h <= treeHeight; h++ {                // then, we hash our way upwards until we hit the root (at height `treeHeight`)
		// we are currently at a node n (initially the leaf). In this iteration, we want to compute the
		// parent's hash. Here, h is the height of the parent, whose hash want to compute.
		// The parent has two children: child n, whose hash we have already computed (aka `computed`);
		// and the sibling to node n, whose hash (aka `siblingHash`) must be defined by the Proof.

		var siblingHash []byte
		if utils.IsBitSet(p.Flags, treeHeight-h) { // if flag is set, siblingHash is stored in the proof
			if proofIndex < 0 { // proof invalid: too few values
				return false
			}
			siblingHash = p.Interims[proofIndex]
			proofIndex--
		} else { // otherwise, siblingHash is a default hash
			siblingHash = GetDefaultHashForHeight(h - 1)
		}
		// hashing is order dependant
		if utils.IsBitSet(p.Path, treeHeight-h) { // we hash our way up to the parent along the parent's right branch
			computed = HashInterNode(siblingHash, computed)
		} else { // we hash our way up to the parent along the parent's left branch
			computed = HashInterNode(computed, siblingHash)
		}
	}
	return bytes.Equal(computed, expectedStateCommitment) == p.Inclusion
}

// VerifyBatchProof verifies all the proof inside the batchproof
func VerifyBatchProof(bp *ledger.BatchProof, expectedRootHash []byte, expectedKeySize int) bool {
	for _, p := range bp.Proofs {
		// any invalid proof
		if !VerifyProof(p, expectedRootHash, expectedKeySize) {
			return false
		}
	}
	return true
}
