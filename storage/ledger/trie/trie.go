package trie

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/gammazero/deque"
	lru "github.com/hashicorp/golang-lru"

	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

var EmptySlice []byte
var defaultLeafHash []byte = Hash(EmptySlice)
var nilChild []byte = make([]byte, 32)

// node is a struct for constructing our Tree
type node struct {
	value  []byte // Hash
	Lchild *node  // Left Child
	Rchild *node  // Right Child
	height int    // Height where the node is at
	key    []byte // key this node is pointing at
}

// SMT is a Basic Sparse Merkle Tree struct
type SMT struct {
	root                 *node                    // Root
	defaultHashes        [256][]byte              // The zero hashes of the level of the tree
	height               int                      // Height of the tree
	database             databases.DAL            // The Database Interface for the trie
	historicalStates     map[string]databases.DAL // Map of string representations of Historical States to Historical Database references
	cachedBranches       map[string]*proofHolder  // Map of string representationf of keys to proofs
	historicalStateRoots deque.Deque              // FIFO queue of historical State Roots in historicalStates map
	numHistoricalStates  int                      // Number of states to keep in historicalStates
	snapshotInterval     int                      // When removing full states from historical states interval between full states
	numFullStates        int                      // Number of Full States to keep in historicalStates
	lruCache             *lru.Cache               // LRU cache of stringified keys to proofs
}

// Hash hashes any input with SHA256.
func Hash(data ...[]byte) []byte {
	hasher := sha256.New()
	for i := 0; i < len(data); i++ {
		_, err := hasher.Write(data[i])
		if err != nil {
			panic(err)
		}
	}
	return hasher.Sum(nil)
}

// newNode creates a new node with the provided value and no children
func newNode(value []byte, height int) *node {
	n := new(node)
	n.value = value
	n.height = height
	n.Lchild = nil
	n.Rchild = nil
	n.key = nil

	return n
}

// GetValue returns the value of the node.
func (n *node) GetValue() []byte {
	return n.value
}

// GetHeight returns the height of the node.
func (n *node) GetHeight() int {
	return n.height
}

// NewSMT creates a new Sparse Merkle Tree.
//
// This function creates the default hashes and populates the tree.
//
// Note: height must be greater than 1.
func NewSMT(
	db databases.DAL,
	height int,
	cacheSize int,
	interval int,
	numHistoricalStates int,
	numFullStates int,
) (*SMT, error) {
	if height < 1 {
		return nil, errors.New("Height of SMT must be at least 1")
	}

	s := new(SMT)

	s.database = db
	s.height = height

	// Creates the Default hashes from base to level height
	s.defaultHashes[0] = defaultLeafHash
	for i := 1; i < height; i++ {
		s.defaultHashes[i] = Hash(s.defaultHashes[i-1], s.defaultHashes[i-1])
	}

	// Set root to the highest level default node
	s.root = newNode(s.defaultHashes[height-1], height-1)
	s.historicalStates = make(map[string]databases.DAL)
	s.numHistoricalStates = numHistoricalStates
	s.numFullStates = numFullStates
	s.snapshotInterval = interval
	s.cachedBranches = make(map[string]*proofHolder)

	lruCache, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}

	s.lruCache = lruCache

	return s, nil
}

// proofHolder is a struct that holds the proofs and flags from a proof check
type proofHolder struct {
	flags      [][]byte   // The flags of the proofs
	proofs     [][][]byte // the non-default nodes in the proof
	inclusions []bool     // flag indicating if this is an inclusion or exclusion
	sizes      []uint8    // size of the proof in steps
}

// newProofHolder is a constructor for proofHolder
func newProofHolder(flags [][]byte, proofs [][][]byte, inclusions []bool, sizes []uint8) *proofHolder {
	holder := new(proofHolder)
	holder.flags = flags
	holder.proofs = proofs
	holder.inclusions = inclusions
	holder.sizes = sizes

	return holder
}

// GetSize returns the length of the proofHolder
func (p *proofHolder) GetSize() int {
	return len(p.flags)
}

