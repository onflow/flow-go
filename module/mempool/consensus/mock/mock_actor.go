package mock

import "github.com/onflow/flow-go/model/flow"

// ExecForkActor allows to create a mock for the ExecForkActor callback
type ExecForkActor interface {
	OnExecFork([]*flow.IncorporatedResultSeal)
}
