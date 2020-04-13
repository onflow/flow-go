package wrapper

import (
	"github.com/dapperlabs/flow/protobuf/go/flow/execution"
)

// ExecutionAPIClient allows for generation of a mock (via mockery) for the real ExecutionAPIClient imported as a dependency
// from the flow repo
type ExecutionAPIClient interface {
	execution.ExecutionAPIClient
}
