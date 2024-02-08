package types

import (
	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

// Backend provides a subset of the FVM environment functionality
// any error returned by a Backend is expected to be a fatal error or
// a Backend error
type Backend interface {
	// ledger interactions
	GetValue(owner []byte, key []byte) ([]byte, error)
	SetValue(owner, key, value []byte) error
	ValueExists(owner []byte, key []byte) (bool, error)
	AllocateStorageIndex(owner []byte) (atree.StorageIndex, error)
	// metering requirements
	MeterComputation(common.ComputationKind, uint) error
	ComputationUsed() (uint64, error)
	ComputationAvailable(common.ComputationKind, uint) bool
	// event emission
	EmitEvent(event cadence.Event) error
	// block information
	GetCurrentBlockHeight() (uint64, error)
	// read random
	ReadRandom([]byte) error
	// invoke function
	// TODO make this not be dependent on environment
	// and rename it to invoke only at the EVM
	Invoke(
		spec environment.ContractFunctionSpec,
		arguments []cadence.Value,
	) (
		cadence.Value,
		error,
	)
}

// Environment is a subset of functionalities of that the FVM environment
// provides,
type Environment interface {
	environment.ValueStore
	environment.Meter
	environment.EventEmitter
	environment.BlockInfo
	environment.RandomGenerator
	environment.ContractFunctionInvoker
}

// WrappedEnvironment wraps an FVM environment
// when an error is handles the error
type WrappedEnvironment struct {
	env Environment
}

// NewWrappedWrappedEnvironment constructs a new wrapped environment
// this method is used to wrap FVM environment
func NewWrappedWrappedEnvironment(env Environment) Backend {
	return &WrappedEnvironment{env}
}

var _ Backend = &WrappedEnvironment{}

func (we *WrappedEnvironment) GetValue(owner, key []byte) ([]byte, error) {
	val, err := we.env.GetValue(owner, key)
	if err != nil {
		return nil, NewBackendError(err)
	}
	return val, nil
}

func (we *WrappedEnvironment) SetValue(owner, key, value []byte) error {
	err := we.env.SetValue(owner, key, value)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (we *WrappedEnvironment) ValueExists(owner, key []byte) (bool, error) {
	b, err := we.env.ValueExists(owner, key)
	if err != nil {
		return false, NewBackendError(err)
	}
	return b, nil
}

func (we *WrappedEnvironment) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	index, err := we.env.AllocateStorageIndex(owner)
	if err != nil {
		return atree.StorageIndex{}, NewBackendError(err)
	}
	return index, nil
}

func (we *WrappedEnvironment) MeterComputation(kind common.ComputationKind, intensity uint) error {
	err := we.env.MeterComputation(kind, intensity)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (we *WrappedEnvironment) ComputationUsed() (uint64, error) {
	val, err := we.env.ComputationUsed()
	if err != nil {
		return 0, NewBackendError(err)
	}
	return val, nil
}

func (we *WrappedEnvironment) ComputationIntensities() meter.MeteredComputationIntensities {
	return we.env.ComputationIntensities()
}

func (we *WrappedEnvironment) ComputationAvailable(kind common.ComputationKind, intensity uint) bool {
	return we.env.ComputationAvailable(kind, intensity)
}

func (we *WrappedEnvironment) MeterMemory(usage common.MemoryUsage) error {
	err := we.env.MeterMemory(usage)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (we *WrappedEnvironment) MemoryUsed() (uint64, error) {
	val, err := we.env.MemoryUsed()
	if err != nil {
		return 0, NewBackendError(err)
	}
	return val, nil
}

func (we *WrappedEnvironment) MeterEmittedEvent(byteSize uint64) error {
	err := we.env.MeterEmittedEvent(byteSize)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (we *WrappedEnvironment) TotalEmittedEventBytes() uint64 {
	return we.env.TotalEmittedEventBytes()
}

func (we *WrappedEnvironment) InteractionUsed() (uint64, error) {
	val, err := we.env.InteractionUsed()
	if err != nil {
		return 0, NewBackendError(err)
	}
	return val, nil
}

func (we *WrappedEnvironment) EmitEvent(event cadence.Event) error {
	err := we.env.EmitEvent(event)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (we *WrappedEnvironment) Events() flow.EventsList {
	return we.env.Events()

}

func (we *WrappedEnvironment) ServiceEvents() flow.EventsList {
	return we.env.ServiceEvents()
}

func (we *WrappedEnvironment) ConvertedServiceEvents() flow.ServiceEventList {
	return we.env.ConvertedServiceEvents()
}

func (we *WrappedEnvironment) Reset() {
	we.env.Reset()
}

func (we *WrappedEnvironment) GetCurrentBlockHeight() (uint64, error) {
	val, err := we.env.GetCurrentBlockHeight()
	if err != nil {
		return 0, NewBackendError(err)
	}
	return val, nil
}

func (we *WrappedEnvironment) GetBlockAtHeight(height uint64) (
	runtime.Block,
	bool,
	error,
) {
	val, found, err := we.env.GetBlockAtHeight(height)
	if err != nil {
		return runtime.Block{}, false, NewBackendError(err)
	}
	return val, found, nil
}

func (we *WrappedEnvironment) ReadRandom(buffer []byte) error {
	err := we.env.ReadRandom(buffer)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (we *WrappedEnvironment) Invoke(
	spec environment.ContractFunctionSpec,
	arguments []cadence.Value,
) (
	cadence.Value,
	error,
) {
	val, err := we.env.Invoke(spec, arguments)
	return val, handleEnvironmentError(err)
}

func handleEnvironmentError(err error) error {
	if err == nil {
		return nil
	}

	// if is fvm fatal error
	if errors.IsFailure(err) {
		return NewFatalError(err)
	}

	return NewBackendError(err)
}