// ExportProof return the flag, proofs, inclusion, an size of the proof at index i
func (p *proofHolder) ExportProof(index int) ([]byte, [][]byte, bool, uint8) {
	return p.flags[index], p.proofs[index], p.inclusions[index], p.sizes[index]
}

// ExportWholeProof returns the proof holder seperated into it's individual fields
func (p *proofHolder) ExportWholeProof() ([][]byte, [][][]byte, []bool, []uint8) {
	return p.flags, p.proofs, p.inclusions, p.sizes
}

// updateCache takes a key and all relevant parts of the proof and insterts in into the cache, removing values if needed
func (s *SMT) updateCache(key []byte, flag []byte, proof [][]byte, inclusion bool, size uint8) {
	holder := newProofHolder([][]byte{flag}, [][][]byte{proof}, []bool{inclusion}, []uint8{size})
	k := hex.EncodeToString(key)
	s.lruCache.Add(k, nil)
	s.cachedBranches[k] = holder
}

// invalidateCache removes the given keys from the cache
func (s *SMT) invalidateCache(keys [][]byte) {
	for _, key := range keys {
		k := hex.EncodeToString(key)
		res := s.cachedBranches[k]
		if res != nil {
			delete(s.cachedBranches, k)
			s.lruCache.Remove(k)
		}
	}
}

// Read takes the keys given and return the values from the database
// If the trusted flag is true, it is just a read from the database
// If trusted is false, then we check to see if the key exists in the trie
func (s *SMT) Read(keys [][]byte, trusted bool, root []byte) ([][]byte, *proofHolder, error) {

	flags := make([][]byte, len(keys))
	proofs := make([][][]byte, len(keys))
	inclusions := make([]bool, len(keys))
	sizes := make([]uint8, len(keys))

	currRoot := s.GetRoot().value
	stringRoot := hex.EncodeToString(root)

	if !trusted {
		if bytes.Equal(root, currRoot) {
			for i, key := range keys {
				k := hex.EncodeToString(key)
				res := s.cachedBranches[k]
				if res == nil {
					flag, proof, size, inclusion := s.GetProof(key)
					flags[i] = flag
					proofs[i] = proof
					inclusions[i] = inclusion
					sizes[i] = size
				} else {
					flags[i] = res.flags[0]
					proofs[i] = res.proofs[0]
					inclusions[i] = res.inclusions[0]
					sizes[i] = res.sizes[0]
				}

				s.updateCache(key, flags[i], proofs[i], inclusions[i], sizes[i])

			}
		} else {

			if s.historicalStates[stringRoot] == nil {
				return nil, nil, errors.New("Invalid Historical State")

			}

			for i, key := range keys {
				flag, proof, size, inclusion, err := s.GetHistoricalProof(key, root, s.historicalStates[stringRoot])
				if err != nil {
					return nil, nil, err
				}

				flags[i] = flag
				proofs[i] = proof
				inclusions[i] = inclusion
				sizes[i] = size
			}
		}
	}

	values := make([][]byte, len(keys))

	// the case where we are reading from current state
	if bytes.Equal(root, currRoot) {
		for i, key := range keys {
			res, err := s.database.GetKVDB(key)
			if err != nil {
				return nil, nil, err
			}
			values[i] = res
		}

		// the case where the root we are looking for does not match our current root
	} else {
		// check to see if it is historical
		if s.historicalStates[stringRoot] == nil {
			return nil, nil, errors.New("Invalid Historical State")

		}

		for i, key := range keys {
			res, err := s.historicalStates[stringRoot].GetKVDB(key)
			if err != nil {
				index := s.getStateIndex(stringRoot)

				for j := index; j >= 0; j-- {
					db := s.historicalStates[fmt.Sprintf("%v", s.historicalStateRoots.At(j))]
					res, err = db.GetKVDB(key)
					if err == nil {
						break
					}
				}

				if res == nil {
					return nil, nil, err
				}
			}
			values[i] = res
		}

	}

	if trusted {
		return values, nil, nil
	}
	holder := newProofHolder(flags, proofs, inclusions, sizes)
	return values, holder, nil
}

