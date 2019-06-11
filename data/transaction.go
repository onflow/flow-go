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

// Hash computes the hash over the necessary Transaction data.
func (tx Transaction) Hash() Hash {
	bytes := EncodeAsBytes(
		tx.ToAddress.Bytes(),
		tx.TxData,
		tx.Nonce,
		tx.PayerSignature,
	)
	return NewHash(bytes)
}