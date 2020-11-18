package cmd

import (
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/state/protocol"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

func initForest() (*mtrie.Forest, error) {
	return common.InitForest(flagExecutionStateDir)
}

func initStates() (protocol.State, state.ExecutionState, error) {
	db := common.InitStorage(flagDataDir)
	return common.InitStates(db, flagExecutionStateDir)
}