// verifyInclusionFlag is used to verify a flag against the trie given the key
func (s *SMT) verifyInclusionFlag(key []byte, flag []byte) bool {

	eflag := make([]byte, s.GetHeight()/8)

	curr := s.GetRoot()
	if curr == nil || bytes.Equal(curr.key, key) {
		return bytes.Equal(eflag, flag)

	}

	var flagMatches bool

	for i := 0; i < len(flag); i++ {
		if utils.IsBitSet(key, i) {
			if curr.Lchild != nil {
				flagMatches = utils.IsBitSet(flag, i)
				if !flagMatches {
					return false
				}
			}

		} else {
			if curr.Rchild != nil {
				flagMatches = utils.IsBitSet(flag, i)
				if !flagMatches {
					return false
				}
			}

		}
	}

	return true
}

// GetProof searching the tree for a value if it exists, and returns the flag and then proof
func (s *SMT) GetProof(key []byte) ([]byte, [][]byte, uint8, bool) {
	flag := make([]byte, s.GetHeight()/8)
	proof := make([][]byte, 0)
	proofLen := uint8(0)

	curr := s.GetRoot()
	if curr == nil {
		return flag, proof, 0, false
	}
	if bytes.Equal(curr.key, key) {
		return flag, proof, proofLen, true
	}

	var nextKey *node

	for i := 0; i < s.GetHeight()-1; i++ {
		if utils.IsBitSet(key, i) {
			if curr.Lchild != nil {
				utils.SetBit(flag, i)
				proof = append(proof, curr.Lchild.value)
			}

			nextKey = curr.Rchild

		} else {
			if curr.Rchild != nil {
				utils.SetBit(flag, i)
				proof = append(proof, curr.Rchild.value)
			}

			nextKey = curr.Lchild
		}

		if nextKey == nil {
			return flag, proof, proofLen, false
		} else {
			curr = nextKey
			proofLen++
			if bytes.Equal(key, curr.key) {
				return flag, proof, proofLen, true
			}
		}
	}

	if curr.key == nil {
		return flag, proof, proofLen, false
	}

	return flag, proof, proofLen, true
}

// getStateIndex returns the index of a stateroot in the historical state roots buffer
func (s *SMT) getStateIndex(stateRoot string) int {
	for i := 0; i < s.historicalStateRoots.Len(); i++ {
		if fmt.Sprintf("%v", s.historicalStateRoots.At(i)) == stateRoot {
			return i
		}
	}

	return -1
}

// GetHistoricalProof reconstructs a proof of inclusion or exclusion for a value in a historical database then returns the flag and proof
func (s *SMT) GetHistoricalProof(key []byte, root []byte, database databases.DAL) ([]byte, [][]byte, uint8, bool, error) {
	flag := make([]byte, s.GetHeight()/8)
	proof := make([][]byte, 0)
	proofLen := uint8(0)

	curr := root
	if curr == nil {
		return flag, proof, 0, false, nil
	}

	var nextKey []byte

	for i := 0; i < s.GetHeight()-1; i++ {
		children, err := database.GetTrieDB(curr)
		if err != nil {
			index := s.getStateIndex(hex.EncodeToString(root))

			for j := index; j >= 0; j-- {
				db := s.historicalStates[fmt.Sprintf("%v", s.historicalStateRoots.At(j))]
				children, err = db.GetTrieDB(curr)
				if err == nil {
					break
				}
			}

			if children == nil {
				return flag, proof, 0, false, err
			}
		}

		var Lchild []byte
		var Rchild []byte
		var Ckey []byte

		// the case where we are not at a leaf node!
		if len(children) == 64 {
			// retrieve value for root from the database, split into Left child value and Right child value
			// by splitting value slice in half!
			Lchild = children[0:32]
			Rchild = children[32:64]
		} else if len(children) == 33 {
			// The case where the node we are pulling from the database has only one set child
			// check the first bit of the flag
			lr := utils.IsBitSet(children, 0)

			if lr {
				// If it is set the left child is nil
				Lchild = nilChild
				Rchild = children[1:33]

			} else {
				// If it is not set the right child is nil
				Lchild = children[1:33]
				Rchild = nilChild
			}

		} else {
			// we are at a leaf node, L and R children will be nil and the key will be the value in the DB
			Lchild, Rchild = nilChild, nilChild
			Ckey = children
		}
		if utils.IsBitSet(key, i) {
			if !bytes.Equal(Lchild, nilChild) {
				utils.SetBit(flag, i)
				proof = append(proof, Lchild)
			}

			nextKey = Rchild

		} else {
			if !bytes.Equal(Rchild, nilChild) {
				utils.SetBit(flag, i)
				proof = append(proof, Rchild)
			}

			nextKey = Lchild
		}

		// We are either at a leaf node or the key is not included in the trie
		if bytes.Equal(nextKey, nilChild) {
			// at a leaf node
			if bytes.Equal(key, Ckey) {
				return flag, proof, proofLen, true, nil
			}

			// key not included in trie
			return flag, proof, proofLen, false, nil
		} else {
			curr = nextKey
			proofLen++
			if bytes.Equal(key, curr) {
				return flag, proof, proofLen, true, nil
			}
		}
	}

	return flag, proof, proofLen, !bytes.Equal(curr, nilChild), nil

}

