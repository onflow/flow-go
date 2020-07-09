package flattener

type StorableNode struct {
	LIndex     uint64
	RIndex     uint64
	Height     uint16 // Height where the node is at
	Path       []byte
	EncPayload []byte // encoded data for payload
	HashValue  []byte
	MaxDepth   uint16
	RegCount   uint64
}

// StorableTrie is a data structure for storing trie
type StorableTrie struct {
	RootIndex uint64
	RootHash  []byte
}
