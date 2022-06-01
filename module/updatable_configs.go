package module

type RequiredApprovalsForSealConstructionInstanceGetter interface {
	GetValue() uint
}

type RequiredApprovalsForSealConstructionInstanceSetter interface {
	RequiredApprovalsForSealConstructionInstanceGetter
	// SetValue takes a new value and returns the old value if the new value is valid.
	// otherwise returns an error
	SetValue(newVal uint) (uint, error)
}
