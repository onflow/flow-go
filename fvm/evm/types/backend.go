package types

import (
	"github.com/onflow/flow-go/fvm/environment"
)

// Backend provides a subset of the FVM environment functionality
// Any error returned by a Backend is expected to be a `FatalError` or
// a `BackendError`.
type Backend interface {
	environment.ValueStore
	environment.Meter
	environment.EventEmitter
	environment.BlockInfo
	environment.RandomGenerator
	environment.ContractFunctionInvoker
	environment.UUIDGenerator
}
