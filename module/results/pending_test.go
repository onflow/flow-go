package results_test

import (
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/results"
)

// check the implementation
var _ module.PendingResults = (*results.PendingResults)(nil)
