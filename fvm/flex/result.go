package flex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Result holds the artifacts generated of execution
// TODO we might not need this and reuse the ExecutionResult provided by VM
// extra methods like convert can be provided by the interface layer
type Result struct {
	Failed                  bool // this tracks user level failures, other errors indicates fatal issues
	RootHash                common.Hash
	DeployedContractAddress common.Address
	RetValue                []byte
	GasConsumed             uint64
	Logs                    []*types.Log
}

func (r *Result) Events() {
	// TODO convert EVM logs into FVM events
}
