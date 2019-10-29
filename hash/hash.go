package hash

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/encoding"
	"github.com/ethereum/go-ethereum/core/types"
)

var DefaultHasher, _ = crypto.NewHasher(crypto.SHA3_256)

func SetTransactionHash(tx *types.Transaction) error {
	b, err := encoding.DefaultEncoder.EncodeTransaction(*tx)
	if err != nil {
		return err
	}

	hash := DefaultHasher.ComputeHash(b)

	tx.Hash = hash

	return nil
}

func SetChunkHash(c *types.Chunk) error {
	b, err := encoding.DefaultEncoder.EncodeChunk(*c)
	if err != nil {
		return err
	}

	hash := DefaultHasher.ComputeHash(b)

	c.Hash = hash

	return nil
}
