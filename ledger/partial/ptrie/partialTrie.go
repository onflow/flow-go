package ptrie

import (
	"fmt"
	"math/bits"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/ledger/common/hash"
)

// countFlagBits returns the number of bits set in the first `steps` bits of
// `flags` (big-endian bit ordering, matching bitutils.ReadBit). It returns an
// error if `flags` does not contain enough bytes to cover `steps` bits.
func countFlagBits(flags []byte, steps uint8) (int, error) {
	if int(steps) > len(flags)*8 {
		return 0, fmt.Errorf("flags (%d bytes) only cover %d bits but proof has %d steps", len(flags), len(flags)*8, steps)
	}

	fullBytes := int(steps / 8)
	remainder := int(steps % 8)

	count := 0
	for i := range fullBytes {
		count += bits.OnesCount8(flags[i])
	}
	if remainder > 0 {
		// big-endian: the first `remainder` bits are the high bits of the byte.
		mask := byte(0xFF << (8 - remainder))
		count += bits.OnesCount8(flags[fullBytes] & mask)
	}
	return count, nil
}

// PSMT (Partial Sparse Merkle Tree) holds a subset of an sparse merkle tree at specific
// state (no historic views). Instead of keeping any unneeded branch, it only keeps
// the hash of subtree. This implementation is fully stored in memory and doesn't use
// a database.
//
// DEFINITIONS and CONVENTIONS:
//   - HEIGHT of a node v in a tree is the number of edges on the longest downward path
//     between v and a tree leaf. The height of a tree is the heights of its root.
//     The height of a Trie is always the height of the fully-expanded tree.
type PSMT struct {
	root       *node // Root
	pathLookUp map[ledger.Path]*node
}

// RootHash returns the rootNode hash value of the SMT
func (p *PSMT) RootHash() ledger.RootHash {
	return ledger.RootHash(p.root.Hash())
}

// GetSinglePayload returns payload of a given path
func (p *PSMT) GetSinglePayload(path ledger.Path) (*ledger.Payload, error) {
	node, found := p.pathLookUp[path]
	if !found {
		return nil, &ErrMissingPath{Paths: []ledger.Path{path}}
	}
	return node.payload, nil
}

// Get returns an slice of payloads (same order), an slice of failed paths and errors (if any)
// TODO return list of indecies instead of paths
func (p *PSMT) Get(paths []ledger.Path) ([]*ledger.Payload, error) {
	var failedPaths []ledger.Path
	payloads := make([]*ledger.Payload, len(paths))
	for i, path := range paths {
		// lookup the path for the payload
		node, found := p.pathLookUp[path]
		if !found {
			failedPaths = append(failedPaths, path)
			continue
		}
		payloads[i] = node.payload
	}
	if len(failedPaths) > 0 {
		return nil, &ErrMissingPath{Paths: failedPaths}
	}
	return payloads, nil
}

// Update updates registers and returns rootValue after updates
// in case of error, it returns a list of paths for which update failed
func (p *PSMT) Update(paths []ledger.Path, payloads []*ledger.Payload) (ledger.RootHash, error) {
	var failedPaths []ledger.Path
	for i, path := range paths {
		payload := payloads[i]
		// lookup the path and update the value
		node, found := p.pathLookUp[path]
		if !found {
			failedPaths = append(failedPaths, path)
			continue
		}
		node.hashValue = ledger.ComputeCompactValue(hash.Hash(path), payload.Value(), node.height)
	}
	if len(failedPaths) > 0 {
		return ledger.RootHash(hash.DummyHash), &ErrMissingPath{Paths: failedPaths}
	}
	// after updating all the nodes, compute the value recursively only once
	return ledger.RootHash(p.root.forceComputeHash()), nil
}

// NewPSMT builds a Partial Sparse Merkle Tree (PSMT) given a chunkdatapack registertouches
// TODO just accept batch proof as input
func NewPSMT(
	rootValue ledger.RootHash,
	batchProof *ledger.TrieBatchProof,
) (*PSMT, error) {
	height := ledger.NodeMaxHeight
	psmt := PSMT{newNode(ledger.GetDefaultHashForHeight(height), height), make(map[ledger.Path]*node)}

	// iterating over proofs for building the tree
	for i, pr := range batchProof.Proofs {
		if pr == nil {
			return nil, fmt.Errorf("proof at index %d is nil", i)
		}
		path := pr.Path
		payload := pr.Payload

		// Validate structural consistency of the proof before indexing into
		// Flags or Interims. A malformed proof (e.g. from a byzantine Execution
		// Node) must be rejected with an error instead of panicking.
		flagCount, err := countFlagBits(pr.Flags, pr.Steps)
		if err != nil {
			return nil, fmt.Errorf("proof at index %d has invalid flags: %w", i, err)
		}
		if flagCount != len(pr.Interims) {
			return nil, fmt.Errorf("proof at index %d has %d flag bits set but %d interims", i, flagCount, len(pr.Interims))
		}

		// we process the path, bit by bit, until we reach the end of the proof (due to compactness)
		prValueIndex := 0        // we keep track of our progress through proofs by prValueIndex
		currentNode := psmt.root // start from the rootNode and walk down the tree
		for j := 0; j < int(pr.Steps); j++ {
			// if a flag (bit j in flags) is false, the value is a default value
			// otherwise the value is stored in the proofs
			defaultHash := ledger.GetDefaultHashForHeight(currentNode.height - 1)
			v := defaultHash
			flag := bitutils.ReadBit(pr.Flags, j)
			if flag == 1 {
				// use the proof at index prValueIndex
				v = pr.Interims[prValueIndex]
				prValueIndex++
			}
			bit := bitutils.ReadBit(path[:], j)
			// look at the bit number j (left to right) for branching
			if bit == 1 { // right branching
				if currentNode.lChild == nil { // check left child
					currentNode.lChild = newNode(v, currentNode.height-1)
				}
				if currentNode.rChild == nil { // create the right child if not exist
					// Caution: we are temporarily initializing the node with default hash, which will later get updated to the
					// proper value (if this is an interim node, its hash will be set when computing the root hash of the PTrie
					// in the end; if this is a leaf, we'll set the hash at the end of processing the proof)
					currentNode.rChild = newNode(defaultHash, currentNode.height-1)
				}
				currentNode = currentNode.rChild
			} else { // left branching
				if currentNode.rChild == nil { // check right child
					currentNode.rChild = newNode(v, currentNode.height-1)
				}
				if currentNode.lChild == nil { // create the left child if not exist
					// Caution: we are temporarily initializing the node with default hash, which will later get updated to the
					// proper value (if this is an interim node, its hash will be set when computing the root hash of the PTrie
					// in the end; if this is a leaf, we'll set the hash at the end of processing the proof)
					currentNode.lChild = newNode(defaultHash, currentNode.height-1)
				}
				currentNode = currentNode.lChild
			}
		}

		currentNode.payload = payload
		// update node's hash value only for inclusion proofs (for others we assume default value)
		if pr.Inclusion {
			currentNode.hashValue = ledger.ComputeCompactValue(hash.Hash(path), payload.Value(), currentNode.height)
		}
		// keep a reference to this node by path (for update purpose)
		psmt.pathLookUp[path] = currentNode
	}

	// check if the rootHash matches the root node's hash value of the partial trie
	if ledger.RootHash(psmt.root.forceComputeHash()) != rootValue {
		return nil, fmt.Errorf("rootNode hash doesn't match the proofs expected [%x], got [%x]", psmt.root.Hash(), rootValue)
	}
	return &psmt, nil
}
