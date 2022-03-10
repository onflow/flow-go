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
func (m *Meter) MergeMeter(child interfaceMeter.Meter) error {
	return nil
}

// MeterComputation is a noop
func (m *Meter) MeterComputation(kind uint, _ uint) error {
	return nil
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
func (m *Meter) MeterMemory(kind uint, _ uint) error {
	return nil
}

// TotalMemoryUsed always returns zero
func (m *Meter) TotalMemoryUsed() uint {
	return 0
}

// TotalMemoryLimit always returns zero
func (m *Meter) TotalMemoryLimit() uint {
	return 0
}
