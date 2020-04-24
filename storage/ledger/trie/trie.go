package trie

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/gammazero/deque"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

var nilChild []byte = make([]byte, 32)

var hashLength = 32
var childHashLen = hashLength + 1
var bothChildrenHashLen = 2 * 32

// as long as those are not 32 bytes there is no collision risk
var metadataKeyPrevious = []byte("metadata_previous") //hash of previous state
var metadataHeight = []byte("metadata_height")        //how many deltas since last full snapshot

type Root []byte

func (r Root) String() string {
	return hex.EncodeToString(r)
}

type Key []byte

func (k Key) String() string {
	return hex.EncodeToString(k)
}

// node is a struct for constructing our Tree
type node struct {
	value  Root   // Hash
	lChild *node  // Left Child
	rChild *node  // Right Child
	height int    // Height where the node is at
	key    []byte // key this node is pointing at
}

func (n *node) deepCopy() *node {
	newNode := &node{height: n.height}

	if n.value != nil {
		value := make([]byte, len(n.value))
		copy(value, n.value)
		newNode.value = value
	}
	if n.key != nil {
		key := make([]byte, len(n.key))
		copy(key, n.key)
		newNode.key = key
	}
	if n.lChild != nil {
		newNode.lChild = n.lChild.deepCopy()
	}
	if n.rChild != nil {
		newNode.rChild = n.rChild.deepCopy()
	}
	return newNode
}

type tree struct {
	root           Root
	rootNode       *node
	database       databases.DAL
	previous       Commitment
	cachedBranches map[string]*proofHolder // Map of string representations of keys to proofs
	forest         *forest
	height         uint64 //similar to block height, useful for snapshotting
	// FIFO queue of ordered historical State Roots
	// In-memory only, used for tracking snapshots. Can be reconstructed from disk if needed
	// Frequent eviction of a tree/restarting might mean database not gets pruned as frequent
	historicalStateCommitments deque.Deque
}

func (t *tree) Commitment() Commitment {
	return newCommitment(t.previous, t.root)
}

func (t *tree) Root() (*node, error) {
	if t.rootNode != nil {
		return t.rootNode, nil
	}

	rootNode, err := LoadNode(t.root, t.forest.height, t.database)
	if err != nil {
		return nil, fmt.Errorf("cannot load tree with root (%s): %w", t.root.String(), err)
	}
	t.rootNode = rootNode
	return t.rootNode, nil
}

func LoadNode(value Root, height int, db databases.DAL) (*node, error) {
	if height < 0 {
		return nil, fmt.Errorf("height cannot be negative")
	}
	payload, err := db.GetTrieDB(value)
	if err != nil {
		return nil, fmt.Errorf("cannot load node (%s) from database: %w", value.String(), err)
	}

	payloadLen := len(payload)

	var lChild *node = nil
	var rChild *node = nil
	var lChildRoot Root = nil
	var rChildRoot Root = nil
	var key []byte = nil

	if payloadLen == childHashLen {
		lr := utils.IsBitSet(payload, 0)

		if lr {
			// If it is set the left child is nil
			rChildRoot = payload[1:childHashLen]
		} else {
			// If it is not set the right child is nil
			lChildRoot = payload[1:childHashLen]
		}
	} else if payloadLen == bothChildrenHashLen {
		lChildRoot = payload[0:hashLength]
		rChildRoot = payload[hashLength:bothChildrenHashLen]
	} else {
		key = payload
	}

	if len(rChildRoot) > 0 {
		rChild, err = LoadNode(rChildRoot, height-1, db)
		if err != nil {
			return nil, fmt.Errorf("cannot load right child (%s): %w", rChildRoot, err)
		}
	}
	if len(lChildRoot) > 0 {
		lChild, err = LoadNode(lChildRoot, height-1, db)
		if err != nil {
			return nil, fmt.Errorf("cannot load left child (%s): %w", lChildRoot, err)
		}
	}

	return &node{
		value:  value,
		lChild: lChild,
		rChild: rChild,
		height: height,
		key:    key,
	}, nil
}

