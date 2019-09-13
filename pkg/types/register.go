package types

import "github.com/dapperlabs/flow-go/pkg/crypto"

type Registers map[crypto.Hash][]byte

type IntermediateRegisters struct {
	TransactionHash crypto.Hash
	Registers       Registers
	ComputeUsed     uint64
}
