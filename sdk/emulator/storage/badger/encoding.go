package badger

import (
	"bytes"
	"encoding/gob"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
)

func init() {
	gob.Register(flow.Address{})
}

func encodeTransaction(tx flow.Transaction) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&tx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeTransaction(tx *flow.Transaction, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(tx)
}

func encodeBlock(block types.Block) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&block); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeBlock(block *types.Block, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(block)
}

func encodeRegisters(registers flow.Registers) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&registers); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeRegisters(registers *flow.Registers, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(registers)
}

func encodeEventList(eventList flow.EventList) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&eventList); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeEventList(eventList *flow.EventList, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(eventList)
}
