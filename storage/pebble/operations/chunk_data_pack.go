package operations

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertChunkDataPack(sc *storage.StoredChunkDataPack) func(w pebble.Writer) error {
	key := makeKey(codeChunkDataPack, sc.ChunkID)
	return insert(key, sc)
}

func RetrieveChunkDataPack(chunkID flow.Identifier, sc *storage.StoredChunkDataPack) func(r pebble.Reader) error {
	key := makeKey(codeChunkDataPack, chunkID)
	return retrieve(key, sc)
}

func RemoveChunkDataPack(chunkID flow.Identifier) func(w pebble.Writer) error {
	key := makeKey(codeChunkDataPack, chunkID)
	return func(w pebble.Writer) error {
		return w.Delete(key, nil)
	}
}