// VerifyInclusionProof calculates the inclusion proof from a given root, flag, proof list, and size.
//
// This function is exclusively for inclusive proofs
func VerifyInclusionProof(key []byte, value []byte, flag []byte, proof [][]byte, size uint8, root []byte, hashes [256][]byte, height int) bool {
	// get index of proof we start our calculations from
	proofIndex := 0

	if len(proof) != 0 {
		proofIndex = len(proof) - 1
	}

	// base case at the bottom of the trie
	computed := Hash(key, value)
	for i := int(size); i > 0; i-- {
		// hashing is order dependant
		if utils.IsBitSet(key, i-1) {
			if !utils.IsBitSet(flag, i-1) {
				computed = Hash(hashes[(height-1)-i], computed)
			} else {
				computed = Hash(proof[proofIndex], computed)
				proofIndex = proofIndex - 1
			}

		} else {
			if !utils.IsBitSet(flag, i-1) {
				computed = Hash(computed, hashes[(height-1)-i])
			} else {
				computed = Hash(computed, proof[proofIndex])
				proofIndex = proofIndex - 1
			}
		}
	}
	return bytes.Equal(computed, root)
}

func VerifyNonInclusionProof(key []byte, value []byte, flag []byte, proof [][]byte, size uint8, root []byte, hashes [256][]byte, height int) bool {
	// get index of proof we start our calculations from
	proofIndex := 0

	if len(proof) != 0 {
		proofIndex = len(proof) - 1
	}

	// base case at the bottom of the trie
	computed := Hash(key, value)
	for i := int(size); i > 0; i-- {
		//hashing is order dependant
		if utils.IsBitSet(key, i-1) {
			if !utils.IsBitSet(flag, i-1) {
				computed = Hash(hashes[(height-1)-i], computed)
			} else {
				computed = Hash(proof[proofIndex], computed)
				proofIndex = proofIndex - 1
			}

		} else {
			if !utils.IsBitSet(flag, i-1) {
				computed = Hash(computed, hashes[(height-1)-i])
			} else {
				computed = Hash(computed, proof[proofIndex])
				proofIndex = proofIndex - 1
			}
		}
	}
	return !bytes.Equal(computed, root)
}

