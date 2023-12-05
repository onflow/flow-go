package database

import (
	"github.com/ethereum/go-ethereum/common"
	gethCommon "github.com/ethereum/go-ethereum/common"
	gethState "github.com/ethereum/go-ethereum/core/state"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"

	"github.com/onflow/flow-go/fvm/evm/types"
)

const (
	accountPrefix  = "A?"
	storagePrefix  = "S?"
	contractPrefix = "C?"
)

type noTrie struct {
	kvdb types.Database
}

var _ gethState.Trie = &noTrie{}

func (t *noTrie) GetKey(key []byte) []byte {
	v, err := t.kvdb.Get(key)
	if err != nil {
		panic(err)
	}
	return v
}

func (t *noTrie) GetAccount(address gethCommon.Address) (*gethTypes.StateAccount, error) {
	v, err := t.kvdb.Get([]byte(accountPrefix + address.Hex()))
	if err != nil {
		return nil, err
	}
	var as *gethTypes.StateAccount
	err = rlp.DecodeBytes(v, as)
	if err != nil {
		return nil, err
	}
	return as, nil
}

func (t *noTrie) GetStorage(addr gethCommon.Address, key []byte) ([]byte, error) {
	return t.kvdb.Get([]byte(storagePrefix + addr.Hex() + string(key)))
}

func (t *noTrie) UpdateAccount(address gethCommon.Address, account *gethTypes.StateAccount) error {
	encoded, err := rlp.EncodeToBytes(account)
	if err != nil {
		return err
	}
	return t.kvdb.Put([]byte(accountPrefix+address.Hex()), encoded)
}

func (t *noTrie) UpdateStorage(addr gethCommon.Address, key, value []byte) error {
	return t.kvdb.Put([]byte(storagePrefix+addr.Hex()+string(key)), value)
}

func (t *noTrie) DeleteAccount(address gethCommon.Address) error {
	return t.kvdb.Delete([]byte(accountPrefix + address.Hex()))
}

func (t *noTrie) DeleteStorage(addr gethCommon.Address, key []byte) error {
	return t.kvdb.Delete([]byte(storagePrefix + addr.Hex() + string(key)))
}

func (t *noTrie) GetContractCode(address, codeHash gethCommon.Hash) ([]byte, error) {
	return t.kvdb.Get([]byte(contractPrefix + address.String() + codeHash.String()))
}

func (t *noTrie) UpdateContractCode(address gethCommon.Address, codeHash gethCommon.Hash, code []byte) error {
	return t.kvdb.Put([]byte(contractPrefix+address.Hash().String()+codeHash.String()), code)
}

func (t *noTrie) Hash() gethCommon.Hash {
	return gethCommon.Hash{}
}

func (t *noTrie) Commit(collectLeaf bool) (gethCommon.Hash, *trienode.NodeSet) {
	return gethCommon.Hash{}, nil
}

func (t *noTrie) NodeIterator(startKey []byte) trie.NodeIterator {
	return nil
}

func (t *noTrie) Prove(key []byte, fromLevel uint, proofDb ethdb.KeyValueWriter) error {
	return nil
}

type TrieDatabase struct {
	trie *noTrie
}

func NewTrieDatabase(kvdb types.Database) *TrieDatabase {
	return &TrieDatabase{&noTrie{kvdb}}
}

func (db TrieDatabase) OpenTrie(root common.Hash) (gethState.Trie, error) {
	return db.trie, nil
}

func (db TrieDatabase) OpenStorageTrie(stateRoot common.Hash, addrHash, root common.Hash) (gethState.Trie, error) {
	return db.trie, nil
}

func (db TrieDatabase) CopyTrie(gethState.Trie) gethState.Trie {
	panic("not implemented")
}

func (db TrieDatabase) ContractCode(addr, codeHash common.Hash) ([]byte, error) {
	return db.trie.GetContractCode(addr, codeHash)
}

func (db TrieDatabase) ContractCodeSize(addr, codeHash common.Hash) (int, error) {
	// TODO: optimize me
	code, err := db.trie.GetContractCode(addr, codeHash)
	return len(code), err
}

func (db TrieDatabase) DiskDB() ethdb.KeyValueStore {
	return db.trie.kvdb
}

func (db TrieDatabase) TrieDB() *trie.Database {
	panic("not implemented")
}
