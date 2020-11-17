package cmd

import (
	"github.com/onflow/flow-go/cmd/util/cmd/read-execution-state/common"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
)

func loadExecutionState() (*mtrie.Forest, error) {
	return common.LoadExecutionState(flagExecutionStateDir)
}
