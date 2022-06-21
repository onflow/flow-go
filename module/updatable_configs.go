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
	// SetRequiredApprovalsForSealingConstruction takes a new value and returns the old value
	// if the new value is valid.  otherwise returns an error,
	// and the value is not updated (equivalent to no-op)
	SetRequiredApprovalsForSealingConstruction(newVal uint) (uint, error)
}
