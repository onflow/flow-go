package data

// Transaction represents a normal transaction.
type Transaction struct {
	TxHash         Hash
	Status         TxStatus
	ToAddress      Address
	TxData         []byte
	Nonce          uint64
	ComputeUsed    uint64
	PayerSignature []byte
}
