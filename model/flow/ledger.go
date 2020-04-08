package flow

import (
	"github.com/dapperlabs/flow-go/crypto/hash"
)

// Ledger is a map of register values.
type Ledger map[string][]byte

type IntermediateRegisters struct {
	TransactionHash hash.Hash
	Registers       Ledger
	ComputeUsed     uint64
}