// SMT is a Basic Sparse Merkle Tree struct
type SMT struct {
	forest *forest

	//rootNode                 *node                    // Root
	height      int // Height of the tree
	keyByteSize int // acceptable number of bytes for key
	//database             databases.DAL            // The Database Interface for the trie
	//historicalStates     map[string]databases.DAL // Map of string representations of Historical States to Historical Database references
	//cachedBranches       map[string]*proofHolder  // Map of string representations of keys to proofs
	//historicalStateRoots deque.Deque              // FIFO queue of historical State Roots in historicalStates map
	numHistoricalStates int    // Number of states to keep in historicalStates
	snapshotInterval    uint64 // When removing full states from historical states interval between full states
	numFullStates       int    // Number of Full States to keep in historicalStates
	//lruCache             *lru.Cache               // LRU cache of stringified keys to proofs
}

// HashLeaf generates hash value for leaf nodes (SHA3-256).
func HashLeaf(key []byte, value []byte) []byte {
	hasher := hash.NewSHA3_256()
	_, err := hasher.Write(key)
	if err != nil {
		panic(err)
	}
	_, err = hasher.Write(value)
	if err != nil {
		panic(err)
	}

	return hasher.SumHash()
}

// HashInterNode generates hash value for intermediate nodes (SHA3-256).
func HashInterNode(hash1 []byte, hash2 []byte) []byte {
	hasher := hash.NewSHA3_256()
	_, err := hasher.Write(hash1)
	if err != nil {
		panic(err)
	}
	_, err = hasher.Write(hash2)
	if err != nil {
		panic(err)
	}
	return hasher.SumHash()
}

