package storage

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

func KeyFromBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) string {
	return fmt.Sprintf("%x%x", blockID, txID)
}

func KeyFromBlockIDIndex(blockID flow.Identifier, txIndex uint32) string {
	idData := make([]byte, 4) //uint32 fits into 4 bytes
	binary.BigEndian.PutUint32(idData, txIndex)
	return fmt.Sprintf("%x%x", blockID, idData)
}

func KeyFromBlockID(blockID flow.Identifier) string {
	return blockID.String()
}

func KeyToBlockIDTransactionID(key string) (flow.Identifier, flow.Identifier, error) {
	blockIDStr := key[:64]
	txIDStr := key[64:]
	blockID, err := flow.HexStringToIdentifier(blockIDStr)
	if err != nil {
		return flow.ZeroID, flow.ZeroID, fmt.Errorf("could not get block ID: %w", err)
	}

	txID, err := flow.HexStringToIdentifier(txIDStr)
	if err != nil {
		return flow.ZeroID, flow.ZeroID, fmt.Errorf("could not get transaction id: %w", err)
	}

	return blockID, txID, nil
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

func KeyToBlockID(key string) (flow.Identifier, error) {

	blockID, err := flow.HexStringToIdentifier(key)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get block ID: %w", err)
	}

	return blockID, err
}
