package badger

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/model/flow"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
)

func handleError(err error, t interface{}) error {
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return storage.ErrNotFound
		}

		return fmt.Errorf("could not retrieve %T: %w", t, err)
	}
	return nil
}

func KeyFromBlockIDIndex(blockID flow.Identifier, txIndex uint32) string {
	idData := make([]byte, 4) //uint32 fits into 4 bytes
	binary.BigEndian.PutUint32(idData, txIndex)
	return fmt.Sprintf("%x%x", blockID, idData)
}

func KeyToBlockIDIndex(key string) (flow.Identifier, uint32, error) {
	blockIDStr := key[:64]
	indexStr := key[64:]
	blockID, err := flow.HexStringToIdentifier(blockIDStr)
	if err != nil {
		return flow.ZeroID, 0, fmt.Errorf("could not get block ID: %w", err)
	}

	txIndexBytes, err := hex.DecodeString(indexStr)
	if err != nil {
		return flow.ZeroID, 0, fmt.Errorf("could not get transaction index: %w", err)
	}
	if len(txIndexBytes) != 4 {
		return flow.ZeroID, 0, fmt.Errorf("could not get transaction index - invalid length: %d", len(txIndexBytes))
	}

	txIndex := binary.BigEndian.Uint32(txIndexBytes)

	return blockID, txIndex, nil
}
