package evm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Result holds the artifacts generated of execution
// TODO we might not need this and reuse the ExecutionResult provided by VM
// extra methods like convert can be provided by the interface layer
type Result struct {
	RootHash                 common.Hash
	DeployedContractAddress  common.Address
	RetValue                 []byte
	GasConsumed              uint64
	Logs                     []*types.Log
	UUIDIndex                uint64
	TotalSupplyOfNativeToken uint64
}
