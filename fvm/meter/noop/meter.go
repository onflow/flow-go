package noop

import (
	"github.com/onflow/cadence/runtime/common"

	interfaceMeter "github.com/onflow/flow-go/fvm/meter"
)

var _ interfaceMeter.Meter = &Meter{}

// Meter is a noop meter
type Meter struct{}

// NewMeter construct a new noop meter
func NewMeter() *Meter {
	return &Meter{}
}

// NewChild returns a new noop meter
func (m *Meter) NewChild() interfaceMeter.Meter {
	return &Meter{}
}

// MergeMeter merges two noop meters
func (m *Meter) MergeMeter(child interfaceMeter.Meter, enforceLimits bool) error {
	return nil
}

// MeterComputation is a noop
func (m *Meter) MeterComputation(_ common.ComputationKind, _ uint) error {
	return nil
}

// ComputationIntensities returns an empty map
func (m *Meter) ComputationIntensities() interfaceMeter.MeteredComputationIntensities {
	return map[common.ComputationKind]uint{}
}

// TotalComputationUsed always returns zero
func (m *Meter) TotalComputationUsed() uint {
	return 0
}

// TotalComputationLimit always returns zero
func (m *Meter) TotalComputationLimit() uint {
	return 0
}

// MeterMemory is a noop
func (m *Meter) MeterMemory(_ common.MemoryKind, _ uint) error {
	return nil
}

// MemoryIntensities returns an empty map
func (m *Meter) MemoryIntensities() interfaceMeter.MeteredMemoryIntensities {
	return map[common.MemoryKind]uint{}
}

// TotalMemoryUsed always returns zero
func (m *Meter) TotalMemoryUsed() uint {
	return 0
}

// TotalMemoryLimit always returns zero
func (m *Meter) TotalMemoryLimit() uint {
	return 0
}
