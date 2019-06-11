package data

// Transaction represents a normal transaction.
type Transaction struct {
	TxHash         Hash
	ToAddress      Address
	Script         []byte
	Nonce          uint64
	ComputeLimit   uint64
	ComputeUsed    uint64
	PayerSignature []byte
	Status         TxStatus
}
