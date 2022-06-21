package noop

import (
	"github.com/onflow/cadence/runtime/common"

	interfaceMeter "github.com/onflow/flow-go/fvm/meter"
)

var _ interfaceMeter.Meter = &Meter{}

// Meter is a noop meter
type Meter struct{}

func (m *Meter) MeterRead(size uint64) error {
	return nil
}

func (m *Meter) MeterWrite(previousSize uint64, newSize uint64) error {
	return nil
}

func (m *Meter) MeterNewWrite(size uint64) error {
	return nil
}

func (m *Meter) ReadCounter() uint64 {
	return 0
}

func (m *Meter) WriteCounter() uint64 {
	return 0
}

func (m *Meter) TotalBytesRead() uint64 {
	return 0
}

func (m *Meter) TotalBytesWritten() uint64 {
	return 0
}

func (m *Meter) TotalInteractionUsed() uint64 {
	return 0
}

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
