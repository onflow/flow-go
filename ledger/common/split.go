package common

import (
	"fmt"

	"github.com/dapperlabs/flow-go/ledger"
)

// SplitByPath splits an slice of payloads based on the value of bit (bitIndex) of paths
// TODO: remove error return
func SplitByPath(paths []ledger.Path, payloads []ledger.Payload, bitIndex int) ([]ledger.Path, []ledger.Payload, []ledger.Path, []ledger.Payload, error) {
	rpaths := make([]ledger.Path, 0, len(paths))
	rpayloads := make([]ledger.Payload, 0, len(payloads))
	lpaths := make([]ledger.Path, 0, len(paths))
	lpayloads := make([]ledger.Payload, 0, len(payloads))

	for i, path := range paths {
		bitIsSet, err := IsBitSet(path, bitIndex)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("can't split payloads, error: %v", err)
		}
		if bitIsSet {
			rpaths = append(rpaths, path)
			rpayloads = append(rpayloads, payloads[i])
		} else {
			lpaths = append(lpaths, path)
			lpayloads = append(lpayloads, payloads[i])
		}
	}
	return lpaths, lpayloads, rpaths, rpayloads, nil
}

// SplitSortedPaths splits a set of ordered paths based on the value of bit (bitIndex)
func SplitSortedPaths(paths []ledger.Path, bitIndex int) ([]ledger.Path, []ledger.Path, error) {
	for i, path := range paths {
		bitIsSet, err := IsBitSet(path, bitIndex)
		if err != nil {
			return nil, nil, fmt.Errorf("can't split paths, error: %v", err)
		}
		// found the breaking point
		if bitIsSet {
			return paths[:i], paths[i:], nil
		}
	}
	// all paths have unset bit at bitIndex
	return paths, nil, nil
}

// SplitTrieProofsByPath splits a set of unordered path and proof pairs based on the value of bit (bitIndex) of path
func SplitTrieProofsByPath(paths []ledger.Path, proofs []*ledger.TrieProof, bitIndex int) ([]ledger.Path, []*ledger.TrieProof, []ledger.Path, []*ledger.TrieProof, error) {
	rpaths := make([]ledger.Path, 0, len(paths))
	rproofs := make([]*ledger.TrieProof, 0, len(proofs))
	lpaths := make([]ledger.Path, 0, len(paths))
	lproofs := make([]*ledger.TrieProof, 0, len(proofs))

	for i, path := range paths {
		bitIsSet, err := IsBitSet(path, bitIndex)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("can't split key proof pairs , error: %v", err)
		}
		if bitIsSet {
			rpaths = append(rpaths, path)
			rproofs = append(rproofs, proofs[i])
		} else {
			lpaths = append(lpaths, path)
			lproofs = append(lproofs, proofs[i])
		}
	}
	return lpaths, lproofs, rpaths, rproofs, nil
}
