package weighted

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
)

type WeightedMeteringOptions func(*MeteringHandler)

type enforcedLimits struct {
	memoryLimits      bool
	computationLimits bool
	interactionLimits bool
}

type MeteringHandler struct {
	enforceLimitsStack      []enforcedLimits
	enforceLimitsStackDepth uint64

	computationLimit uint64
	memoryLimit      uint64
	interactionLimit uint64

	computationWeights ExecutionEffortWeights
	memoryWeights      ExecutionMemoryWeights
}

func (mh *MeteringHandler) SetLimitEnforcements(computationLimits, memoryLimits, interactionLimits bool) {
	mh.enforceLimitsStackDepth += 1
	mh.enforceLimitsStack = append(mh.enforceLimitsStack, enforcedLimits{
		memoryLimits:      memoryLimits,
		computationLimits: computationLimits,
		interactionLimits: interactionLimits,
	})
}

func (mh *MeteringHandler) RestoreLimitEnforcements() {
	if mh.enforceLimitsStackDepth == 0 {
		return
	}
	mh.enforceLimitsStack = mh.enforceLimitsStack[:mh.enforceLimitsStackDepth]
	mh.enforceLimitsStackDepth -= 1
}

func (mh *MeteringHandler) DisableAllLimitEnforcements() {
	mh.enforceLimitsStackDepth += 1
	mh.enforceLimitsStack = append(mh.enforceLimitsStack, enforcedLimits{
		memoryLimits:      false,
		computationLimits: false,
		interactionLimits: false,
	})
}

func (mh *MeteringHandler) EnforcingComputationLimits() bool {
	return mh.enforceLimitsStack[mh.enforceLimitsStackDepth].computationLimits
}

func (mh *MeteringHandler) EnforcingMemoryLimits() bool {
	return mh.enforceLimitsStack[mh.enforceLimitsStackDepth].memoryLimits
}

func (mh *MeteringHandler) EnforcingInteractionLimits() bool {
	return mh.enforceLimitsStack[mh.enforceLimitsStackDepth].interactionLimits
}

func (mh *MeteringHandler) CheckInteractionLimit(used uint64) error {
	if mh.EnforcingInteractionLimits() && used > mh.interactionLimit {
		return errors.NewLedgerInteractionLimitExceededError(used, mh.interactionLimit)
	}
	return nil
}

func (mh *MeteringHandler) CheckMemoryLimit(used uint64) error {
	if mh.EnforcingMemoryLimits() && used > mh.memoryLimit {
		return errors.NewMemoryLimitExceededError(uint64(mh.TotalMemoryLimit()))
	}
	return nil
}

func (mh *MeteringHandler) CheckComputationLimit(used uint64) error {
	if mh.EnforcingComputationLimits() && used > mh.computationLimit {
		return errors.NewComputationLimitExceededError(uint64(mh.TotalComputationLimit()))
	}
	return nil
}

func (mh *MeteringHandler) ComputationWeight(kind common.ComputationKind) (uint64, bool) {
	value, ok := mh.computationWeights[kind]
	return value, ok
}

func (mh *MeteringHandler) MemoryWeight(kind common.MemoryKind) (uint64, bool) {
	value, ok := mh.memoryWeights[kind]
	return value, ok
}

var _ meter.MeteringHandler = &MeteringHandler{}

// NewMeteringHandler constructs a new MeteringHandler
func NewMeteringHandler(computationLimit, memoryLimit, interactionLimit uint64, options ...WeightedMeteringOptions) *MeteringHandler {

	defaultLimits := enforcedLimits{
		memoryLimits:      true,
		computationLimits: true,
		interactionLimits: true,
	}

	mh := &MeteringHandler{
		enforceLimitsStack:      []enforcedLimits{defaultLimits},
		enforceLimitsStackDepth: 0,
		computationLimit:        computationLimit << MeterExecutionInternalPrecisionBytes,
		memoryLimit:             memoryLimit,
		interactionLimit:        interactionLimit,
		computationWeights:      DefaultComputationWeights,
		memoryWeights:           DefaultMemoryWeights,
	}

	for _, option := range options {
		option(mh)
	}

	return mh
}

// WithComputationWeights sets the weights for computation intensities
func WithComputationWeights(weights ExecutionEffortWeights) WeightedMeteringOptions {
	return func(mh *MeteringHandler) {
		mh.computationWeights = weights
	}
}

// WithMemoryWeights sets the weights for the memory intensities
func WithMemoryWeights(weights ExecutionMemoryWeights) WeightedMeteringOptions {
	return func(mh *MeteringHandler) {
		mh.memoryWeights = weights
	}
}

// SetTotalMemoryLimit sets the total memory limit
func (mh *MeteringHandler) SetTotalMemoryLimit(limit uint64) {
	mh.memoryLimit = limit
}

// TotalComputationLimit returns the total computation limit
func (mh *MeteringHandler) TotalComputationLimit() uint {
	return uint(mh.computationLimit >> MeterExecutionInternalPrecisionBytes)
}

// TotalMemoryLimit returns the total memory limit
func (mh *MeteringHandler) TotalMemoryLimit() uint {
	return uint(mh.memoryLimit)
}

// TotalInteractionLimit returns the total interaction limit
func (mh *MeteringHandler) TotalInteractionLimit() uint {
	return uint(mh.interactionLimit)
}

// SetComputationWeights sets the computation weights
func (mh *MeteringHandler) SetComputationWeights(weights ExecutionEffortWeights) {
	mh.computationWeights = weights
}

// SetMemoryWeights sets the memory weights
func (mh *MeteringHandler) SetMemoryWeights(weights ExecutionMemoryWeights) {
	mh.memoryWeights = weights
}
