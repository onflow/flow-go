package trie

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"

	lru "github.com/hashicorp/golang-lru"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	"github.com/dapperlabs/flow-go/utils/io"
)

type Commitment []byte

func (c Commitment) Root() (Root, error) {
	if len(c) < 64 {
		return nil, errors.New("commitment length is not enough")
	}
	// root portion of commitment
	return Root(c[64:]), nil
}

func (c Commitment) String() string {
	return hex.EncodeToString(c)
}

func newCommitment(prevCom Commitment, newRoot Root) Commitment {
	commitment := make([]byte, 0)
	// compute the hash of previous root
	hasher := hash.NewSHA3_256()
	_, err := hasher.Write(prevCom)
	if err != nil {
		panic(err)
	}
	prevComHash := []byte(hasher.SumHash())
	commitment = append(commitment, prevComHash...)
	// append the current root
	commitment = append(commitment, newRoot...)
	return Commitment(commitment)
}

type forest struct {
	dbDir  string
	cache  *lru.Cache
	height int
}

func NewForest(dbDir string, maxSize int, maxFullSize int, height int) (*forest, error) {
	evict := func(key interface{}, value interface{}) {
		tree, ok := value.(*tree)
		if !ok {
			panic(fmt.Sprintf("cache contains item of type %T", value))
		}
		_, _ = tree.database.SafeClose()
	}
	cache, err := lru.NewWithEvict(maxSize, evict)
	if err != nil {
		return nil, fmt.Errorf("cannot create forest cache: %w", err)
	}
	//fullCache, err := lru.NewWithEvict(maxFullSize, evict)
	//if err != nil {
	//	return nil, fmt.Errorf("cannot create forest full cache: %w", err)
	//}
	return &forest{
		dbDir: dbDir,
		cache: cache,
		//fullCache: fullCache,
		height: height,
	}, nil
}

func (f *forest) Add(tree *tree) {
	//if tree.numDeltas == 0 {
	//	f.fullCache.Add(tree.root.String(), tree)
	//	return
	//}
	f.cache.Add(tree.Commitment().String(), tree)
}

func (f *forest) Get(cm Commitment) (*tree, error) {

	if foundTree, ok := f.cache.Get(cm.String()); ok {
		return foundTree.(*tree), nil
	}

	db, err := f.newDB(cm)
	if err != nil {
		return nil, fmt.Errorf("cannot create LevelDB: %w", err)
	}

	root, err := cm.Root()
	if err != nil {
		return nil, fmt.Errorf("cannot get root from commitment: %w", err)
	}

	tree, err := f.newTree(root, db)
	if err != nil {
		return nil, fmt.Errorf("cannot create tree: %w", err)
	}

	f.Add(tree)

	return tree, nil
}

func (f *forest) newDB(commitment []byte) (*leveldb.LevelDB, error) {
	treePath := filepath.Join(f.dbDir, hex.EncodeToString(commitment))

	db, err := leveldb.NewLevelDB(treePath)
	return db, err
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

// Size returns the size of the forest on disk in bytes
func (f *forest) Size() (int64, error) {
	return io.DirSize(f.dbDir)
}

// PrintCache prints all trees inside the cache
func (f forest) PrintCache() {
	for _, key := range f.cache.Keys() {
		commitment, _ := hex.DecodeString(key.(string))
		t, _ := f.Get(commitment)
		fmt.Println(key, t.rootNode.FmtStr("", ""))
	}
}