// newNode creates a new node with the provided value and no children
func newNode(value []byte, height int) *node {
	n := new(node)
	n.value = value
	n.height = height
	n.lChild = nil
	n.rChild = nil
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

// ComputeValue recomputes value for this node in recursive manner
func (n *node) ComputeValue() []byte {
	// leaf node
	if n.lChild == nil && n.rChild == nil {
		if n.value != nil {
			return n.value
		}
		return GetDefaultHashForHeight(n.height)
	}
	// otherwise compute
	h1 := GetDefaultHashForHeight(n.height - 1)
	if n.lChild != nil {
		h1 = n.lChild.ComputeValue()
	}
	h2 := GetDefaultHashForHeight(n.height - 1)
	if n.rChild != nil {
		h2 = n.rChild.ComputeValue()
	}
	// For debugging purpose uncomment this
	// n.value = HashInterNode(h1, h2)
	return HashInterNode(h1, h2)
}

// FmtStr provides formated string represntation of the node and sub tree
func (n node) FmtStr(prefix string, path string) string {
	right := ""
	if n.rChild != nil {
		right = fmt.Sprintf("\n%v", n.rChild.FmtStr(prefix+"\t", path+"1"))
	}
	left := ""
	if n.lChild != nil {
		left = fmt.Sprintf("\n%v", n.lChild.FmtStr(prefix+"\t", path+"0"))
	}
	return fmt.Sprintf("%v%v: (%v,%v)[%s] %v %v ", prefix, n.height, n.key, hex.EncodeToString(n.value), path, left, right)
}

// NewSMT creates a new Sparse Merkle Tree.
//
// This function creates the default hashes and populates the tree.
//
// Note: height must be greater than 1.
func NewSMT(
	dbDir string,
	height int,
	interval uint64,
	numHistoricalStates int,
	numFullStates int,
) (*SMT, error) {
	if height < 1 {
		return nil, errors.New("height of SMT must be at least 1")
	}

	// check for possible key length collisions
	// we determine number of children of a node by length of data
	// so those particular values can cause confusion

	if height == childHashLen {
		return nil, fmt.Errorf("height of SMT must not be %d", childHashLen)
	}
	if height == bothChildrenHashLen {
		return nil, fmt.Errorf("height of SMT must not be %d", bothChildrenHashLen)
	}

	s := new(SMT)

	//s.database = db
	s.height = height
	s.keyByteSize = (height - 1) / 8

	// Set rootNode to the highest level default node
	//s.rootNode = newNode(GetDefaultHashForHeight(height-1), height-1)
	//s.historicalStates = make(map[string]databases.DAL)
	s.numHistoricalStates = numHistoricalStates
	s.numFullStates = numFullStates
	s.snapshotInterval = interval
	//s.cachedBranches = make(map[string]*proofHolder)

	forest, err := NewForest(dbDir, numHistoricalStates, numFullStates, height)
	if err != nil {
		return nil, fmt.Errorf("cannot create forest: %w", err)
	}

	// add empty tree
	emptyHash := GetDefaultHashForHeight(height - 1)
	// empty commitment emptyHash + emptyHash
	emptyCommitment := make([]byte, 0)
	emptyCommitment = append(emptyCommitment, emptyHash...)
	emptyCommitment = append(emptyCommitment, emptyHash...)

	emptyDB, err := forest.newDB(emptyHash)
	if err != nil {
		return nil, fmt.Errorf("cannot create empty DB: %w", err)
	}

	emptyNode := newNode(emptyHash, height-1)

	newTree := &tree{
		rootNode:       emptyNode,
		root:           emptyHash,
		database:       emptyDB,
		previous:       nil,
		height:         0,
		cachedBranches: make(map[string]*proofHolder),
		forest:         forest,
	}

	forest.Add(newTree)

	s.forest = forest
	//lruCache, err := lru.New(cacheSize)
	//if err != nil {
	//	return nil, err
	//}

	//s.lruCache = lruCache

	return s, nil
}

// updateCache takes a key and all relevant parts of the proof and insterts in into the cache, removing values if needed
func (t *tree) updateCache(key Key, flag []byte, proof [][]byte, inclusion bool, size uint8) {
	holder := newProofHolder([][]byte{flag}, [][][]byte{proof}, []bool{inclusion}, []uint8{size})
	k := key.String()
	//s.lruCache.Add(k, nil)
	t.cachedBranches[k] = holder
}

//func (t *tree) isSnapshot() bool {
//	return t.height == 0
//}

// Read takes the keys given and return the values from the database
// If the trusted flag is true, it is just a read from the database
// If trusted is false, then we check to see if the key exists in the trie
func (s *SMT) Read(keys [][]byte, trusted bool, commitment Commitment) ([][]byte, *proofHolder, error) {

	// check key sizes
	for _, k := range keys {
		if len(k) != s.keyByteSize {
			return nil, nil, fmt.Errorf("key size doesn't match the trie height: %x", k)
		}
	}

	flags := make([][]byte, len(keys))
	proofs := make([][][]byte, len(keys))
	inclusions := make([]bool, len(keys))
	sizes := make([]uint8, len(keys))

	//currRoot := s.GetRoot().value
	//stringRoot := root.String()

	tree, err := s.forest.Get(commitment)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid historical state: %w", err)
	}

	if !trusted {
		if s.IsSnapshot(tree) {

			//if bytes.Equal(root, currRoot) {
			for i, key := range keys {
				k := hex.EncodeToString(key)
				res := tree.cachedBranches[k]
				if res == nil {
					flag, proof, size, inclusion, err := s.GetProof(key, tree.rootNode)
					if err != nil {
						return nil, nil, fmt.Errorf("cannot get proof: %w", err)
					}
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

				tree.updateCache(key, flags[i], proofs[i], inclusions[i], sizes[i])
			}
		} else {

			for i, key := range keys {
				flag, proof, size, inclusion, err := s.GetHistoricalProof(key, commitment, tree.database)
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

	for i, key := range keys {
		res, err := tree.database.GetKVDB(key)
		if err != nil {

			localCommitment := commitment
			for {
				tree, err := s.forest.Get(localCommitment)
				if err != nil {
					return nil, nil, fmt.Errorf("cannot get tree (%s): %w", localCommitment, err)
				}

				res, err = tree.database.GetKVDB(key)
				if err == nil {
					break
				}
				if s.IsSnapshot(tree) {
					break
				}
				localCommitment = tree.previous
			}

			if res == nil && !errors.Is(err, databases.ErrNotFound) {
				return nil, nil, err
			}
		}
		values[i] = res
	}

	//}

	if trusted {
		return values, nil, nil
	}
	holder := newProofHolder(flags, proofs, inclusions, sizes)
	return values, holder, nil
}

// Print writes the structure of the tree to the stdout.
// Warning this should only be used for debugging
func (s *SMT) Print(c Commitment) error {
	t, err := s.forest.Get(c)
	if err != nil {
		return err
	}
	fmt.Println(t.rootNode.FmtStr("", ""))
	return nil
}

// verifyInclusionFlag is used to verify a flag against the trie given the key
func (s *SMT) verifyInclusionFlag(key []byte, flag []byte, commitment Commitment) (bool, error) {

	eflag := make([]byte, s.GetHeight()/8)

	tree, err := s.forest.Get(commitment)
	if err != nil {
		return false, fmt.Errorf("cannot get tree %s: %w", commitment, err)
	}
	curr := tree.rootNode
	if curr == nil || bytes.Equal(curr.key, key) {
		return bytes.Equal(eflag, flag), nil

	}

	var flagMatches bool

	for i := 0; i < len(flag); i++ {
		if utils.IsBitSet(key, i) {
			if curr.lChild != nil {
				flagMatches = utils.IsBitSet(flag, i)
				if !flagMatches {
					return false, nil
				}
			}

		} else {
			if curr.rChild != nil {
				flagMatches = utils.IsBitSet(flag, i)
				if !flagMatches {
					return false, nil
				}
			}

		}
	}

	return true, nil
}

func (s *SMT) GetBatchProof(keys [][]byte, commitment Commitment) (*proofHolder, error) {

	tree, err := s.forest.Get(commitment)
	if err != nil {
		return nil, fmt.Errorf("invalid historical state: %w", err)
	}

	flags := make([][]byte, len(keys))
	proofs := make([][][]byte, len(keys))
	inclusions := make([]bool, len(keys))
	sizes := make([]uint8, len(keys))

	notFoundKeys := make([][]byte, 0)
	values := make([][]byte, 0)

	for _, key := range keys {
		_, err := tree.database.GetKVDB(key)
		// TODO check the error
		if err != nil {
			notFoundKeys = append(notFoundKeys, key)
			values = append(values, []byte{})
		}
	}

	orgRoot, err := tree.Root()
	if err != nil {
		return nil, fmt.Errorf("cannot load tree root: %w", err)
	}
	treeRoot := orgRoot.deepCopy()

	if len(notFoundKeys) > 0 {
		batcher := tree.database.NewBatcher()
		// sort keys before update
		sort.Slice(notFoundKeys, func(i, j int) bool {
			return bytes.Compare(notFoundKeys[i], notFoundKeys[j]) < 0
		})

		treeRoot, err = s.UpdateAtomically(treeRoot, notFoundKeys, values, s.height-1, batcher, tree.database)
		if err != nil {
			return nil, err
		}
		// TODO assert new Root is the same
		// if bytes.Equal(new)
	}

	incl := true
	for i, key := range keys {
		flag := make([]byte, s.GetHeight()/8)
		proof := make([][]byte, 0)
		proofLen := uint8(0)

		curr := treeRoot

		for i := 0; i < s.GetHeight()-1; i++ {
			if bytes.Equal(curr.key, key) {
				break
			}
			if utils.IsBitSet(key, i) {
				if curr.lChild != nil {
					utils.SetBit(flag, i)
					proof = append(proof, curr.lChild.value)
				}
				curr = curr.rChild
				proofLen++
			} else {
				if curr.rChild != nil {
					utils.SetBit(flag, i)
					proof = append(proof, curr.rChild.value)
				}
				curr = curr.lChild
				proofLen++
			}
			if curr == nil {
				// TODO error ??
				incl = false
				break
			}

		}

		flags[i] = flag
		proofs[i] = proof
		inclusions[i] = incl
		sizes[i] = proofLen
	}
	holder := newProofHolder(flags, proofs, inclusions, sizes)
	return holder, nil
}

// GetProof searching the tree for a value if it exists, and returns the flag and then proof
func (s *SMT) GetProof(key []byte, rootNode *node) ([]byte, [][]byte, uint8, bool, error) {
	flag := make([]byte, s.GetHeight()/8) // Flag is used to save space by removing default hashesh (zeros) from the proofs
	proof := make([][]byte, 0)
	proofLen := uint8(0)

	curr := rootNode
	if curr == nil {
		return flag, proof, 0, false, fmt.Errorf("nil root")
	}
	if bytes.Equal(curr.key, key) {
		return flag, proof, proofLen, true, nil
	}

	var nextKey *node

	for i := 0; i < s.GetHeight()-1; i++ {
		if utils.IsBitSet(key, i) {
			if curr.lChild != nil {
				utils.SetBit(flag, i)
				proof = append(proof, curr.lChild.value)
			}

			nextKey = curr.rChild

		} else {
			if curr.rChild != nil {
				utils.SetBit(flag, i)
				proof = append(proof, curr.rChild.value)
			}

			nextKey = curr.lChild
		}
		if nextKey == nil { // nonInclusion proof
			return flag, proof, proofLen, false, nil
		} else {
			curr = nextKey
			proofLen++
			// inclusion proof
			if bytes.Equal(key, curr.key) {
				return flag, proof, proofLen, true, nil
			}
		}
	}

	if curr.key == nil {
		return flag, proof, proofLen, false, nil
	}
	return flag, proof, proofLen, true, nil
}

// getStateIndex returns the index of a stateroot in the historical state roots buffer
//func (s *SMT) getStateIndex(stateRoot string) int {
//	for i := 0; i < s.historicalStateRoots.Len(); i++ {
//		if fmt.Sprintf("%v", s.historicalStateRoots.At(i)) == stateRoot {
//			return i
//		}
//	}
//
//	return -1
//}

// GetHistoricalProof reconstructs a proof of inclusion or exclusion for a value in a historical database then returns the flag and proof
func (s *SMT) GetHistoricalProof(key []byte, commitment Commitment, database databases.DAL) ([]byte, [][]byte, uint8, bool, error) {
	flag := make([]byte, s.GetHeight()/8)
	proof := make([][]byte, 0)
	proofLen := uint8(0)

	curr, err := commitment.Root()
	if err != nil {
		return nil, nil, 0, false, errors.New("can't fetch the root")
	}
	if curr == nil {
		return flag, proof, 0, false, nil
	}

	var nextKey []byte

	for i := 0; i < s.GetHeight()-1; i++ {
		children, err := database.GetTrieDB(curr)
		if err != nil {

			localCommitment := commitment
			for {
				tree, err := s.forest.Get(localCommitment)
				if err != nil {
					return flag, proof, 0, false, fmt.Errorf("cannot get tree (%s): %w", localCommitment, err)
				}

				children, err = tree.database.GetTrieDB(curr)
				if err == nil {
					break
				}
				if s.IsSnapshot(tree) {
					break
				}
				localCommitment = tree.previous
			}

			if children == nil {
				return flag, proof, 0, false, err
			}
		}

		var Lchild []byte
		var Rchild []byte
		var Ckey []byte

		// the case where we are not at a leaf node!
		if len(children) == bothChildrenHashLen {
			// retrieve value for rootNode from the database, split into Left child value and Right child value
			// by splitting value slice in half!

			Lchild = children[0:hashLength]
			Rchild = children[hashLength:bothChildrenHashLen]
		} else if len(children) == childHashLen {
			// The case where the node we are pulling from the database has only one set child
			// check the first bit of the flag
			lr := utils.IsBitSet(children, 0)

			if lr {
				// If it is set the left child is nil
				Lchild = nilChild
				Rchild = children[1:childHashLen]

			} else {
				// If it is not set the right child is nil
				Lchild = children[1:childHashLen]
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

func (s *SMT) updateHistoricalStates(oldTree *tree, newTree *tree, batcher databases.Batcher) error {

	err := oldTree.database.CopyTo(newTree.database)
	if err != nil {
		return fmt.Errorf("error while copying DB: %s", err)
	}

	numStates := newTree.historicalStateCommitments.Len()
	switch {
	case numStates > s.numHistoricalStates:
		return errors.New("we have more Historical States stored than the Maximum Allowed Amount!")

	case numStates >= int(s.snapshotInterval):
		if newTree.height%s.snapshotInterval != 0 {
			stateToPrune := newTree.historicalStateCommitments.At(int(s.snapshotInterval)).(Commitment)
			referenceState := newTree.historicalStateCommitments.At(int(s.snapshotInterval - 1)).(Commitment)
			dbToPrune, err := s.forest.Get(stateToPrune)
			if err != nil {
				return fmt.Errorf("cannot get tree referenced in history: %w", err)
			}
			dbReference, err := s.forest.Get(referenceState)
			if err != nil {
				return fmt.Errorf("cannot get tree referenced in history: %w", err)
			}

			err = dbToPrune.database.PruneDB(dbReference.database)
			if err != nil {
				return err
			}
			_ = newTree.historicalStateCommitments.PopFront()

		}
	}

	height := make([]byte, 8)
	binary.BigEndian.PutUint64(height, newTree.height)

	batcher.Put(metadataKeyPrevious, oldTree.root)
	batcher.Put(metadataHeight, height)

	return nil
}

// Update takes a sorted list of keys and associated values and inserts
// them into the trie, and if that is successful updates the databases.
func (s *SMT) Update(keys [][]byte, values [][]byte, commitment Commitment) (Commitment, error) {

	t, err := s.forest.Get(commitment)
	if err != nil {
		return nil, fmt.Errorf("cannot get tree: %w", err)
	}

	// sort keys and deduplicate keys (we only consider the first occurance, and ignore the rest)
	sortedKeys := make([][]byte, 0)
	valueMap := make(map[string][]byte, 0)
	for i, key := range keys {
		// check key sizes
		if len(key) != s.keyByteSize {
			return nil, fmt.Errorf("key size doesn't match the trie height: %x", key)
		}
		// check if doesn't exist
		if _, ok := valueMap[string(key)]; !ok {
			//do something here
			sortedKeys = append(sortedKeys, key)
			valueMap[string(key)] = values[i]
			i++
		}
	}

	sort.Slice(sortedKeys, func(i, j int) bool {
		return bytes.Compare(sortedKeys[i], sortedKeys[j]) < 0
	})

	sortedValues := make([][]byte, 0)
	for _, key := range sortedKeys {
		sortedValues = append(sortedValues, valueMap[string(key)])
	}

	// use sorted keys
	keys = sortedKeys
	values = sortedValues

	batcher := t.database.NewBatcher()

	treeRoot, err := t.Root()
	if err != nil {
		return nil, fmt.Errorf("cannot load tree root: %w", err)
	}
	newRootNode, err := s.UpdateAtomically(treeRoot.deepCopy(), keys, values, s.height-1, batcher, t.database)

	if err != nil {
		return nil, err
	}
	nCom := newCommitment(t.Commitment(), newRootNode.value)

	db, err := s.forest.newDB(nCom)
	if err != nil {
		return nil, fmt.Errorf("cannot create new DB: %w", err)
	}

	var newHistoricalStateCommitments deque.Deque

	for i := 0; i < t.historicalStateCommitments.Len(); i++ {
		newHistoricalStateCommitments.PushBack(t.historicalStateCommitments.At(i))
	}
	newHistoricalStateCommitments.PushBack(nCom)

	newTree := &tree{
		root:                       newRootNode.value,
		rootNode:                   newRootNode,
		database:                   db,
		previous:                   t.Commitment(),
		cachedBranches:             make(map[string]*proofHolder),
		forest:                     s.forest,
		height:                     t.height + 1,
		historicalStateCommitments: newHistoricalStateCommitments,
	}

	err = s.updateHistoricalStates(t, newTree, batcher)
	if err != nil {
		return nil, err
	}

	err = db.UpdateTrieDB(batcher)
	if err != nil {
		return nil, err
	}

	err = db.UpdateKVDB(keys, values)
	if err != nil {
		return nil, err
	}

	s.forest.Add(newTree)
	return newTree.Commitment(), nil
}

// GetHeight returns the Height of the SMT
func (s *SMT) GetHeight() int {
	return s.height
}

func (s *SMT) insertIntoKeys(database databases.DAL, insert []byte, keys [][]byte, values [][]byte) ([][]byte, [][]byte, error) {
	// Initialize new slices, with capacity accounting for the inserted value
	newKeys := make([][]byte, 0, len(keys)+1)
	newValues := make([][]byte, 0, len(values)+1)

	for i, key := range keys {
		if bytes.Equal(insert, key) {
			return keys, values, nil
		}

		// Assuming the keys are presorted, this means we've found the spot in the slice to insert
		if bytes.Compare(insert, key) < 0 {
			// Insert the new key and remaining keys into the newKeys
			newKeys = append(newKeys, insert)
			newKeys = append(newKeys, keys[i:]...)

			// Insert the old value and remaining values into newValues
			oldVal, err := database.GetKVDB(insert)
			if err != nil {
				return nil, nil, err
			}
			newValues = append(newValues, oldVal)
			newValues = append(newValues, values[i:]...)

			return newKeys, newValues, nil
		}
		// Otherwise, append the key + value pair and loop
		newKeys = append(newKeys, keys[i])

		newValues = append(newValues, values[i])
	}

	// Did not find a spot for it in the slice, means it's place is at the end
	oldVal, err := database.GetKVDB(insert)
	if err != nil {
		return nil, nil, err
	}
	return append(newKeys, insert), append(newValues, oldVal), nil
}

// UpdateAtomically updates the trie atomically and returns the state rootNode
// NOTE: This function assumes keys and values are sorted and haves indexes mapping to each other
func (s *SMT) UpdateAtomically(rootNode *node, keys [][]byte, values [][]byte, height int, batcher databases.Batcher, database databases.DAL) (*node, error) {
	var err error
	if rootNode.value != nil {
		batcher.Put(rootNode.value, nil)
	}
	if rootNode.key != nil {
		keys, values, err = s.insertIntoKeys(database, rootNode.key, keys, values)
		if err != nil {
			return nil, err
		}
		//s.invalidateCache([][]byte{rootNode.key})
		rootNode.key = nil
	}

	if len(keys) != len(values) {
		return nil, errors.New("Total Key/Value Length mismatch")
	}

	// If we are at a leaf node, then we create said node
	if len(keys) == 1 && rootNode.lChild == nil && rootNode.rChild == nil {
		return s.ComputeRootNode(nil, nil, rootNode, keys, values, height, batcher), nil
	}

	//We initialize the nodes as empty to prevent nil pointer exceptions later
	lnode, rnode := rootNode.GetandSetChildren(GetDefaultHashes())

	// Split the keys and values array so we can update the trie in parallel
	lkeys, rkeys, splitIndex := utils.SplitKeys(keys, s.height-height-1)
	lvalues, rvalues := values[:splitIndex], values[splitIndex:]

	if len(lkeys) != len(lvalues) {
		return nil, errors.New("left Key/Value Length mismatch")
	}
	if len(rkeys) != len(rvalues) {
		return nil, errors.New("right Key/Value Length mismatch")
	}

	if len(rkeys) == 0 && len(lkeys) > 0 {
		// if we only have keys belonging on the left side of the trie to update
		return s.updateLeft(lnode, rnode, rootNode, lkeys, lvalues, height, batcher, database)
	} else if len(lkeys) == 0 && len(rkeys) > 0 {
		// if we only have keys belonging on the right side of the trie to update
		return s.updateRight(lnode, rnode, rootNode, rkeys, rvalues, height, batcher, database)
	} else if len(lkeys) > 0 && len(rkeys) > 0 {
		// update in parallel otherwise
		return s.updateParallel(lnode, rnode, rootNode, keys, values, lkeys, rkeys, lvalues, rvalues, height, batcher, database)
	}

	return rootNode, nil
}

// GetandSetChildren checks if any of the children are nill and creates them as a default node if they are, otherwise
// we just return the children
func (n *node) GetandSetChildren(hashes [257]Root) (*node, *node) {
	if n.lChild == nil {
		n.lChild = newNode(nil, n.height-1)
	}
	if n.rChild == nil {
		n.rChild = newNode(nil, n.height-1)
	}

	return n.lChild, n.rChild
}

// updateParallel updates both the left and right subtrees and computes a new rootNode
func (s *SMT) updateParallel(lnode *node, rnode *node, rootNode *node, keys [][]byte, values [][]byte, lkeys [][]byte, rkeys [][]byte, lvalues [][]byte, rvalues [][]byte, height int, batcher databases.Batcher, database databases.DAL) (*node, error) {
	lupdate, err1 := s.UpdateAtomically(lnode, lkeys, lvalues, height-1, batcher, database)
	rupdate, err2 := s.UpdateAtomically(rnode, rkeys, rvalues, height-1, batcher, database)

	if err1 != nil {
		return nil, err1
	}
	if err2 != nil {
		return nil, err2
	}

	return s.ComputeRootNode(lupdate, rupdate, rootNode, keys, values, height, batcher), nil
}

// updateLeft updates the left subtrees and computes a new rootNode
func (s *SMT) updateLeft(lnode *node, rnode *node, rootNode *node, lkeys [][]byte, lvalues [][]byte, height int, batcher databases.Batcher, database databases.DAL) (*node, error) {
	update, err := s.UpdateAtomically(lnode, lkeys, lvalues, height-1, batcher, database)
	if err != nil {
		return nil, err
	}
	return s.ComputeRootNode(update, rnode, rootNode, lkeys, lvalues, height, batcher), nil
}

// updateRight updates the right subtrees and computes a new rootNode
func (s *SMT) updateRight(lnode *node, rnode *node, rootNode *node, rkeys [][]byte, rvalues [][]byte, height int, batcher databases.Batcher, database databases.DAL) (*node, error) {
	update, err := s.UpdateAtomically(rnode, rkeys, rvalues, height-1, batcher, database)
	if err != nil {
		return nil, err
	}
	return s.ComputeRootNode(lnode, update, rootNode, rkeys, rvalues, height, batcher), nil
}

// ComputeRoot either returns a new leafNode or computes a new rootNode by hashing its children
func (s *SMT) ComputeRootNode(lnode *node, rnode *node, oldRootNode *node, keys [][]byte, values [][]byte, height int, batcher databases.Batcher) *node {
	if lnode == nil && rnode == nil {
		ln := newNode(ComputeCompactValue(keys[0], values[0], height, s.height), height)
		ln.key = keys[0]
		batcher.Put(ln.value, ln.key)
		return ln
	} else {
		return interiorNode(lnode, rnode, height, batcher)
	}
}

// interiorNode computes the new node's Hash by hashing it's children's nodes, and also cleans up any default nodes
// that are not needed
func interiorNode(lnode *node, rnode *node, height int, batcher databases.Batcher) *node {

	// If any nodes are default nodes, they are no longer needed and can be discarded
	if lnode != nil && bytes.Equal(lnode.value, nil) {
		lnode = nil
	}
	if rnode != nil && bytes.Equal(rnode.value, nil) {
		rnode = nil
	}

	// Hashes the children depending on if they are nil or filled
	if (lnode != nil) && (rnode != nil) {
		in := newNode(HashInterNode(lnode.value, rnode.value), height)
		in.lChild = lnode
		in.rChild = rnode
		batcher.Put(in.value, append(in.lChild.value, in.rChild.value...))
		return in
	} else if lnode == nil && rnode != nil {
		in := newNode(HashInterNode(GetDefaultHashForHeight(height-1), rnode.value), height)
		in.lChild = lnode
		in.rChild = rnode
		// if the left node is nil value of the rChild attached to key in DB will be prefaced by
		// 4 bits with the first bit set
		lFlag := make([]byte, 1)
		utils.SetBit(lFlag, 0)
		batcher.Put(in.value, append(lFlag, in.rChild.value...))
		return in
	} else if rnode == nil && lnode != nil {
		in := newNode(HashInterNode(lnode.value, GetDefaultHashForHeight(height-1)), height)
		in.lChild = lnode
		in.rChild = rnode
		rFlag := make([]byte, 1)
		// if the right node is nil value of the lChild attached to key in DB will be prefaced by
		// 4 unset bits
		batcher.Put(in.value, append(rFlag, in.lChild.value...))
		return in
	}
	return nil
}

// SafeClose is an exported function to safely close the databases
func (s *SMT) SafeClose() {

	// Purge calls eviction method, which closes the DB
	s.forest.cache.Purge()
	//s.forest.fullCache.Purge()
}

func (s *SMT) IsSnapshot(t *tree) bool {
	return t.height%s.snapshotInterval == 0
}

// Size returns the size of the forest on disk in bytes
func (s *SMT) Size() (int64, error) {
	return s.forest.Size()
}

// ComputeCompactValue computes the value for the node considering the sub tree to only include this value and default values.
func ComputeCompactValue(key []byte, value []byte, height int, maxHeight int) []byte {
	// if value is nil return default hash
	if len(value) == 0 {
		return GetDefaultHashForHeight(height)
	}
	computedHash := HashLeaf(key, value)

	for j := maxHeight - 2; j > maxHeight-height-2; j-- {
		if utils.IsBitSet(key, j) { // right branching
			computedHash = HashInterNode(GetDefaultHashForHeight(maxHeight-j-2), computedHash)
		} else { // left branching
			computedHash = HashInterNode(computedHash, GetDefaultHashForHeight(maxHeight-j-2))
		}
	}
	return computedHash
}