func (s *SMT) updateHistoricalStates(root []byte) error {
	// Make a copy of the historical state and link it to the old state root
	// get string representation of the current root of the trie
	oldRoot := hex.EncodeToString(root)
	historicDB, err := s.database.CopyDB(oldRoot)
	if err != nil {
		return err
	}
	s.historicalStates[oldRoot] = historicDB
	numStates := s.historicalStateRoots.Len()

	switch {
	case numStates > s.numHistoricalStates:
		return errors.New("We have more Historical States stored than the Maximum Allowed Amount!")

	case numStates < s.numHistoricalStates && numStates < s.numFullStates:
		s.historicalStateRoots.PushBack(oldRoot)

	case numStates < s.numHistoricalStates && numStates >= s.numFullStates:
		s.historicalStateRoots.PushBack(oldRoot)
		si := numStates - s.numFullStates
		if si%s.snapshotInterval != 0 {
			stateToPrune := fmt.Sprintf("%v", s.historicalStateRoots.At(s.numFullStates))
			referenceState := fmt.Sprintf("%v", s.historicalStateRoots.At(s.numFullStates-1))
			err = s.historicalStates[stateToPrune].PruneDB(s.historicalStates[referenceState])
			if err != nil {
				return err
			}
		}

	case numStates == s.numHistoricalStates:
		s.historicalStateRoots.PushBack(oldRoot)
		rootToRemove := fmt.Sprintf("%v", s.historicalStateRoots.PopFront())
		s.historicalStates[rootToRemove] = nil
		si := numStates - s.numFullStates
		if si%s.snapshotInterval != 0 {
			stateToPrune := fmt.Sprintf("%v", s.historicalStateRoots.At(s.numFullStates))
			referenceState := fmt.Sprintf("%v", s.historicalStateRoots.At(s.numFullStates-1))
			err = s.historicalStates[stateToPrune].PruneDB(s.historicalStates[referenceState])
			if err != nil {
				return err
			}
		}
	}

	return nil

}

// Update takes a sorted list of keys and associated values and inserts
// them into the trie, and if that is successful updates the databases.
func (s *SMT) Update(keys [][]byte, values [][]byte) error {
	s.database.NewBatch()
	res, err := s.UpdateAtomically(s.GetRoot(), keys, values, s.height-1)
	if err != nil {
		return err
	}

	err = s.updateHistoricalStates(s.GetRoot().value)
	if err != nil {
		return err
	}

	err = s.database.UpdateTrieDB()
	if err != nil {
		return err
	}

	err = s.database.UpdateKVDB(keys, values)
	if err != nil {
		return err
	}

	s.root = res

	s.invalidateCache(keys)

	return nil
}

// GetHeight returns the Height of the SMT
func (s *SMT) GetHeight() int {
	return s.height
}

// GetRoot returns the Root of the SMT
func (s *SMT) GetRoot() *node {
	return s.root
}

// GetDefaultHashes returns the default hashes of the SMT.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func (s *SMT) GetDefaultHashes() [256][]byte {
	return s.defaultHashes
}

func (s *SMT) insertIntoKeys(insert []byte, keys [][]byte, values [][]byte) ([][]byte, [][]byte, error) {
	for i, key := range keys {
		if bytes.Equal(insert, key) {
			return keys, values, nil
		}

		if bytes.Compare(insert, key) < 0 {
			// Insert the key into the keys
			newkeys := make([][]byte, 0)
			newkeys = append(newkeys, keys[:i]...)
			newkeys = append(newkeys, insert)
			newkeys = append(newkeys, keys[i:]...)

			// Insert the old value into values
			newvalues := make([][]byte, 0)
			newvalues = append(newvalues, values[:i]...)
			oldVal, err := s.database.GetKVDB(insert)
			if err != nil {
				return nil, nil, err
			}
			newvalues = append(newvalues, oldVal)
			newvalues = append(newvalues, values[i:]...)

			return newkeys, newvalues, nil
		}
	}

	oldVal, err := s.database.GetKVDB(insert)
	if err != nil {
		return nil, nil, err
	}
	return append(keys, insert), append(values, oldVal), nil
}

