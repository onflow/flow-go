package types

import (
	"github.com/onflow/flow-go/fvm/environment"
)

// BackendStorage provides an interface for storage of registers
type BackendStorage interface {
	environment.ValueStore
}

// Backend provides a subset of the FVM environment functionality
// Any error returned by a Backend is expected to be a `FatalError` or
// a `BackendError`.
type Backend interface {
	BackendStorage
	environment.Meter
	environment.EventEmitter
	environment.BlockInfo
	environment.RandomGenerator
	environment.ContractFunctionInvoker
	environment.UUIDGenerator
	environment.Tracer
	environment.EVMMetricsReporter
	environment.LoggerProvider
}
