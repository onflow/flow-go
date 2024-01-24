package types

// FLOWTokenVault holds a balance of flow token
type FLOWTokenVault struct {
	balance Balance
}

func NewFlowTokenVault(balance Balance) *FLOWTokenVault {
	return &FLOWTokenVault{balance: balance}
}

func (t *FLOWTokenVault) Balance() Balance {
	return t.balance
}

func (t *FLOWTokenVault) Withdraw(b Balance) (*FLOWTokenVault, error) {
	var err error
	t.balance, err = SubBalance(t.balance, b)
	return NewFlowTokenVault(b), err
}

func (t *FLOWTokenVault) Deposit(inp *FLOWTokenVault) error {
	var err error
	t.balance, err = AddBalance(t.balance, inp.balance)
	return err
}
