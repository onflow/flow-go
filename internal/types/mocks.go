package types

import (
	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func MockHash() crypto.Hash {
	return crypto.Hash{}
}

func MockSignature() crypto.Signature {
	return crypto.Signature{}
}

func MockKeyWeight() KeyWeight {
	return KeyWeight{
		Key:    []byte("MOCK"),
		Weight: 0,
	}
}

func MockKeyWeightMessage() *bambooProto.TransactionRegister_KeyWeight {
	return &bambooProto.TransactionRegister_KeyWeight{
		Key:    []byte("MOCK"),
		Weight: 0,
	}
}

func MockRegister() *Register {
	return &Register{
		ID:    []byte("TEST"),
		Value: []byte("MOCK"),
	}
}

func MockRegisterMessage() *bambooProto.Register {
	return &bambooProto.Register{
		Id:    []byte("TEST"),
		Value: []byte("MOCK"),
	}
}

func MockIntermediateRegisters() *IntermediateRegisters {
	registers := make([]Register, 0)
	for i := 1; i < 5; i++ {
		registers = append(registers, *MockRegister())
	}

	return &IntermediateRegisters{
		TransactionHash: MockHash(),
		Registers:       registers,
		ComputeUsed:     0,
	}
}

func MockIntermediateRegistersMessage() *bambooProto.IntermediateRegisters {
	registers := make([]*bambooProto.Register, 0)
	for i := 1; i < 5; i++ {
		registers = append(registers, MockRegisterMessage())
	}

	return &bambooProto.IntermediateRegisters{
		TransactionHash: MockHash().Bytes(),
		Registers:       registers,
		ComputeUsed:     0,
	}
}

func MockTransactionRegister() *TransactionRegister {
	keys := make([]KeyWeight, 0)
	for i := 1; i < 5; i++ {
		keys = append(keys, MockKeyWeight())
	}

	return &TransactionRegister{
		Type:       SIMPLE,
		AccessMode: CREATE,
		ID:         []byte("TEST"),
		Keys:       keys,
	}
}

func MockTransactionRegisterMessage() *bambooProto.TransactionRegister {
	keys := make([]*bambooProto.TransactionRegister_KeyWeight, 0)
	for i := 1; i < 5; i++ {
		keys = append(keys, MockKeyWeightMessage())
	}

	return &bambooProto.TransactionRegister{
		Type:       bambooProto.TransactionRegister_SIMPLE,
		AccessMode: bambooProto.TransactionRegister_CREATE,
		Id:         []byte("TEST"),
		Keys:       keys,
	}
}
