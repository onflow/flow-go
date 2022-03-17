package noop

import (
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
func (m *Meter) MergeMeter(_ interfaceMeter.Meter) error {
	return nil
}

// SetComputationWeights is a noop
func (m *Meter) SetComputationWeights(_ map[uint]uint) {
}

// MeterComputation is a noop
func (m *Meter) MeterComputation(_ uint, _ uint) error {
	return nil
}

// ComputationIntensities returns an empty map
func (m *Meter) ComputationIntensities() map[uint]uint {
	return map[uint]uint{}
}

// TotalComputationUsed always returns zero
func (m *Meter) TotalComputationUsed() uint {
	return 0
}

// TotalComputationLimit always returns zero
func (m *Meter) TotalComputationLimit() uint {
	return 0
}

// SetMemoryWeights is a noop
func (m *Meter) SetMemoryWeights(_ map[uint]uint) {
}

// MeterMemory is a noop
func (m *Meter) MeterMemory(_ uint, _ uint) error {
	return nil
}

// MemoryIntensities returns an empty map
func (m *Meter) MemoryIntensities() map[uint]uint {
	return map[uint]uint{}
}

// TotalMemoryUsed always returns zero
func (m *Meter) TotalMemoryUsed() uint {
	return 0
}

// TotalMemoryLimit always returns zero
func (m *Meter) TotalMemoryLimit() uint {
	return 0
}
