package meter

type MeteringHandler interface {
	CheckMemoryLimit(used uint64) error
	CheckComputationLimit(used uint64) error
	CheckInteractionLimit(used uint64) error

	TotalComputationLimit() uint
	TotalMemoryLimit() uint
	TotalInteractionLimit() uint

	SetTotalMemoryLimit(limit uint64)

	EnforcingComputationLimits() bool
	EnforcingMemoryLimits() bool
	EnforcingInteractionLimits() bool
	RestoreLimitEnforcements()
	DisableAllLimitEnforcements()
	SetLimitEnforcements(computationLimits, memoryLimits, interactionLimits bool)
}
