package backends

import (
	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// WrappedEnvironment wraps a FVM environment,
// handles external errors and provides backend to the EVM.
type WrappedEnvironment struct {
	env environment.Environment
}

// NewWrappedEnvironment constructs a new wrapped environment
func NewWrappedEnvironment(env environment.Environment) *WrappedEnvironment {
	return &WrappedEnvironment{env}
}

var _ types.Backend = &WrappedEnvironment{}

// GetValue gets a value from the storage for the given owner and key pair,
// if value not found empty slice and no error is returned.
func (we *WrappedEnvironment) GetValue(owner, key []byte) ([]byte, error) {
	val, err := we.env.GetValue(owner, key)
	return val, handleEnvironmentError(err)
}

// SetValue sets a value into the storage for the given owner and key pair.
func (we *WrappedEnvironment) SetValue(owner, key, value []byte) error {
	err := we.env.SetValue(owner, key, value)
	return handleEnvironmentError(err)
}

// ValueExists checks if a value exist for the given owner and key pair.
func (we *WrappedEnvironment) ValueExists(owner, key []byte) (bool, error) {
	b, err := we.env.ValueExists(owner, key)
	return b, handleEnvironmentError(err)
}

// AllocateSlabIndex allocates a slab index under the given account.
func (we *WrappedEnvironment) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	index, err := we.env.AllocateSlabIndex(owner)
	return index, handleEnvironmentError(err)
}

func (we *WrappedEnvironment) RunWithMeteringDisabled(f func()) {
	we.env.RunWithMeteringDisabled(f)
}

// MeterComputation updates the total computation used based on the kind and intensity of the operation.
func (we *WrappedEnvironment) MeterComputation(usage common.ComputationUsage) error {
	err := we.env.MeterComputation(usage)
	return handleEnvironmentError(err)
}

// ComputationUsed returns the computation used so far
func (we *WrappedEnvironment) ComputationUsed() (uint64, error) {
	val, err := we.env.ComputationUsed()
	return val, handleEnvironmentError(err)
}

// ComputationIntensities returns a the list of computation intensities
func (we *WrappedEnvironment) ComputationIntensities() meter.MeteredComputationIntensities {
	return we.env.ComputationIntensities()
}

// ComputationAvailable returns true if there is computation room
// for the given kind and intensity operation.
func (we *WrappedEnvironment) ComputationAvailable(usage common.ComputationUsage) bool {
	return we.env.ComputationAvailable(usage)
}

// ComputationRemaining returns the remaining computation for the given kind.
func (we *WrappedEnvironment) ComputationRemaining(kind common.ComputationKind) uint64 {
	return we.env.ComputationRemaining(kind)
}

func (we *WrappedEnvironment) MeterMemory(usage common.MemoryUsage) error {
	err := we.env.MeterMemory(usage)
	return handleEnvironmentError(err)
}

// MemoryUsed returns the total memory used so far.
func (we *WrappedEnvironment) MemoryUsed() (uint64, error) {
	val, err := we.env.MemoryUsed()
	return val, handleEnvironmentError(err)
}

// MeterEmittedEvent meters a newly emitted event.
func (we *WrappedEnvironment) MeterEmittedEvent(byteSize uint64) error {
	err := we.env.MeterEmittedEvent(byteSize)
	return handleEnvironmentError(err)
}

// TotalEmittedEventBytes returns the total byte size of events emitted so far.
func (we *WrappedEnvironment) TotalEmittedEventBytes() uint64 {
	return we.env.TotalEmittedEventBytes()
}

// EmitEvent emits an event.
func (we *WrappedEnvironment) EmitEvent(event cadence.Event) error {
	err := we.env.EmitEvent(event)
	return handleEnvironmentError(err)
}

// Events returns the list of emitted events.
func (we *WrappedEnvironment) Events() flow.EventsList {
	return we.env.Events()

}

// ServiceEvents returns the list of emitted service events
func (we *WrappedEnvironment) ServiceEvents() flow.EventsList {
	return we.env.ServiceEvents()
}

// ConvertedServiceEvents returns the converted list of emitted service events.
func (we *WrappedEnvironment) ConvertedServiceEvents() flow.ServiceEventList {
	return we.env.ConvertedServiceEvents()
}

// Reset resets and discards all the changes to the
// all stateful environment modules (events, storage, ...)
func (we *WrappedEnvironment) Reset() {
	we.env.Reset()
}

// GetCurrentBlockHeight returns the current Flow block height
func (we *WrappedEnvironment) GetCurrentBlockHeight() (uint64, error) {
	val, err := we.env.GetCurrentBlockHeight()
	return val, handleEnvironmentError(err)
}

// GetBlockAtHeight returns the block at the given height
func (we *WrappedEnvironment) GetBlockAtHeight(height uint64) (
	runtime.Block,
	bool,
	error,
) {
	val, found, err := we.env.GetBlockAtHeight(height)
	return val, found, handleEnvironmentError(err)
}

// ReadRandom sets a random number into the buffer
func (we *WrappedEnvironment) ReadRandom(buffer []byte) error {
	err := we.env.ReadRandom(buffer)
	return handleEnvironmentError(err)
}

// Invoke invokes call inside the fvm env.
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

// GenerateUUID generates a uuid
func (we *WrappedEnvironment) GenerateUUID() (uint64, error) {
	uuid, err := we.env.GenerateUUID()
	return uuid, handleEnvironmentError(err)
}

// StartChildSpan starts a new child open tracing span.
func (we *WrappedEnvironment) StartChildSpan(
	name trace.SpanName,
	options ...otelTrace.SpanStartOption,
) tracing.TracerSpan {
	return we.env.StartChildSpan(name, options...)
}

func (we *WrappedEnvironment) SetNumberOfDeployedCOAs(count uint64) {
	we.env.SetNumberOfDeployedCOAs(count)
}

func (we *WrappedEnvironment) EVMTransactionExecuted(
	gasUsed uint64,
	isDirectCall bool,
	failed bool,
) {
	we.env.EVMTransactionExecuted(gasUsed, isDirectCall, failed)
}

func (we *WrappedEnvironment) EVMBlockExecuted(
	txCount int,
	totalGasUsed uint64,
	totalSupplyInFlow float64,
) {
	we.env.EVMBlockExecuted(txCount, totalGasUsed, totalSupplyInFlow)
}

func (we *WrappedEnvironment) Logger() zerolog.Logger {
	return we.env.Logger()
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
