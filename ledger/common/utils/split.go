package utils

import (
	"github.com/onflow/flow-go/ledger"
)

// SplitByPath permutes the input paths to be partitioned into 2 parts. The first part contains paths with a zero bit
// at the input bitIndex, the second part contains paths with a one at the bitIndex. The index of partition
// is returned.
// The same permutation is applied to the payloads slice.
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
//
//  For instance, if `paths` contains the following 3 paths, and bitIndex is `1`:
//  [[0,0,1,1], [0,1,0,1], [0,0,0,1]]
//  then `SplitByPath` returns 1 and updates `paths` into:
//  [[0,0,1,1], [0,0,0,1], [0,1,0,1]]
func SplitByPath(paths []ledger.Path, payloads []ledger.Payload, bitIndex int) int {
	i := 0
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

// SplitPaths permutes the input paths to be partitioned into 2 parts. The first part contains paths with a zero bit
// at the input bitIndex, the second part contains paths with a one at the bitIndex. The index of partition
// is returned.
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
func SplitPaths(paths []ledger.Path, bitIndex int) int {
	i := 0
	for j, path := range paths {
		bit := Bit(path, bitIndex)
		if bit == 0 {
			paths[i], paths[j] = paths[j], paths[i]
			i++
		}
	}
	return i
}

// SplitByPath permutes the input paths to be partitioned into 2 parts. The first part contains paths with a zero bit
// at the input bitIndex, the second part contains paths with a one at the bitIndex. The index of partition
// is returned.
// The same permutation is applied to the proofs slice.
//
// This would be the partition step of an ascending quick sort of paths (lexicographic order)
// with the pivot being the path with all zeros and 1 at bitIndex.
// The comparison of paths is only based on the bit at bitIndex, the function therefore assumes all paths have
// equal bits from 0 to bitIndex-1
func SplitTrieProofsByPath(paths []ledger.Path, proofs []*ledger.TrieProof, bitIndex int) int {
	i := 0
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
