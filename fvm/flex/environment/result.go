package env

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Result holds the artifacts generated of execution
// TODO we might not need this and reuse the ExecutionResult provided by VM
// extra methods like convert can be provided by the interface layer
type Result struct {
	Failed                   bool // this tracks user level failures, other errors indicates fatal issues
	Error                    error
	RootHash                 common.Hash
	DeployedContractAddress  common.Address
	RetValue                 []byte
	GasConsumed              uint64
	Logs                     []*types.Log
	UUIDIndex                uint64
	TotalSupplyOfNativeToken uint64
}

func (r *Result) Events() {
	// TODO convert EVM logs into FVM events
	// for _, log := range r.Logs {
	//   log.EncodeRLP()
	// }
}

func (r *Result) WrappedError() error {
	// TODO
	return nil
}
