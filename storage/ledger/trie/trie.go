package trie

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/gammazero/deque"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/io"
)

var nilChild = make([]byte, 32)

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
	previous       Root
	cachedBranches map[string]*proofHolder // Map of string representations of keys to proofs
	forest         *forest
	height         uint64 // similar to block height, useful for snapshotting
	// FIFO queue of ordered historical State Roots
	// In-memory only, used for tracking snapshots. Can be reconstructed from disk if needed
	// Frequent eviction of a tree/restarting might mean database not gets pruned as frequent
	historicalStateRoots deque.Deque
	isLocked             bool
}

func (t *tree) init() error {
	batcher := t.database.NewBatcher()

	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, t.height)

	batcher.Put(metadataKeyPrevious, []byte{})
	batcher.Put(metadataHeight, heightBytes)

	return t.database.UpdateTrieDB(batcher)
}

func (t *tree) Lock() {
	t.isLocked = true
}

func (t *tree) Unlock() {
	t.isLocked = false
}

func (t *tree) IsLocked() bool {
	return t.isLocked
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

func (f *forest) newTree(root Root, db databases.DAL) (*tree, error) {

	deltaNumber, err := db.GetTrieDB(metadataHeight)
	if err != nil {
		return nil, fmt.Errorf("cannot get delta number: %w", err)
	}

	previousRoot, err := db.GetTrieDB(metadataKeyPrevious)
	if err != nil {
		return nil, fmt.Errorf("cannot get previous root: %w", err)
	}

	return &tree{
		rootNode:       nil,
		root:           root,
		database:       db,
		previous:       previousRoot,
		height:         binary.BigEndian.Uint64(deltaNumber),
		cachedBranches: make(map[string]*proofHolder),
		forest:         f,
	}, nil
}

type forest struct {
	dbDir  string
	cache  treeCache
	height int
}

func NewForest(dbDir string, cacheSize, height int) (*forest, error) {
	cache, err := newLRUTreeCache(cacheSize)
	if err != nil {
		return nil, fmt.Errorf("cannot create forest cache: %w", err)
	}

	return &forest{
		dbDir:  dbDir,
		cache:  cache,
		height: height,
	}, nil
}

func (f *forest) Add(t *tree) {
	// if DB is already here, safe close it before overriding
	if foundTree, ok := f.cache.Get(t.root); ok {
		_, _ = foundTree.database.SafeClose()
	}

	f.cache.Add(t.root, t)
}

func (f *forest) Has(root Root) bool {
	return f.cache.Contains(root)
}

func (f *forest) Get(root Root) (*tree, error) {

	if foundTree, ok := f.cache.Get(root); ok {
		return foundTree, nil
	}

	db, err := f.newDB(root)
	if err != nil {
		return nil, fmt.Errorf("cannot create LevelDB: %w", err)
	}

	tree, err := f.newTree(root, db)
	if err != nil {
		return nil, fmt.Errorf("cannot create tree: %w", err)
	}

	f.Add(tree)

	return tree, nil
}

func (f *forest) newDB(root Root) (databases.DAL, error) {
	treePath := filepath.Join(f.dbDir, root.String())

	db, err := leveldb.NewLevelDB(treePath)
	return db, err
}

// Size returns the size of the forest on disk in bytes
func (f *forest) Size() (int64, error) {
	return io.DirSize(f.dbDir)
}

// SMT is a Basic Sparse Merkle Tree struct
type SMT struct {
	forest              *forest
	height              int    // Height of the tree
	keyByteSize         int    // acceptable number of bytes for key
	numHistoricalStates int    // Number of states to keep in historicalStates
	snapshotInterval    uint64 // When removing full states from historical states interval between full states
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
	cacheSize int,
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

	s.height = height
	s.keyByteSize = (height - 1) / 8

	s.numHistoricalStates = numHistoricalStates
	s.snapshotInterval = interval

	forest, err := NewForest(dbDir, cacheSize, height)
	if err != nil {
		return nil, fmt.Errorf("cannot create forest: %w", err)
	}

	// add empty tree
	emptyHash := GetDefaultHashForHeight(height - 1)
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

	err = newTree.init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize base tree: %w", err)
	}

	forest.Add(newTree)

	s.forest = forest

	return s, nil
}

