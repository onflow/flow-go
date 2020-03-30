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

// GetRootHash returns the root value of the SMT
func (p *PSMT) GetRootHash() []byte {
	return p.root.ComputeValue()
}

// Update updates the register values and returns rootValue after updates
// in case of error, it returns a list of keys for which update failed
func (p *PSMT) Update(registerIDs [][]byte, values [][]byte) ([]byte, []string, error) {
	var failedKeys []string
	for i, key := range registerIDs {
		value := values[i]

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
	return p.root.ComputeValue(), failedKeys, nil
}

// NewPSMT builds a PSMT given chunkdatapack registertouches
func NewPSMT(
	rootValue []byte, // stateCommitment
	height int,
	keys [][]byte,
	values [][]byte,
	proofs [][]byte,
) (*PSMT, error) {

	psmt := PSMT{newNode(nil, height-1), height, make(map[string]*node)}
	// TODO add checks for decode proof
	proofholder := DecodeProof(proofs)

	// iterating over proofs
	for i, proofSize := range proofholder.sizes {
		value := values[i]
		key := keys[i]
		flags := proofholder.flags[i]
		proof := proofholder.proofs[i]
		inclusion := proofholder.inclusions[i]

		// we keep track of our progress through proofs by proofIndex
		proofIndex := 0

		// start from the root and walk down the tree
		currentNode := psmt.root

		// we process the key bit by bit until we reach the end of the proof (due to compactness)
		for j := 0; j < int(proofSize); j++ {
			// determine v
			v := GetDefaultHashForHeight(currentNode.height - 1)
			// if a flag (bit j in flags) is false, the value is a default value
			// otherwise the value is stored in the proofs
			if utils.IsBitSet(flags, j) {
				// use the proof at index proofIndex
				v = proof[proofIndex]
				proofIndex++
			}
			if utils.IsBitSet(key, j) { // right branching
				if currentNode.Lchild == nil { // check left child
					currentNode.Lchild = newNode(v, currentNode.height-1)
				} else if !bytes.Equal(currentNode.Lchild.ComputeValue(), v) {
					return nil, fmt.Errorf("incompatible proof (left node value doesn't match)")
				}
				if currentNode.Rchild == nil { // create the right child if not exist
					currentNode.Rchild = newNode(nil, currentNode.height-1)
				}
				currentNode = currentNode.Rchild
			} else { // left branching
				if currentNode.Rchild == nil { // check right child
					currentNode.Rchild = newNode(v, currentNode.height-1)
				} else if !bytes.Equal(currentNode.Rchild.ComputeValue(), v) {
					return nil, fmt.Errorf("incompatible proof (right node value doesn't match)")
				}
				if currentNode.Lchild == nil { // create the left child if not exist
					currentNode.Lchild = newNode(nil, currentNode.height-1)
				}
				currentNode = currentNode.Lchild
			}
		}
		if inclusion { // inclusion proof
			// set leaf
			currentNode.key = key
			psmt.keyLookUp[string(key)] = currentNode
			currentNode.value = ComputeCompactValue(key, value, currentNode.height, height)

		} else { // exclusion proof
			// expand it till reaching the leaf node
			for j := currentNode.height; j > 0; j-- {
				v := GetDefaultHashForHeight(j - 1)
				if utils.IsBitSet(key, height-j-1) { // right branching
					if currentNode.Rchild == nil {
						currentNode.Rchild = newNode(nil, currentNode.height-1)
					}
					if currentNode.Lchild == nil {
						currentNode.Lchild = newNode(v, currentNode.height-1)
					}
					if !bytes.Equal(currentNode.Lchild.ComputeValue(), v) {
						return nil, fmt.Errorf("incompatible proof (left node value doesn't match)")
					}
					currentNode = currentNode.Rchild
				} else { // left branching
					if currentNode.Lchild == nil {
						currentNode.Lchild = newNode(nil, currentNode.height-1)
					}
					if currentNode.Rchild == nil {
						currentNode.Rchild = newNode(v, currentNode.height-1)
					}
					if !bytes.Equal(currentNode.Rchild.ComputeValue(), v) {
						return nil, fmt.Errorf("incompatible proof (left node value doesn't match)")
					}
					currentNode = currentNode.Lchild
				}
			}
			// set leaf
			currentNode.key = key
			psmt.keyLookUp[string(key)] = currentNode
			currentNode.value = defaultLeafHash
		}
	}
	// check if the state commitment matches
	// useing computeValue instead of value for extensive checking
	if !bytes.Equal(psmt.root.ComputeValue(), rootValue) {
		return nil, fmt.Errorf("root hash doesn't match the proofs")
	}
	return &psmt, nil
}
