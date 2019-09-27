package types

import (
	"bytes"
	"encoding/gob"

	"github.com/dapperlabs/flow-go/pkg/crypto"
)

type ExecutionReceipt struct {
	PreviousReceiptHash       crypto.Hash
	BlockHash                 crypto.Hash
	Signatures                []crypto.Signature
	InitialRegisters          Registers
	IntermediateRegistersList []IntermediateRegisters
}

// Encode implements the crypto.Encoder interface.
func (m *ExecutionReceipt) Encode() []byte {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(m); err != nil {
		panic(err)
	}
	return b.Bytes()
}
