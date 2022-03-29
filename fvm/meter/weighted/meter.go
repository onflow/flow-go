package weighted

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	interfaceMeter "github.com/onflow/flow-go/fvm/meter"
)

// MeterInternalPrecisionBytes are the amount of bytes that are used internally by the Meter
// to allow for metering computation smaller than one unit of computation. This allows for more fine weights.
// A weight of 1 unit of computation is equal to 1<<16. The minimum possible weight is 1/65536.
const MeterInternalPrecisionBytes = 16

type ExecutionWeights map[common.ComputationKind]uint64

var (
	// DefaultComputationWeights is the default weights for computation intensities
	// these weighs make the computation metering the same as it was before dynamic execution fees
	DefaultComputationWeights = ExecutionWeights{
		common.ComputationKindStatement:          1 << MeterInternalPrecisionBytes,
		common.ComputationKindLoop:               1 << MeterInternalPrecisionBytes,
		common.ComputationKindFunctionInvocation: 1 << MeterInternalPrecisionBytes,
	}
	// DefaultMemoryWeights are empty as memory metering is not fully defined yet
	DefaultMemoryWeights = ExecutionWeights{}
)

var _ interfaceMeter.Meter = &Meter{}

// Meter collects memory and computation usage and enforces limits
// for any each memory/computation usage call it sums intensity multiplied by the weight of the intensity to the total
// memory/computation usage metrics and returns error if limits are not met.
type Meter struct {
	computationUsed  uint64
	computationLimit uint64
	memoryUsed       uint64
	memoryLimit      uint64

	computationIntensities interfaceMeter.MeteredIntensities
	memoryIntensities      interfaceMeter.MeteredIntensities

	computationWeights ExecutionWeights
	memoryWeights      ExecutionWeights
}

type WeightedMeterOptions func(*Meter)

// NewMeter constructs a new Meter
func NewMeter(computationLimit, memoryLimit uint, options ...WeightedMeterOptions) *Meter {

	m := &Meter{
		computationLimit:       uint64(computationLimit) << MeterInternalPrecisionBytes,
		memoryLimit:            uint64(memoryLimit) << MeterInternalPrecisionBytes,
		computationWeights:     DefaultComputationWeights,
		memoryWeights:          DefaultMemoryWeights,
		computationIntensities: make(interfaceMeter.MeteredIntensities),
		memoryIntensities:      make(interfaceMeter.MeteredIntensities),
	}

	for _, option := range options {
		option(m)
	}

	return m
}

// WithComputationWeights sets the weights for computation intensities
func WithComputationWeights(weights ExecutionWeights) WeightedMeterOptions {
	return func(m *Meter) {
		m.computationWeights = weights
	}
}

// WithMemoryWeights sets the weights for the memory intensities
func WithMemoryWeights(weights ExecutionWeights) WeightedMeterOptions {
	return func(m *Meter) {
		m.memoryWeights = weights
	}
}

// NewChild construct a new Meter instance with the same limits as parent
func (m *Meter) NewChild() interfaceMeter.Meter {
	return &Meter{
		computationLimit:       m.computationLimit,
		memoryLimit:            m.memoryLimit,
		computationWeights:     m.computationWeights,
		memoryWeights:          m.memoryWeights,
		computationIntensities: make(interfaceMeter.MeteredIntensities),
		memoryIntensities:      make(interfaceMeter.MeteredIntensities),
	}
}

// MergeMeter merges the input meter into the current meter and checks for the limits
func (m *Meter) MergeMeter(child interfaceMeter.Meter) error {

	var childComputationUsed uint64
	if basic, ok := child.(*Meter); ok {
		childComputationUsed = basic.computationUsed
	} else {
		childComputationUsed = uint64(child.TotalComputationUsed()) << MeterInternalPrecisionBytes
	}
	m.computationUsed = m.computationUsed + childComputationUsed
	if m.computationUsed > m.computationLimit {
		return errors.NewComputationLimitExceededError(m.computationLimit)
	}

	for key, intensity := range child.ComputationIntensities() {
		m.computationIntensities[key] += intensity
	}

	var childMemoryUsed uint64
	if basic, ok := child.(*Meter); ok {
		childMemoryUsed = basic.memoryUsed
	} else {
		childMemoryUsed = uint64(child.TotalMemoryUsed()) << MeterInternalPrecisionBytes
	}
	m.memoryUsed = m.memoryUsed + childMemoryUsed
	if m.memoryUsed > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(m.memoryLimit)
	}

	for key, intensity := range child.MemoryIntensities() {
		m.memoryIntensities[key] += intensity
	}
	return nil
}

// SetComputationWeights sets the computation weights
func (m *Meter) SetComputationWeights(weights map[common.ComputationKind]uint64) {
	m.computationWeights = weights
}

// MeterComputation captures computation usage and returns an error if it goes beyond the limit
func (m *Meter) MeterComputation(kind common.ComputationKind, intensity uint) error {
	m.computationIntensities[kind] += intensity
	w, ok := m.computationWeights[kind]
	if !ok {
		return nil
	}
	m.computationUsed += w * uint64(intensity)
	if m.computationUsed > m.computationLimit {
		return errors.NewComputationLimitExceededError(m.computationLimit)
	}
	return nil
}

// ComputationIntensities returns all the measured computational intensities
func (m *Meter) ComputationIntensities() interfaceMeter.MeteredIntensities {
	return m.computationIntensities
}

// TotalComputationUsed returns the total computation used
func (m *Meter) TotalComputationUsed() uint {
	return uint(m.computationUsed >> MeterInternalPrecisionBytes)
}

// TotalComputationLimit returns the total computation limit
func (m *Meter) TotalComputationLimit() uint {
	return uint(m.computationLimit >> MeterInternalPrecisionBytes)
}

// SetMemoryWeights sets the memory weights
func (m *Meter) SetMemoryWeights(weights map[common.ComputationKind]uint64) {
	m.memoryWeights = weights
}

// MeterMemory captures memory usage and returns an error if it goes beyond the limit
func (m *Meter) MeterMemory(kind common.ComputationKind, intensity uint) error {
	m.memoryIntensities[kind] += intensity
	w, ok := m.memoryWeights[kind]
	if !ok {
		return nil
	}
	m.memoryUsed += w * uint64(intensity)
	if m.memoryUsed > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(m.memoryLimit)
	}
	return nil
}

// MemoryIntensities returns all the measured memory intensities
func (m *Meter) MemoryIntensities() interfaceMeter.MeteredIntensities {
	return m.memoryIntensities
}

// TotalMemoryUsed returns the total memory used
func (m *Meter) TotalMemoryUsed() uint {
	return uint(m.memoryUsed >> MeterInternalPrecisionBytes)
}

// TotalMemoryLimit returns the total memory limit
func (m *Meter) TotalMemoryLimit() uint {
	return uint(m.memoryLimit >> MeterInternalPrecisionBytes)
}
