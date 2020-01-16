package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// Ledger is a map of register values.
type Ledger map[string][]byte

type IntermediateRegisters struct {
	TransactionHash crypto.Hash
	Registers       Ledger
	ComputeUsed     uint64
}
