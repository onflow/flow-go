package noop

import (
	"github.com/onflow/flow-go/fvm/meter"
)

type MeteringHandler struct{}

func (m MeteringHandler) SetLimitEnforcements(computationLimits, memoryLimits, interactionLimits bool) {
}

func (m MeteringHandler) TotalComputationLimit() uint {
	return 0
}

func (m MeteringHandler) TotalMemoryLimit() uint {
	return 0
}

func (m MeteringHandler) TotalInteractionLimit() uint {
	return 0
}

func (m MeteringHandler) CheckMemoryLimit(used uint64) error {
	return nil
}

func (m MeteringHandler) CheckComputationLimit(used uint64) error {
	return nil
}

func (m MeteringHandler) CheckInteractionLimit(used uint64) error {
	return nil
}

func (m MeteringHandler) SetTotalMemoryLimit(limit uint64) {
}

func (m MeteringHandler) EnforcingComputationLimits() bool {
	return false
}

func (m MeteringHandler) EnforcingMemoryLimits() bool {
	return false
}

func (m MeteringHandler) EnforcingInteractionLimits() bool {
	return false
}

func (m MeteringHandler) RestoreLimitEnforcements() {
}

func (m MeteringHandler) DisableAllLimitEnforcements() {
}

var _ meter.MeteringHandler = &MeteringHandler{}
