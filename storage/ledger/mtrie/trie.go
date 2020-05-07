package mtrie

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// MTrie is a fully in memory trie with option to persist to disk
type MTrie struct {
	root           *node `json:"root"`
	parent         *MTrie
	values         map[string][]byte
	rootHash       []byte `json:"rootHash"`
	parentRootHash []byte `json:"parentRootHash"`
	lock           sync.Mutex
}

// SetParent sets parents for this trie
func (mt *MTrie) SetParent(pt *MTrie) {
	mt.parent = pt
	mt.parentRootHash = pt.rootHash
}

// Persist persist
func (mt *MTrie) Persist(path string) error {
	mt.lock.Lock()
	defer mt.lock.Unlock()

	file, err := json.MarshalIndent(mt, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(path, hex.EncodeToString(mt.rootHash)+".json"), file, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (mt *MTrie) Load(path string) error {
	mt.lock.Lock()
	defer mt.lock.Unlock()

	fpath := filepath.Join(path, hex.EncodeToString(mt.rootHash)+".json")
	// check path exist
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		return err
	}

	f, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(f), mt)
	if err != nil {
		return err
	}
	return nil
}

// NewMTrie returns the same root
func NewMTrie(root *node) *MTrie {
	return &MTrie{root: root, values: make(map[string][]byte)}
}
