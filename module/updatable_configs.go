package module

type SealingConfigsGetter interface {
	// updatable
	RequireApprovalsForSealConstructionDynamicValue() uint

	// not-updatable
	ChunkAlphaConst() uint
	RequireApprovalsForSealVerificationConst() uint
}

type SealingConfigsSetter interface {
	SealingConfigsGetter
	// SetValue takes a new value and returns the old value if the new value is valid.
	// otherwise returns an error, and the value is not updated (equivalent to no-op)
	SetValue(newVal uint) (uint, error)
}
