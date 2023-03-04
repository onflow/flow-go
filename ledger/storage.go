package ledger

import "github.com/onflow/flow-go/ledger/common/hash"

type Storage interface {
	Get(hash.Hash) ([]byte, error)
	GetMul([]hash.Hash) ([][]byte, error)
	SetMul(keyValuePairs map[hash.Hash][]byte) error
	Close() error
}

type LeafNode struct {
	Hash    hash.Hash
	Path    Path
	Payload Payload
}

type PayloadStorage interface {
	Get(hash.Hash) (Path, *Payload, error)
	Add([]LeafNode) error
}
