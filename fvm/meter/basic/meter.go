package basic

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	interfaceMeter "github.com/onflow/flow-go/fvm/meter"
)

var _ interfaceMeter.Meter = &Meter{}

// Meter collects memory and computation usage and enforeces limits
// for any each memory/computation usage call it sums intensity to the total
// memory/computation usage metrics and returns error if limits are not met.
type Meter struct {
	computationUsed  uint
	computationLimit uint
	memoryUsed       uint
	memoryLimit      uint

	computationIntensities map[uint]uint
	memoryIntensities      map[uint]uint

	computationWeights map[uint]uint
	memoryWeights      map[uint]uint
}

var (
	// defaultComputationWeights is the default weights for computation intensities
	// these weighs make the computation metering the same as it was before dynamic execution fees
	defaultComputationWeights = map[uint]uint{
		uint(common.ComputationKindStatement):          1,
		uint(common.ComputationKindLoop):               1,
		uint(common.ComputationKindFunctionInvocation): 1,
	}
	defaultMemoryWeights = map[uint]uint{}
)

// NewMeter constructs a new Meter
func NewMeter(computationLimit, memoryLimit uint) *Meter {
	return &Meter{
		computationLimit:       computationLimit,
		memoryLimit:            memoryLimit,
		computationWeights:     defaultComputationWeights,
		memoryWeights:          defaultMemoryWeights,
		computationIntensities: make(map[uint]uint),
		memoryIntensities:      make(map[uint]uint),
	}
}

// NewChild construct a new Meter instance with the same limits as parent
func (m *Meter) NewChild() interfaceMeter.Meter {
	return &Meter{
		computationLimit:       m.computationLimit,
		memoryLimit:            m.memoryLimit,
		computationWeights:     m.computationWeights,
		memoryWeights:          m.memoryWeights,
		computationIntensities: make(map[uint]uint),
		memoryIntensities:      make(map[uint]uint),
	}
}

// MergeMeter merges the input meter into the current meter and checks for the limits
func (m *Meter) MergeMeter(child interfaceMeter.Meter) error {
	m.computationUsed = m.computationUsed + child.TotalComputationUsed()
	if m.computationUsed > m.computationLimit {
		return errors.NewComputationLimitExceededError(uint64(m.computationLimit))
	}

	for key, intensity := range child.ComputationIntensities() {
		m.computationIntensities[key] += intensity
	}

	m.memoryUsed = m.memoryUsed + child.TotalMemoryUsed()
	if m.memoryUsed > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(uint64(m.memoryLimit))
	}

	for key, intensity := range child.MemoryIntensities() {
		m.memoryIntensities[key] += intensity
	}
	return nil
}

// SetComputationWeights sets the weights of the different kinds of computation usage
func (m *Meter) SetComputationWeights(w map[uint]uint) {
	m.computationWeights = w
}

// MeterComputation captures computation usage and returns an error if it goes beyond the limit
func (m *Meter) MeterComputation(kind uint, intensity uint) error {
	m.computationIntensities[kind] += intensity
	w, ok := m.computationWeights[kind]
	if !ok {
		return nil
	}
	m.computationUsed += w * intensity
	if m.computationUsed > m.computationLimit {
		return errors.NewComputationLimitExceededError(uint64(m.computationLimit))
	}
	return nil
}

// ComputationIntensities returns all the measured computational intensities
func (m *Meter) ComputationIntensities() map[uint]uint {
	return m.computationIntensities
}

// TotalComputationUsed returns the total computation used
func (m *Meter) TotalComputationUsed() uint {
	return m.computationUsed
}

// TotalComputationLimit returns the total computation limit
func (m *Meter) TotalComputationLimit() uint {
	return m.computationLimit
}

// SetMemoryWeights sets the weights of the different kinds of memory usage
func (m *Meter) SetMemoryWeights(w map[uint]uint) {
	m.memoryWeights = w
}

// MeterMemory captures memory usage and returns an error if it goes beyond the limit
func (m *Meter) MeterMemory(kind uint, intensity uint) error {
	m.memoryIntensities[kind] += intensity
	w, ok := m.memoryWeights[kind]
	if !ok {
		return nil
	}
	m.memoryUsed += w * intensity
	if m.memoryUsed > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(uint64(m.memoryLimit))
	}
	return nil
}

// MemoryIntensities returns all the measured memory intensities
func (m *Meter) MemoryIntensities() map[uint]uint {
	return m.memoryIntensities
}

// TotalMemoryUsed returns the total memory used
func (m *Meter) TotalMemoryUsed() uint {
	return m.memoryUsed
}

// TotalMemoryLimit returns the total memory limit
func (m *Meter) TotalMemoryLimit() uint {
	return m.memoryLimit
}
