package models

// TokenVault holds a balance of native token
type TokenVault struct {
	balance Balance
}

func NewTokenVault(balance Balance) *TokenVault {
	return &TokenVault{balance: balance}
}

func (t *TokenVault) Balance() Balance {
	return t.balance
}

func (t *TokenVault) Withdraw(b Balance) *TokenVault {
	t.balance = t.balance.Sub(b)
	return NewTokenVault(b)
}

func (t *TokenVault) Deposit(inp TokenVault) {
	t.balance = t.balance.Add(inp.Balance())
}
