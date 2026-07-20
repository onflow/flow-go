package util

import (
	"math"

	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"
)

// NopMeter is a meter that does nothing. It can be used in migrations.
type NopMeter struct{}

func (n NopMeter) RunWithMeteringDisabled(f func()) {}

func (n NopMeter) MeterComputation(_ common.ComputationUsage) error {
	return nil
}

func (n NopMeter) MeteringResult() (meter.MeteringResult, error) {
	return meter.MeteringResult{}, nil
}

func (n NopMeter) ComputationRemaining(_ common.ComputationKind) uint64 {
	return math.MaxUint64
}

func (n NopMeter) MeterMemory(_ common.MemoryUsage) error {
	return nil
}

func (n NopMeter) MeterEmittedEvent(_ uint64) error {
	return nil
}

func (n NopMeter) InteractionUsed() (uint64, error) {
	return 0, nil
}

var _ environment.Meter = NopMeter{}
