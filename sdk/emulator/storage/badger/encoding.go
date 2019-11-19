package badger

import (
	"bytes"
	"encoding/gob"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
)

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

func encodeUint64(v uint64) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeUint64(v *uint64, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(v)
}

func encodeLedger(ledger flow.Ledger) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&ledger); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeLedger(ledger *flow.Ledger, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(ledger)
}

func encodeEvents(events []flow.Event) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&events); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeEvents(events *[]flow.Event, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(events)
}

func encodeChangelist(clist changelist) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&clist.blocks); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeChangelist(clist *changelist, from []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(from)).Decode(&clist.blocks)
}
