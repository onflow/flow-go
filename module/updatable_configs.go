package module

type RequiredApprovalsForSealConstructionInstanceGetter interface {
	GetValue() uint
	ChunkAlphaConst() uint
	RequireApprovalsForSealVerificationConst() uint
}

type RequiredApprovalsForSealConstructionInstanceSetter interface {
	RequiredApprovalsForSealConstructionInstanceGetter
	// SetValue takes a new value and returns the old value if the new value is valid.
	// otherwise returns an error, and the value is not updated (equivalent to no-op)
	SetValue(newVal uint) (uint, error)
}
