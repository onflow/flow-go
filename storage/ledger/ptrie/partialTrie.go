package ptrie

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/proof"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

// PSMT (Partial Sparse Merkle Tree) holds a subset of a sparse merkle tree (SMT) at specific
// state commitment (no historic views). It avoids compact nodes to differentiate between
// unpopulated branches and parts that are compact. It is fully stored in memory and doesn't use
// a database.
//
// DEFINITIONS and CONVENTIONS:
//   * HEIGHT of a node v in a tree is the number of edges on the longest downward path
//     between v and a tree leaf. The height of a tree is the heights of its root.
//     The height of a Trie is always the height of the fully-expanded tree.
type PSMT struct {
	root        *node // Root
	keyByteSize int   // expected size [bytes] of register key
	keyLookUp   map[string]*node
}

// KeySize returns the expected expected size [bytes] of register key
func (p *PSMT) KeySize() int {
	return p.keyByteSize
}

// GetRootHash returns the rootNode value of the SMT
func (p *PSMT) RootHash() []byte {
	return p.root.ComputeValue()
}

// Update updates the register values and returns rootValue after updates
// in case of error, it returns a list of keys for which update failed
func (p *PSMT) Update(registerIDs [][]byte, values [][]byte) ([]byte, []string, error) {
	var failedKeys []string
	for i, key := range registerIDs {
		value := values[i]
		// lookup the key and update the value
		node, found := p.keyLookUp[string(key)]
		if !found {
			failedKeys = append(failedKeys, string(key))
			continue
		}
		node.value = common.ComputeCompactValue(key, value, node.height)
	}
	if len(failedKeys) > 0 {
		return nil, failedKeys, fmt.Errorf("key(s) doesn't exist")
	}
	// after updating all the nodes, compute the value recursively only once
	return p.root.ComputeValue(), failedKeys, nil
}

// NewPSMT builds a Partial Sparse Merkle Tree (PMST) given a chunkdatapack registertouches
func NewPSMT(
	rootValue []byte, // stateCommitment
	keyByteSize int,
	keys [][]byte,
	values [][]byte,
	proofs [][]byte,
) (*PSMT, error) {

	if keyByteSize < 1 {
		return nil, errors.New("trie's key size [in bytes] must be positive")
	}
	psmt := PSMT{newNode(nil, keyByteSize*8), keyByteSize, make(map[string]*node)}

	// Decode proof encodings
	if len(proofs) < 1 {
		return nil, fmt.Errorf("at least a proof is needed to be able to contruct a partial trie")
	}
	batchProof, err := proof.DecodeBatchProof(proofs)
	if err != nil {
		return nil, fmt.Errorf("decoding proof failed: %w", err)
	}

	// check size of key, values and proofs are consistent
	if len(keys) != len(values) {
		return nil, fmt.Errorf("keys' size (%d) and values' size (%d) doesn't match", len(keys), len(values))
	}
	if len(keys) != batchProof.Size() {
		return nil, fmt.Errorf("keys' size (%d) and values' size (%d) doesn't match", len(keys), batchProof.Size())
	}

	// iterating over proofs for building the tree
	for i, pr := range batchProof.Proofs {
		key := keys[i]
		value := values[i]
		// check key size
		if len(key) != keyByteSize {
			return nil, fmt.Errorf("key [%x] size (%d) doesn't match the expected value (%d)", key, len(key), keyByteSize)
		}

		// we keep track of our progress through proofs by proofIndex
		prValueIndex := 0

		// start from the rootNode and walk down the tree
		currentNode := psmt.root

		// we process the key bit by bit until we reach the end of the proof (due to compactness)
		for j := 0; j < int(pr.Steps); j++ {
			// if a flag (bit j in flags) is false, the value is a default value
			// otherwise the value is stored in the proofs
			v := common.GetDefaultHashForHeight(currentNode.height - 1)
			if utils.IsBitSet(pr.Flags, j) {
				// use the proof at index proofIndex
				v = pr.Values[prValueIndex]
				prValueIndex++
			}
			// look at the bit number j (left to right) for branching
			if utils.IsBitSet(key, j) { // right branching
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

		currentNode.key = key
		// update values only for inclusion proofs (for others we assume default value)
		if pr.Inclusion {
			currentNode.value = common.ComputeCompactValue(key, value, currentNode.height)
		}
		// keep a reference to this node by key (for update purpose)
		psmt.keyLookUp[string(key)] = currentNode

	}

	// check if the state commitment matches the root value of the partial trie
	if !bytes.Equal(psmt.root.ComputeValue(), rootValue) {
		return nil, fmt.Errorf("rootNode hash doesn't match the proofs expected [%x], got [%x]", psmt.root.ComputeValue(), rootValue)
	}
	return &psmt, nil
}
