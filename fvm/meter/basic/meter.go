package basic

import (
	"github.com/onflow/flow-go/fvm/errors"
	interfaceMeter "github.com/onflow/flow-go/fvm/meter"
)

var _ interfaceMeter.Meter = &Meter{}

// Meter is a basic meter, which collects memory and computation usage
// and enforeces limits
// for any kind of memory or computation usage it sums intensity to the total
// memory and computation used respectively.
type Meter struct {
	computationUsed  uint
	computationLimit uint
	memoryUsed       uint
	memoryLimit      uint
}

func NewMeter(computationLimit, memoryLimit uint) *Meter {
	return &Meter{
		computationLimit: uint(computationLimit),
		memoryLimit:      uint(memoryLimit),
	}
}

// NewChild construct a new Meter instance with the same limits as parent meter
func (m *Meter) NewChild() interfaceMeter.Meter {
	return &Meter{
		computationLimit: m.computationLimit,
		memoryLimit:      m.memoryLimit,
	}
}

// MergeMeter merges two meters, and checks for the limits
func (m *Meter) MergeMeter(child interfaceMeter.Meter) error {
	// TODO type check

	m.computationUsed = m.computationUsed + child.TotalComputationUsed()
	if m.computationUsed > m.computationLimit {
		return errors.NewComputationLimitExceededError(uint64(m.computationLimit))
	}

	m.memoryUsed = m.memoryUsed + child.TotalMemoryUsed()
	if m.memoryUsed > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(uint64(m.memoryLimit))
	}
	return nil
}

// MeterComputation captures computation usage and returns an error if it goes beyond the limit
func (m *Meter) MeterComputation(kind uint, intensity uint) error {
	m.computationUsed += intensity
	if m.computationUsed > m.computationLimit {
		return errors.NewComputationLimitExceededError(uint64(m.computationLimit))
	}
	return nil
}

// TotalComputationUsed returns the total computation used
func (m *Meter) TotalComputationUsed() uint {
	return m.computationUsed
}

// TotalComputationUsed returns the limit for the total computation used
func (m *Meter) TotalComputationLimit() uint {
	return m.computationLimit
}

// MeterMemory captures memory usage and returns an error if it goes beyond the limit
func (m *Meter) MeterMemory(kind uint, intensity uint) error {
	m.memoryUsed += intensity
	if m.memoryUsed > m.memoryLimit {
		return errors.NewMemoryLimitExceededError(uint64(m.memoryLimit))
	}
	return nil
}

// TotalMemoryUsed returns the total amount of used memory
func (m *Meter) TotalMemoryUsed() uint {
	return m.memoryUsed
}

// TotalMemoryLimit returns the limit for total memory used
func (m *Meter) TotalMemoryLimit() uint {
	return m.memoryLimit
}
