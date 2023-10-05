package models

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Result struct {
	StateRootHash           common.Hash
	LogsRootHash            common.Hash
	DeployedContractAddress FlexAddress
	ReturnedValue           []byte
	GasConsumed             uint64
	Logs                    []*types.Log
}
