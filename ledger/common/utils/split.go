package utils

import (
	"github.com/onflow/flow-go/ledger"
)

// SplitByPath splits an slice of payloads based on the value of bit (bitIndex) of paths
func SplitByPath(paths []ledger.Path, payloads []ledger.Payload, bitIndex int) ([]ledger.Path, []ledger.Payload, []ledger.Path, []ledger.Payload) {
	rpaths := make([]ledger.Path, 0, len(paths)) // why len(paths)? is that to avoid reallocating memory in append?
	rpayloads := make([]ledger.Payload, 0, len(payloads))
	lpaths := make([]ledger.Path, 0, len(paths))
	lpayloads := make([]ledger.Payload, 0, len(payloads))

	for i, path := range paths { // TODO: if paths are sorted, binary search
		bit := Bit(path, bitIndex)
		if bit == 1 {
			rpaths = append(rpaths, path)
			rpayloads = append(rpayloads, payloads[i])
		} else {
			lpaths = append(lpaths, path)
			lpayloads = append(lpayloads, payloads[i])
		}
	}
	return lpaths, lpayloads, rpaths, rpayloads
}

// SplitSortedPaths splits a set of ordered paths based on the value of bit (bitIndex).
// Requirement: paths must be sorted in ascending order.
func SplitSortedPaths(paths []ledger.Path, bitIndex int) ([]ledger.Path, []ledger.Path) {
	// TODO: binary search
	for i, path := range paths {
		bit := Bit(path, bitIndex)
		// found the breaking point
		if bit == 1 {
			return paths[:i], paths[i:]
		}
	}
	// all paths have unset bit at bitIndex
	return paths, nil
}

// SplitTrieProofsByPath splits a set of unordered path and proof pairs based on the value of bit (bitIndex) of path
// Requirement: paths must be sorted in ascending order.
func SplitTrieProofsByPath(paths []ledger.Path, proofs []*ledger.TrieProof, bitIndex int) ([]ledger.Path, []*ledger.TrieProof, []ledger.Path, []*ledger.TrieProof) {
	rpaths := make([]ledger.Path, 0, len(paths))
	rproofs := make([]*ledger.TrieProof, 0, len(proofs))
	lpaths := make([]ledger.Path, 0, len(paths))
	lproofs := make([]*ledger.TrieProof, 0, len(proofs))

	// TODO: binary search
	for i, path := range paths {
		bit := Bit(path, bitIndex)
		if bit == 1 {
			rpaths = append(rpaths, path)
			rproofs = append(rproofs, proofs[i])
		} else {
			lpaths = append(lpaths, path)
			lproofs = append(lproofs, proofs[i])
		}
	}
	return lpaths, lproofs, rpaths, rproofs
}
