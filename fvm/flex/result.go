package flex

import (
	"github.com/ethereum/go-ethereum/common"
)

// Result holds the artifacts generated of execution
type Result struct {
	RootHash                common.Hash
	DeployedContractAddress common.Address
	RetValue                []byte
	GasConsumed             uint64
	// Logs                    ???
}