// UpdateAtomically updates the trie atomically and returns the state root
// NOTE: This function assumes keys and values are sorted and haves indexes mapping to each other
func (s *SMT) UpdateAtomically(rootNode *node, keys [][]byte, values [][]byte, height int) (*node, error) {
	var err error
	if rootNode.value != nil {
		s.database.PutIntoBatcher(rootNode.value, nil)
	}
	if rootNode.key != nil {
		keys, values, err = s.insertIntoKeys(rootNode.key, keys, values)
		if err != nil {
			return nil, err
		}
		s.invalidateCache([][]byte{rootNode.key})
		rootNode.key = nil
	}

	if len(keys) != len(values) {
		return nil, errors.New("Total Key/Value Length mismatch")
	}

	// If we are at a leaf node, then we create said node
	if len(keys) == 1 && rootNode.Lchild == nil && rootNode.Rchild == nil {
		return s.ComputeRootNode(nil, nil, rootNode, keys, values, height), nil
	}

	//We initialize the nodes as empty to prevent nil pointer exceptions later
	lnode, rnode := rootNode.GetandSetChildren(s.defaultHashes)

	// Split the keys and values array so we can update the trie in parallel
	lkeys, rkeys, splitIndex := utils.SplitKeys(keys, s.height-height-1)
	lvalues, rvalues := values[:splitIndex], values[splitIndex:]

	if len(lkeys) != len(lvalues) {
		return nil, errors.New("Left Key/Value Length mismatch")
	}
	if len(rkeys) != len(rvalues) {
		return nil, errors.New("Right Key/Value Length mismatch")
	}

	if len(rkeys) == 0 && len(lkeys) > 0 {
		// if we only have keys belonging on the left side of the trie to update
		return s.updateLeft(lnode, rnode, rootNode, lkeys, lvalues, height)
	} else if len(lkeys) == 0 && len(rkeys) > 0 {
		// if we only have keys belonging on the right side of the trie to update
		return s.updateRight(lnode, rnode, rootNode, rkeys, rvalues, height)
	} else if len(lkeys) > 0 && len(rkeys) > 0 {
		// update in parallel otherwise
		return s.updateParallel(lnode, rnode, rootNode, keys, values, lkeys, rkeys, lvalues, rvalues, height)
	}

	return rootNode, nil
}

// GetandSetChildren checks if any of the children are nill and creates them as a default node if they are, otherwise
// we just return the children
func (n *node) GetandSetChildren(hashes [256][]byte) (*node, *node) {
	if n.Lchild == nil {
		n.Lchild = newNode(nil, n.height-1)
	}
	if n.Rchild == nil {
		n.Rchild = newNode(nil, n.height-1)
	}

	return n.Lchild, n.Rchild
}

// updateParallel updates both the left and right subtrees and computes a new rootNode
func (s *SMT) updateParallel(lnode *node, rnode *node, rootNode *node, keys [][]byte, values [][]byte, lkeys [][]byte, rkeys [][]byte, lvalues [][]byte, rvalues [][]byte, height int) (*node, error) {
	lupdate, err1 := s.UpdateAtomically(lnode, lkeys, lvalues, height-1)
	rupdate, err2 := s.UpdateAtomically(rnode, rkeys, rvalues, height-1)

	if err1 != nil {
		return nil, err1
	}
	if err2 != nil {
		return nil, err2
	}

	return s.ComputeRootNode(lupdate, rupdate, rootNode, keys, values, height), nil
}

// updateLeft updates the left subtrees and computes a new rootNode
func (s *SMT) updateLeft(lnode *node, rnode *node, rootNode *node, lkeys [][]byte, lvalues [][]byte, height int) (*node, error) {
	update, err := s.UpdateAtomically(lnode, lkeys, lvalues, height-1)
	if err != nil {
		return nil, err
	}
	return s.ComputeRootNode(update, rnode, rootNode, lkeys, lvalues, height), nil
}

// updateRight updates the right subtrees and computes a new rootNode
func (s *SMT) updateRight(lnode *node, rnode *node, rootNode *node, rkeys [][]byte, rvalues [][]byte, height int) (*node, error) {
	update, err := s.UpdateAtomically(rnode, rkeys, rvalues, height-1)
	if err != nil {
		return nil, err
	}
	return s.ComputeRootNode(lnode, update, rootNode, rkeys, rvalues, height), nil
}

