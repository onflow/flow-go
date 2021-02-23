package ptrie

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hasher"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// PSMT (Partial Sparse Merkle Tree) holds a subset of an sparse merkle tree at specific
// state (no historic views). Instead of keeping any unneeded branch, it only keeps
// the hash of subtree. This implementation is fully stored in memory and doesn't use
// a database.
//
// DEFINITIONS and CONVENTIONS:
//   * HEIGHT of a node v in a tree is the number of edges on the longest downward path
//     between v and a tree leaf. The height of a tree is the heights of its root.
//     The height of a Trie is always the height of the fully-expanded tree.
type PSMT struct {
	root         *node // Root
	pathByteSize int   // expected size [bytes] of path
	pathLookUp   map[string]*node
	ledgerHasher *hasher.LedgerHasher
}

// PathSize returns the expected expected size [bytes] of path
func (p *PSMT) PathSize() int {
	return p.pathByteSize
}

// RootHash returns the rootNode hash value of the SMT
func (p *PSMT) RootHash() []byte {
	return p.root.HashValue(p.ledgerHasher)
}

// Get returns an slice of payloads (same order), an slice of failed paths and errors (if any)
// TODO return list of indecies instead of paths
func (p *PSMT) Get(paths []ledger.Path) ([]*ledger.Payload, error) {
	var failedPaths []ledger.Path
	payloads := make([]*ledger.Payload, 0)
	for _, path := range paths {
		// lookup the path for the payload
		node, found := p.pathLookUp[string(path)]
		if !found {
			payloads = append(payloads, nil)
			failedPaths = append(failedPaths, path)
			continue
		}
		payloads = append(payloads, node.payload)
	}
	if len(failedPaths) > 0 {
		return nil, &ErrMissingPath{Paths: failedPaths}
	}
	// after updating all the nodes, compute the value recursively only once
	return payloads, nil
}

// Update updates registers and returns rootValue after updates
// in case of error, it returns a list of paths for which update failed
func (p *PSMT) Update(paths []ledger.Path, payloads []*ledger.Payload) ([]byte, error) {
	var failedKeys []ledger.Key
	for i, path := range paths {
		payload := payloads[i]
		// lookup the path and update the value
		node, found := p.pathLookUp[string(path)]
		if !found {
			failedKeys = append(failedKeys, payload.Key)
			continue
		}
		node.hashValue = p.ledgerHasher.ComputeCompactValue(path, payload, node.height)
	}
	if len(failedKeys) > 0 {
		return nil, &ledger.ErrMissingKeys{Keys: failedKeys}
	}
	// after updating all the nodes, compute the value recursively only once
	return p.root.HashValue(p.ledgerHasher), nil
}

// NewPSMT builds a Partial Sparse Merkle Tree (PMST) given a chunkdatapack registertouches
// TODO just accept batch proof as input
func NewPSMT(
	rootValue []byte, // rootHash
	pathByteSize int,
	batchProof *ledger.TrieBatchProof,
	lh *hasher.LedgerHasher,
) (*PSMT, error) {

	if pathByteSize < 1 {
		return nil, errors.New("trie's path size [in bytes] must be positive")
	}
	psmt := PSMT{newNode(nil, pathByteSize*8), pathByteSize, make(map[string]*node), lh}

	paths := batchProof.Paths()
	payloads := batchProof.Payloads()

	// check that size of path, size of payloads are consistent
	if len(paths) != len(payloads) {
		return nil, fmt.Errorf("paths' size (%d) and payloads' size (%d) doesn't match", len(paths), len(payloads))
	}
	// check that size of path, size of proofs are consistent
	if len(paths) != batchProof.Size() {
		return nil, fmt.Errorf("paths' size (%d) and proofs' size (%d) doesn't match", len(paths), batchProof.Size())
	}

	// iterating over proofs for building the tree
	for i, pr := range batchProof.Proofs {
		path := paths[i]
		payload := payloads[i]
		// check path size
		if len(path) != pathByteSize {
			return nil, fmt.Errorf("path [%x] size (%d) doesn't match the expected value (%d)", path, len(path), pathByteSize)
		}

		// we keep track of our progress through proofs by proofIndex
		prValueIndex := 0

		// start from the rootNode and walk down the tree
		currentNode := psmt.root

		// we process the path, bit by bit, until we reach the end of the proof (due to compactness)
		for j := 0; j < int(pr.Steps); j++ {
			// if a flag (bit j in flags) is false, the value is a default value
			// otherwise the value is stored in the proofs
			v := lh.GetDefaultHashForHeight(currentNode.height - 1)
			flagIsSet, err := utils.IsBitSet(pr.Flags, j)
			if err != nil {
				return nil, err
			}
			if flagIsSet {
				// use the proof at index proofIndex
				v = pr.Interims[prValueIndex]
				prValueIndex++
			}
			bitIsSet, err := utils.IsBitSet(path, j)
			if err != nil {
				return nil, err
			}
			// look at the bit number j (left to right) for branching
			if bitIsSet { // right branching
				if currentNode.lChild == nil { // check left child
					currentNode.lChild = newNode(v, currentNode.height-1)
				}
				//  else if !bytes.Equal(currentNode.lChild.ComputeValue(), v) {
				// 	return nil, fmt.Errorf("incompatible proof (left node value doesn't match) expected [%x], got [%x]", currentNode.lChild.ComputeValue(), v)
				// }
				if currentNode.rChild == nil { // create the right child if not exist
					currentNode.rChild = newNode(nil, currentNode.height-1)
				}
				currentNode = currentNode.rChild
			} else { // left branching
				if currentNode.rChild == nil { // check right child
					currentNode.rChild = newNode(v, currentNode.height-1)
				}
				// else if !bytes.Equal(currentNode.rChild.ComputeValue(), v) {
				// 	return nil, fmt.Errorf("incompatible proof (right node value doesn't match) expected [%x], got [%x]", currentNode.rChild.ComputeValue(), v)
				// }
				if currentNode.lChild == nil { // create the left child if not exist
					currentNode.lChild = newNode(nil, currentNode.height-1)
				}
				currentNode = currentNode.lChild
			}
		}

		currentNode.payload = payload
		currentNode.path = path
		// update node's hashvalue only for inclusion proofs (for others we assume default value)
		if pr.Inclusion {
			currentNode.hashValue = lh.ComputeCompactValue(path, payload, currentNode.height)
		}
		// keep a reference to this node by path (for update purpose)
		psmt.pathLookUp[string(path)] = currentNode

	}

	// check if the rootHash matches the root node's hash value of the partial trie
	if !bytes.Equal(psmt.root.HashValue(lh), rootValue) {
		return nil, fmt.Errorf("rootNode hash doesn't match the proofs expected [%x], got [%x]", psmt.root.HashValue(lh), rootValue)
	}
	return &psmt, nil
}
