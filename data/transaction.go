package data

// Transaction represents a normal transaction.
type Transaction struct {
	Status         TxStatus
	ToAddress      Address
	TxData         []byte
	Nonce          uint64
	ComputeUsed    uint64
	PayerSignature []byte
}