// ComputeRoot either returns a new leafNode or computes a new rootNode by hashing its children
func (s *SMT) ComputeRootNode(lnode *node, rnode *node, oldRootNode *node, keys [][]byte, values [][]byte, height int) *node {
	if lnode == nil && rnode == nil {
		ln := newNode(Hash(keys[0], values[0]), height)
		ln.key = keys[0]
		s.database.PutIntoBatcher(ln.value, ln.key)
		return ln
	} else {
		return s.interiorNode(lnode, rnode, height)
	}
}

// interiorNode computes the new node's Hash by hashing it's children's nodes, and also cleans up any default nodes
// that are not needed
func (s *SMT) interiorNode(lnode *node, rnode *node, height int) *node {

	// If any nodes are default nodes, they are no longer needed and can be discarded
	if lnode != nil && bytes.Equal(lnode.value, nil) {
		lnode = nil
	}
	if rnode != nil && bytes.Equal(rnode.value, nil) {
		rnode = nil
	}

	// Hashes the children depending on if they are nil or filled
	if (lnode != nil) && (rnode != nil) {
		in := newNode(Hash(lnode.value, rnode.value), height)
		in.Lchild = lnode
		in.Rchild = rnode
		s.database.PutIntoBatcher(in.value, append(in.Lchild.value, in.Rchild.value...))
		return in
	} else if lnode == nil && rnode != nil {
		in := newNode(Hash(s.defaultHashes[height-1], rnode.value), height)
		in.Lchild = lnode
		in.Rchild = rnode
		// if the left node is nil value of the Rchild attached to key in DB will be prefaced by
		// 4 bits with the first bit set
		lFlag := make([]byte, 1)
		utils.SetBit(lFlag, 0)
		s.database.PutIntoBatcher(in.value, append(lFlag, in.Rchild.value...))
		return in
	} else if rnode == nil && lnode != nil {
		in := newNode(Hash(lnode.value, s.defaultHashes[height-1]), height)
		in.Lchild = lnode
		in.Rchild = rnode
		rFlag := make([]byte, 1)
		// if the right node is nil value of the Lchild attached to key in DB will be prefaced by
		// 4 unset bits
		s.database.PutIntoBatcher(in.value, append(rFlag, in.Lchild.value...))
		return in
	}
	return nil
}

// SafeClose is an exported function to safely close the databases
func (s *SMT) SafeClose() (error, error) {
	err1, err2 := s.database.SafeClose()
	if err1 != nil || err2 != nil {
		return err1, err2
	}
	for i := 0; i < s.historicalStateRoots.Len(); i++ {
		db := s.historicalStates[fmt.Sprintf("%v", s.historicalStateRoots.At(i))]
		err1, err2 = db.SafeClose()
		if err1 != nil || err2 != nil {
			return err1, err2
		}
	}

	return nil, nil
}

// DecodeProof takes in an encodes array of byte arrays an converts them into a proofHolder
func DecodeProof(proofs [][]byte) *proofHolder {
	flags := make([][]byte, 0)
	newProofs := make([][][]byte, 0)
	inclusions := make([]bool, 0)
	sizes := make([]uint8, 0)
	// The decode logic is as follows:
	// The first byte in the array is the inclusion flag, with the first bit set as the inclusion (1 = inclusion, 0 = non-inclusion)
	// The second byte is size, needs to be converted to uint8
	// The next 32 bytes are the flag
	// Each subsequent 32 bytes are the proofs needed for the verifier
	// Each result is put into their own array and put into a proofHolder
	for _, proof := range proofs {
		byteInclusion := proof[0:1]
		inclusion := utils.IsBitSet(byteInclusion, 0)
		inclusions = append(inclusions, inclusion)
		sizes = append(sizes, proof[1:2]...)
		if len(proof) > 34 {
			flags = append(flags, proof[2:33])
		} else {
			flags = append(flags, proof[2:])
		}
		byteProofs := make([][]byte, 0)
		i := 33
		for i < len(proof) {
			if i+32 <= len(proof) {
				byteProofs = append(byteProofs, proof[i:i+32])
			} else {
				byteProofs = append(byteProofs, proof[i:])
			}
			i = i + 32
		}
		newProofs = append(newProofs, byteProofs)
	}
	return newProofHolder(flags, newProofs, inclusions, sizes)
}
