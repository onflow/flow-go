package basic

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	interfaceMeter "github.com/onflow/flow-go/fvm/meter"
)

var _ interfaceMeter.Meter = &Meter{}

// Meter collects memory and computation usage and enforces limits
// for any each memory/computation usage call it sums intensity to the total
// memory/computation usage metrics and returns error if limits are not met.
type Meter struct {
	computationUsed  uint
	computationLimit uint
	memoryUsed       uint
	memoryLimit      uint

	computationIntensities interfaceMeter.MeteredComputationIntensities
	memoryIntensities      interfaceMeter.MeteredMemoryIntensities
}

// NewMeter constructs a new Meter
func NewMeter(computationLimit, memoryLimit uint) *Meter {
	return &Meter{
		computationLimit:       computationLimit,
		memoryLimit:            memoryLimit,
		computationIntensities: make(interfaceMeter.MeteredComputationIntensities),
		memoryIntensities:      make(interfaceMeter.MeteredMemoryIntensities),
	}
}

// NewChild construct a new Meter instance with the same limits as parent
func (m *Meter) NewChild() interfaceMeter.Meter {
	return &Meter{
		computationLimit:       m.computationLimit,
		memoryLimit:            m.memoryLimit,
		computationIntensities: make(interfaceMeter.MeteredComputationIntensities),
		memoryIntensities:      make(interfaceMeter.MeteredMemoryIntensities),
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

// MeterComputation captures computation usage and returns an error if it goes beyond the limit
func (m *Meter) MeterComputation(kind common.ComputationKind, intensity uint) error {
	m.computationIntensities[kind] += intensity
	var meterable bool
	switch kind {
	case common.ComputationKindStatement,
		common.ComputationKindLoop,
		common.ComputationKindFunctionInvocation:
		meterable = true
	}
	if meterable {
		m.computationUsed += intensity
		if m.computationUsed > m.computationLimit {
			return errors.NewComputationLimitExceededError(uint64(m.computationLimit))
		}
	}
	return nil
}

// ComputationIntensities returns all the measured computational intensities
func (m *Meter) ComputationIntensities() interfaceMeter.MeteredComputationIntensities {
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

// MeterMemory captures memory usage and returns an error if it goes beyond the limit
func (m *Meter) MeterMemory(kind common.MemoryKind, intensity uint) error {
	m.memoryIntensities[kind] += intensity
	m.memoryUsed += intensity
	if m.memoryUsed > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(uint64(m.memoryLimit))
	}
	return nil
}

// MemoryIntensities returns all the measured memory intensities
func (m *Meter) MemoryIntensities() interfaceMeter.MeteredMemoryIntensities {
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
