package ledger

import "github.com/onflow/flow-go/ledger/common/hash"

type Storage interface {
	Get([]byte) ([]byte, error)
	SetMul(keys [][]byte, values [][]byte) error
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
