package util

import (
	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"
)

// NopMeter is a meter that does nothing. It can be used in migrations.
type NopMeter struct{}

func (n NopMeter) ComputationAvailable(_ common.ComputationUsage) bool {
	return false
}

func (n NopMeter) MeterComputation(_ common.ComputationUsage) error {
	return nil
}

func (n NopMeter) ComputationUsed() (uint64, error) {
	return 0, nil
}

func (n NopMeter) ComputationIntensities() meter.MeteredComputationIntensities {
	return meter.MeteredComputationIntensities{}
}

func (n NopMeter) MeterMemory(_ common.MemoryUsage) error {
	return nil
}

func (n NopMeter) MemoryUsed() (uint64, error) {
	return 0, nil
}

func (n NopMeter) MeterEmittedEvent(_ uint64) error {
	return nil
}

func (n NopMeter) TotalEmittedEventBytes() uint64 {
	return 0
}

func (n NopMeter) InteractionUsed() (uint64, error) {
	return 0, nil
}

var _ environment.Meter = NopMeter{}
