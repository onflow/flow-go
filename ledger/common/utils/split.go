package utils

import (
	"github.com/onflow/flow-go/ledger"
)

// SplitByPath permutes the input paths to be partitioned into 2 parts:
// * The first part contains all paths with bit-value 0 at position bitIndex;
// * the second part contains all paths with bit-value 1.
// The same permutation is applied to the payloads slice. Permutations are
// IN-PLACE.
// The returned pivot Index is the index of the _first_ element with
// bit-value 1. Therefore, all elements with bit-value 0 at position bitIndex
// can be obtained via `paths[:pivotIndex]` while all elements with
// bit-value 1 are in `paths[pivotIndex:]`
// For instance, if `paths` contains the following 3 paths, and bitIndex is `1`:
//     [[0,0,1,1], [0,1,0,1], [0,0,0,1]]
// then `SplitByPath` returns 1 and updates `paths` into:
//     [[0,0,1,1], [0,0,0,1], [0,1,0,1]]
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
func SplitByPath(paths []ledger.Path, payloads []ledger.Payload, bitIndex int) int {
	i := 0 // index of first element with bit-value 1
	for j, path := range paths {
		bit := Bit(path, bitIndex)
		if bit == 0 {
			paths[i], paths[j] = paths[j], paths[i]
			payloads[i], payloads[j] = payloads[j], payloads[i]
			i++
		}
	}
	return i
}

// SplitPaths permutes the input paths to be partitioned into 2 parts:
// * The first part contains all paths with bit-value 0 at position bitIndex;
// * the second part contains all paths with bit-value 1.
// Permutations are IN-PLACE.
// The returned pivot Index is the index of the _first_ element with
// bit-value 1. Therefore, all elements with bit-value 0 at position bitIndex
// can be obtained via `paths[:pivotIndex]` while all elements with
// bit-value 1 are in `paths[pivotIndex:]`
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
func SplitPaths(paths []ledger.Path, bitIndex int) int {
	i := 0 // index of first element with bit-value 1
	for j, path := range paths {
		bit := Bit(path, bitIndex)
		if bit == 0 {
			paths[i], paths[j] = paths[j], paths[i]
			i++
		}
	}
	return i
}

// SplitTrieProofsByPath permutes the input paths to be partitioned into 2 parts:
// * The first part contains all paths with bit-value 0 at position bitIndex;
// * the second part contains all paths with bit-value 1.
// The same permutation is applied to the proofs slice. Permutations are
// IN-PLACE.
// The returned pivot Index is the index of the _first_ element with
// bit-value 1. Therefore, all elements with bit-value 0 at position bitIndex
// can be obtained via `paths[:pivotIndex]` while all elements with
// bit-value 1 are in `paths[pivotIndex:]`
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
func SplitTrieProofsByPath(paths []ledger.Path, proofs []*ledger.TrieProof, bitIndex int) int {
	i := 0 // index of first element with bit-value 1
	for j, path := range paths {
		bit := Bit(path, bitIndex)
		if bit == 0 {
			paths[i], paths[j] = paths[j], paths[i]
			proofs[i], proofs[j] = proofs[j], proofs[i]
			i++
		}
	}
	return i
}
