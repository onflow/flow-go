package trie

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

// PSMT (Partial Sparse Merkle Tree) holds a subset of a sparse merkle tree (SMT) at specific
// state commitment (no historic views). It avoids compact nodes to differentiate between
// unpopulated branches and parts that are compact. It is fully stored in memory and doesn't use
// a database.
type PSMT struct {
	root      *node // Root
	height    int   // Height of the tree
	keyLookUp map[string]*node
}

// GetHeight returns the Height of the SMT
func (p *PSMT) GetHeight() int {
	return p.height
}

// GetRootHash returns the rootNode value of the SMT
func (p *PSMT) GetRootHash() []byte {
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
		node.value = ComputeCompactValue(key, value, node.height, p.height)
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
	height int,
	keys [][]byte,
	values [][]byte,
	proofs [][]byte,
) (*PSMT, error) {

	if height < 1 {
		return nil, fmt.Errorf("minimum acceptable value for the  hight is 1")
	}
	psmt := PSMT{newNode(nil, height-1), height, make(map[string]*node)}

	// We need to decode proof encodings
	proofholder, err := DecodeProof(proofs)
	if err != nil {
		return nil, fmt.Errorf("decoding proof failed: %w", err)
	}

	// check size of key, values and proofs
	if len(keys) != len(values) {
		return nil, fmt.Errorf("keys' size (%d) and values' size (%d) doesn't match", len(keys), len(values))
	}

	if len(keys) != len(proofholder.sizes) {
		return nil, fmt.Errorf("keys' size (%d) and values' size (%d) doesn't match", len(keys), len(proofholder.sizes))
	}

	// iterating over proofs for building the tree
	for i, proofSize := range proofholder.sizes {
		value := values[i]
		key := keys[i]
		flags := proofholder.flags[i]
		proof := proofholder.proofs[i]
		inclusion := proofholder.inclusions[i]

		// check key size
		if len(key) != (height-1)/8 {
			return nil, fmt.Errorf("key [%x] size (%d) doesn't match the trie height (%d)", key, len(key), height)

		}

		// we keep track of our progress through proofs by proofIndex
		proofIndex := 0

		// start from the rootNode and walk down the tree
		currentNode := psmt.root

		// we process the key bit by bit until we reach the end of the proof (due to compactness)
		for j := 0; j < int(proofSize); j++ {
			// if a flag (bit j in flags) is false, the value is a default value
			// otherwise the value is stored in the proofs
			v := GetDefaultHashForHeight(currentNode.height - 1)
			if utils.IsBitSet(flags, j) {
				// use the proof at index proofIndex
				v = proof[proofIndex]
				proofIndex++
			}
			// look at the bit number j (left to right) for branching
			if utils.IsBitSet(key, j) { // right branching
				if currentNode.lChild == nil { // check left child
					currentNode.lChild = newNode(v, currentNode.height-1)
				} else if !bytes.Equal(currentNode.lChild.ComputeValue(), v) {
					return nil, fmt.Errorf("incompatible proof (left node value doesn't match) expected [%x], got [%x]", currentNode.lChild.ComputeValue(), v)
				}
				if currentNode.rChild == nil { // create the right child if not exist
					currentNode.rChild = newNode(nil, currentNode.height-1)
				}
				currentNode = currentNode.rChild
			} else { // left branching
				if currentNode.rChild == nil { // check right child
					currentNode.rChild = newNode(v, currentNode.height-1)
				} else if !bytes.Equal(currentNode.rChild.ComputeValue(), v) {
					return nil, fmt.Errorf("incompatible proof (right node value doesn't match) expected [%x], got [%x]", currentNode.rChild.ComputeValue(), v)
				}
				if currentNode.lChild == nil { // create the left child if not exist
					currentNode.lChild = newNode(nil, currentNode.height-1)
				}
				currentNode = currentNode.lChild
			}
		}
		if inclusion { // inclusion proof
			// for the last node set the leaf key and value
			currentNode.key = key
			currentNode.value = ComputeCompactValue(key, value, currentNode.height, height)
			// keep a reference to this node by key (for update purpose)
			psmt.keyLookUp[string(key)] = currentNode

		} else { // exclusion proof
			// for exclusion proofs we continue expanding the tree till it reaches the leaf node
			// this will simplify the update operations for these nodes
			// by differentiating unknown registers and unset registers
			for j := currentNode.height; j > 0; j-- {
				// set node value based on the height
				v := GetDefaultHashForHeight(j - 1)
				// continue checking the key bits
				if utils.IsBitSet(key, height-j-1) { // right branching
					if currentNode.rChild == nil {
						currentNode.rChild = newNode(nil, currentNode.height-1)
					}
					if currentNode.lChild == nil {
						currentNode.lChild = newNode(v, currentNode.height-1)
					}
					if !bytes.Equal(currentNode.lChild.ComputeValue(), v) {
						return nil, fmt.Errorf("incompatible proof (left node value doesn't match) expected [%x], got [%x]", currentNode.lChild.ComputeValue(), v)
					}
					currentNode = currentNode.rChild
				} else { // left branching
					if currentNode.lChild == nil {
						currentNode.lChild = newNode(nil, currentNode.height-1)
					}
					if currentNode.rChild == nil {
						currentNode.rChild = newNode(v, currentNode.height-1)
					}
					if !bytes.Equal(currentNode.rChild.ComputeValue(), v) {
						return nil, fmt.Errorf("incompatible proof (left node value doesn't match) expected [%x], got [%x]", currentNode.rChild.ComputeValue(), v)
					}
					currentNode = currentNode.lChild
				}
			}
			// set leaf
			currentNode.key = key
			psmt.keyLookUp[string(key)] = currentNode
			currentNode.value = defaultLeafHash
		}
	}
	// check if the state commitment matches the root value of the partial trie
	if !bytes.Equal(psmt.root.ComputeValue(), rootValue) {
		return nil, fmt.Errorf("rootNode hash doesn't match the proofs expected [%x], got [%x]", psmt.root.ComputeValue(), rootValue)
	}
	return &psmt, nil
}
