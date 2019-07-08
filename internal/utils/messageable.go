package utils

import (
	"reflect"

	"github.com/golang/protobuf/ptypes"

	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/internal/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func FromMessage(m *interface{}) *interface{} {
	switch messageType := reflect.TypeOf(m); {
	case messageType == *bambooProto.Register:
		return nil
	case messageType == *bambooProto.IntermediateRegisters:
		return nil
	case messageType == *bambooProto.TransactionRegister:
		return nil
	case messageType == *bambooProto.Collection:
		return nil
	case messageType == *bambooProto.SignedCollectionHash:
		return nil
	case messageType == *bambooProto.Block:
		return nil
	case messageType == *bambooProto.BlockSeal:
		return nil
	case messageType == *bambooProto.Transaction:
		return nil
	case messageType == *bambooProto.SignedTransaction:
		return nil
	case messageType == *bambooProto.ExecutionReceipt:
		return nil
	case messageType == *bambooProto.InvalidExecutionReceiptChallenge:
		return nil
	case messageType == *bambooProto.ResultApproval:
		return nil
	default:
		return nil
	}
}

func ToMessage(t *interface{}) *interface{} {
	switch internalType := reflect.TypeOf(t); {
	case internalType == *types.Register:
		return nil
	case internalType == *types.IntermediateRegisters:
		return nil
	case internalType == *types.TransactionRegister:
		return nil
	case internalType == *types.Collection:
		return nil
	case internalType == *types.SignedCollectionHash:
		return nil
	case internalType == *types.Block:
		return nil
	case internalType == *types.BlockSeal:
		return nil
	case internalType == *types.Transaction:
		return nil
	case internalType == *types.SignedTransaction:
		return nil
	case internalType == *types.ExecutionReceipt:
		return nil
	case internalType == *types.InvalidExecutionReceiptChallenge:
		return nil
	case internalType == *types.ResultApproval:
		return nil
	default:
		return nil
	}
}
