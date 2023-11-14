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

func (t *FLOWTokenVault) Withdraw(b Balance) *FLOWTokenVault {
	t.balance = t.balance.Sub(b)
	return NewFlowTokenVault(b)
}

func (t *FLOWTokenVault) Deposit(inp *FLOWTokenVault) {
	t.balance = t.balance.Add(inp.Balance())
}