// proofHolder is a struct that holds the proofs and flags from a proof check
type proofHolder struct {
	flags      [][]byte   // The flags of the proofs (is set if an intermediate node has a non-default)
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

func (p proofHolder) String() string {
	res := fmt.Sprintf("> proof holder includes %d proofs\n", len(p.sizes))
	for i, size := range p.sizes {
		flags := p.flags[i]
		proof := p.proofs[i]
		flagStr := ""
		for _, f := range flags {
			flagStr += fmt.Sprintf("%08b", f)
		}
		proofStr := fmt.Sprintf("size: %d flags: %v\n", size, flagStr)
		if p.inclusions[i] {
			proofStr += fmt.Sprintf("\t proof %d (inclusion)\n", i)
		} else {
			proofStr += fmt.Sprintf("\t proof %d (noninclusion)\n", i)
		}
		proofIndex := 0
		for j := 0; j < int(size); j++ {
			if utils.IsBitSet(flags, j) {
				proofStr += fmt.Sprintf("\t\t %d: [%x]\n", j, proof[proofIndex])
				proofIndex++
			}
		}
		res = res + "\n" + proofStr
	}
	return res
}

// updateCache takes a key and all relevant parts of the proof and inserts in into the cache, removing values if needed
func (t *tree) updateCache(key Key, flag []byte, proof [][]byte, inclusion bool, size uint8) {
	holder := newProofHolder([][]byte{flag}, [][][]byte{proof}, []bool{inclusion}, []uint8{size})
	k := key.String()
	t.cachedBranches[k] = holder
}

// Read takes the keys given and return the values from the database
// If the trusted flag is true, it is just a read from the database
// If trusted is false, then we check to see if the key exists in the trie
func (s *SMT) Read(keys [][]byte, trusted bool, root Root) ([][]byte, *proofHolder, error) {

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

	tree, err := s.forest.Get(root)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid historical state: %w", err)
	}

	// lock tree to prevent cache eviction
	tree.Lock()
	defer tree.Unlock()

	if !trusted {
		if s.IsSnapshot(tree) {
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
				flag, proof, size, inclusion, err := s.GetHistoricalProof(key, root, tree.database)
				if err != nil {
					return nil, nil, fmt.Errorf("cannot get historical proof: %w", err)
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
		res, err1 := tree.database.GetKVDB(key)
		if err != nil {

			localRoot := root
			for {
				tree, err := s.forest.Get(localRoot)
				if err != nil {
					return nil, nil, fmt.Errorf("cannot get tree (%s): %w", localRoot, err)
				}

				res, err = tree.database.GetKVDB(key)
				if err == nil {
					break
				} else if !errors.Is(err, databases.ErrNotFound) {
					return nil, nil, fmt.Errorf("unexpected err (%s): %w", localRoot, err)
				}

				if s.IsSnapshot(tree) {
					break
				}

				localRoot = tree.previous
			}

			if res == nil && !errors.Is(err1, databases.ErrNotFound) {
				return nil, nil, fmt.Errorf("cannot get key (%s): %w", tree.root, err)
			}
		}
		values[i] = res
	}

	if trusted {
		return values, nil, nil
	}
	holder := newProofHolder(flags, proofs, inclusions, sizes)
	return values, holder, nil
}

// Print writes the structure of the tree to the stdout.
//
// Warning: this function should only be used for debugging.
func (s *SMT) Print(root Root) error {
	t, err := s.forest.Get(root)
	if err != nil {
		return err
	}
	fmt.Println(t.rootNode.FmtStr("", ""))
	return nil
}

// verifyInclusionFlag is used to verify a flag against the trie given the key
func (s *SMT) verifyInclusionFlag(key []byte, flag []byte, root Root) (bool, error) {

	eflag := make([]byte, s.GetHeight()/8)

	tree, err := s.forest.Get(root)
	if err != nil {
		return false, fmt.Errorf("cannot get tree %s: %w", root, err)
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

// GetBatchProof returns proof for a batch of keys.
//
// no duplicated keys are allowed for this method
// split key with trie has some issues.
func (s *SMT) GetBatchProof(keys [][]byte, root Root) (*proofHolder, error) {

	tree, err := s.forest.Get(root)
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

// GetHistoricalProof reconstructs a proof of inclusion or exclusion for a value in a historical database then returns the flag and proof
func (s *SMT) GetHistoricalProof(key []byte, root Root, database databases.DAL) ([]byte, [][]byte, uint8, bool, error) {
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

			localRoot := root
			for {
				tree, err := s.forest.Get(localRoot)
				if err != nil {
					return flag, proof, 0, false, fmt.Errorf("cannot get tree (%s): %w", localRoot, err)
				}

				children, err = tree.database.GetTrieDB(curr)
				if err == nil {
					break
				}
				if s.IsSnapshot(tree) {
					break
				}
				localRoot = tree.previous
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

// VerifyInclusionProof calculates the inclusion proof from a given rootNode, flag, proof list, and size.
//
// This function is exclusively for inclusive proofs
func VerifyInclusionProof(key []byte, value []byte, flag []byte, proof [][]byte, size uint8, root []byte, height int) bool {
	// get index of proof we start our calculations from
	proofIndex := 0

	if len(proof) != 0 {
		proofIndex = len(proof) - 1
	}
	// base case at the bottom of the trie
	computed := ComputeCompactValue(key, value, height-int(size)-1, height)
	for i := int(size) - 1; i > -1; i-- {
		// hashing is order dependant
		if utils.IsBitSet(key, i) {
			if !utils.IsBitSet(flag, i) {
				computed = HashInterNode(GetDefaultHashForHeight((height-i)-2), computed)
			} else {
				computed = HashInterNode(proof[proofIndex], computed)
				proofIndex--
			}
		} else {
			if !utils.IsBitSet(flag, i) {
				computed = HashInterNode(computed, GetDefaultHashForHeight((height-i)-2))
			} else {
				computed = HashInterNode(computed, proof[proofIndex])
				proofIndex--
			}
		}
	}
	return bytes.Equal(computed, root)
}

func VerifyNonInclusionProof(key []byte, value []byte, flag []byte, proof [][]byte, size uint8, root []byte, height int) bool {
	// get index of proof we start our calculations from
	proofIndex := 0

	if len(proof) != 0 {
		proofIndex = len(proof) - 1
	}
	// base case at the bottom of the trie
	computed := ComputeCompactValue(key, value, height-int(size)-1, height)
	for i := int(size) - 1; i > -1; i-- {
		// hashing is order dependant
		if utils.IsBitSet(key, i) {
			if !utils.IsBitSet(flag, i) {
				computed = HashInterNode(GetDefaultHashForHeight((height-i)-2), computed)
			} else {
				computed = HashInterNode(proof[proofIndex], computed)
				proofIndex--
			}
		} else {
			if !utils.IsBitSet(flag, i) {
				computed = HashInterNode(computed, GetDefaultHashForHeight((height-i)-2))
			} else {
				computed = HashInterNode(computed, proof[proofIndex])
				proofIndex--
			}
		}
	}
	return !bytes.Equal(computed, root)
}

func (s *SMT) updateHistoricalStates(oldTree *tree, newTree *tree, batcher databases.Batcher) error {

	err := oldTree.database.CopyTo(newTree.database)
	if err != nil {
		return fmt.Errorf("error while copying DB: %s", err)
	}
	numStates := newTree.historicalStateRoots.Len()
	switch {
	case numStates > s.numHistoricalStates:
		return errors.New("we have more Historical States stored than the Maximum Allowed Amount!")

	case numStates >= int(s.snapshotInterval):
		if newTree.height%s.snapshotInterval != 0 {
			stateToPrune := newTree.historicalStateRoots.At(int(s.snapshotInterval) - 1).(Root)
			referenceState := newTree.historicalStateRoots.At(int(s.snapshotInterval - 2)).(Root)
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
			_ = newTree.historicalStateRoots.PopFront()

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
func (s *SMT) Update(keys [][]byte, values [][]byte, root Root) (Root, error) {
	// if no update
	if len(keys) < 1 {
		return root, nil
	}

	t, err := s.forest.Get(root)
	if err != nil {
		return nil, fmt.Errorf("cannot get tree: %w", err)
	}

	// sort keys and deduplicate keys (we only consider the first occurance, and ignore the rest)
	sortedKeys := make([][]byte, 0)
	valueMap := make(map[string][]byte)
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

	treeRoot, err := t.Root()
	if err != nil {
		return nil, fmt.Errorf("cannot load tree root: %w", err)
	}

	newTreeRoot := treeRoot.deepCopy()
	batcher := t.database.NewBatcher()
	newRootNode, err := s.UpdateAtomically(newTreeRoot, keys, values, s.height-1, batcher, t.database)

	if err != nil {
		return nil, err
	}

	// TODO improve this in case of getting an state which has been there before
	if s.forest.Has(newRootNode.value) {
		return newRootNode.value, nil
	}

	db, err := leveldb.NewLevelDB(filepath.Join(s.forest.dbDir, newRootNode.value.String()))
	if err != nil {
		return nil, fmt.Errorf("cannot create new DB: %w", err)
	}

	var newHistoricalStatRoots deque.Deque
	for i := 0; i < t.historicalStateRoots.Len(); i++ {
		newHistoricalStatRoots.PushBack(t.historicalStateRoots.At(i))
	}

	newHistoricalStatRoots.PushBack(newRootNode.value)

	newTree := &tree{
		root:                 newRootNode.value,
		rootNode:             newRootNode,
		database:             db,
		previous:             root,
		cachedBranches:       make(map[string]*proofHolder),
		forest:               s.forest,
		height:               t.height + 1,
		historicalStateRoots: newHistoricalStatRoots,
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
	return newRootNode.value, nil
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
//
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
		rootNode.key = nil
	}

	if len(keys) != len(values) {
		return nil, errors.New("Total Key/Value Length mismatch")
	}

	// If we are at a leaf node, then we create said node
	if len(keys) == 1 && rootNode.lChild == nil && rootNode.rChild == nil {
		return s.ComputeRootNode(nil, nil, rootNode, keys, values, height, batcher), nil
	}

	// We initialize the nodes as empty to prevent nil pointer exceptions later
	lnode, rnode := rootNode.GetAndSetChildren(GetDefaultHashes())

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

// GetAndSetChildren checks if any of the children are nil and creates them as a default node if they are, otherwise
// we just return the children
func (n *node) GetAndSetChildren(hashes [257]Root) (*node, *node) {
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

// EncodeProof encodes a proof holder into an array of byte arrays
// The following code is the encoding logic
// Each slice in the proofHolder is stored as a byte array, and the whole thing is stored
// as a [][]byte
// First we have a byte, and set the first bit to 1 if it is an inclusion proof
// Then the size is encoded as a single byte
// Then the flag is encoded (size is defined by size)
// Finally the proofs are encoded one at a time, and is stored as a byte array
func EncodeProof(pholder *proofHolder) [][]byte {
	proofs := make([][]byte, 0)
	for i := 0; i < pholder.GetSize(); i++ {
		flag, singleProof, inclusion, size := pholder.ExportProof(i)
		byteSize := []byte{size}
		byteInclusion := make([]byte, 1)
		if inclusion {
			utils.SetBit(byteInclusion, 0)
		}
		proof := append(byteInclusion, byteSize...)

		flagSize := []byte{uint8(len(flag))}
		proof = append(proof, flagSize...)
		proof = append(proof, flag...)

		for _, p := range singleProof {
			proof = append(proof, p...)
		}
		// ledgerStorage is a struct that holds our SM
		proofs = append(proofs, proof)
	}
	return proofs
}

// DecodeProof takes in an encodes array of byte arrays an converts them into a proofHolder
func DecodeProof(proofs [][]byte) (*proofHolder, error) {
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
		if len(proof) < 4 {
			return nil, fmt.Errorf("error decoding the proof: proof size too small")
		}
		byteInclusion := proof[0:1]
		inclusion := utils.IsBitSet(byteInclusion, 0)
		inclusions = append(inclusions, inclusion)
		size := proof[1:2]
		sizes = append(sizes, size...)
		flagSize := int(proof[2])
		if flagSize < 1 {
			return nil, fmt.Errorf("error decoding the proof: flag size should be greater than 0")
		}
		flags = append(flags, proof[3:flagSize+3])
		byteProofs := make([][]byte, 0)
		for i := flagSize + 3; i < len(proof); i += 32 {
			// TODO understand the logic here
			if i+32 <= len(proof) {
				byteProofs = append(byteProofs, proof[i:i+32])
			} else {
				byteProofs = append(byteProofs, proof[i:])
			}
		}
		newProofs = append(newProofs, byteProofs)
	}

	return newProofHolder(flags, newProofs, inclusions, sizes), nil
}
