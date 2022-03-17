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
}

// NewMeter constructs a new Meter
func NewMeter(computationLimit, memoryLimit uint) *Meter {
	return &Meter{
		computationLimit: computationLimit,
		memoryLimit:      memoryLimit,
	}
}

// NewChild construct a new Meter instance with the same limits as parent
func (m *Meter) NewChild() interfaceMeter.Meter {
	return &Meter{
		computationLimit: m.computationLimit,
		memoryLimit:      m.memoryLimit,
	}
}

// MergeMeter merges the input meter into the current meter and checks for the limits
func (m *Meter) MergeMeter(child interfaceMeter.Meter) error {
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
	var meterable bool
	switch common.ComputationKind(kind) {
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

// TotalComputationUsed returns the total computation used
func (m *Meter) TotalComputationUsed() uint {
	return m.computationUsed
}

// TotalComputationLimit returns the total computation limit
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

// TotalMemoryUsed returns the total memory used
func (m *Meter) TotalMemoryUsed() uint {
	return m.memoryUsed
}

// TotalMemoryLimit returns the total memory limit
func (m *Meter) TotalMemoryLimit() uint {
	return m.memoryLimit
}
