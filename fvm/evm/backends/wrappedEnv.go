package backends

import (
	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

// WrappedEnvironment wraps an FVM environment
type WrappedEnvironment struct {
	env environment.Environment
}

// NewWrappedEnvironment constructs a new wrapped environment
func NewWrappedEnvironment(env environment.Environment) types.Backend {
	return &WrappedEnvironment{env}
}

var _ types.Backend = &WrappedEnvironment{}

func (we *WrappedEnvironment) GetValue(owner, key []byte) ([]byte, error) {
	val, err := we.env.GetValue(owner, key)
	return val, handleEnvironmentError(err)
}

func (we *WrappedEnvironment) SetValue(owner, key, value []byte) error {
	err := we.env.SetValue(owner, key, value)
	return handleEnvironmentError(err)
}

func (we *WrappedEnvironment) ValueExists(owner, key []byte) (bool, error) {
	b, err := we.env.ValueExists(owner, key)
	return b, handleEnvironmentError(err)
}

func (we *WrappedEnvironment) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	index, err := we.env.AllocateStorageIndex(owner)
	return index, handleEnvironmentError(err)
}

func (we *WrappedEnvironment) MeterComputation(kind common.ComputationKind, intensity uint) error {
	err := we.env.MeterComputation(kind, intensity)
	return handleEnvironmentError(err)
}

func (we *WrappedEnvironment) ComputationUsed() (uint64, error) {
	val, err := we.env.ComputationUsed()
	return val, handleEnvironmentError(err)
}

func (we *WrappedEnvironment) ComputationIntensities() meter.MeteredComputationIntensities {
	return we.env.ComputationIntensities()
}

func (we *WrappedEnvironment) ComputationAvailable(kind common.ComputationKind, intensity uint) bool {
	return we.env.ComputationAvailable(kind, intensity)
}

func (we *WrappedEnvironment) MeterMemory(usage common.MemoryUsage) error {
	err := we.env.MeterMemory(usage)
	return handleEnvironmentError(err)
}

func (we *WrappedEnvironment) MemoryUsed() (uint64, error) {
	val, err := we.env.MemoryUsed()
	return val, handleEnvironmentError(err)
}

func (we *WrappedEnvironment) MeterEmittedEvent(byteSize uint64) error {
	err := we.env.MeterEmittedEvent(byteSize)
	return handleEnvironmentError(err)
}

func (we *WrappedEnvironment) TotalEmittedEventBytes() uint64 {
	return we.env.TotalEmittedEventBytes()
}

func (we *WrappedEnvironment) InteractionUsed() (uint64, error) {
	val, err := we.env.InteractionUsed()
	return val, handleEnvironmentError(err)
}

func (we *WrappedEnvironment) EmitEvent(event cadence.Event) error {
	err := we.env.EmitEvent(event)
	return handleEnvironmentError(err)
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
	return val, handleEnvironmentError(err)
}

func (we *WrappedEnvironment) GetBlockAtHeight(height uint64) (
	runtime.Block,
	bool,
	error,
) {
	val, found, err := we.env.GetBlockAtHeight(height)
	return val, found, handleEnvironmentError(err)
}

func (we *WrappedEnvironment) ReadRandom(buffer []byte) error {
	err := we.env.ReadRandom(buffer)
	return handleEnvironmentError(err)
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

func (we *WrappedEnvironment) GenerateUUID() (uint64, error) {
	uuid, err := we.env.GenerateUUID()
	return uuid, handleEnvironmentError(err)
}

func handleEnvironmentError(err error) error {
	if err == nil {
		return nil
	}

	// fvm fatal errors
	if errors.IsFailure(err) {
		return types.NewFatalError(err)
	}

	return types.NewBackendError(err)
}
