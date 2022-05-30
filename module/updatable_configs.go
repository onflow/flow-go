package module

type RequiredApprovalsForSealConstructionInstanceGetter interface {
	GetValue() uint
}

type RequiredApprovalsForSealConstructionInstanceSetter interface {
	RequiredApprovalsForSealConstructionInstanceGetter
	// SetValue takes a new value and returns the old value
	SetValue(newVal uint) uint
}
