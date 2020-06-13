package flattener

type StorableNode struct {
	LIndex    uint64
	RIndex    uint64
	Height    uint16 // Height where the node is at
	Key       []byte
	Value     []byte
	HashValue []byte
}

// StorableTrie is a data structure for storing trie
type StorableTrie struct {
	RootIndex      uint64
	Number         uint64
	RootHash       []byte
	ParentRootHash []byte
}
