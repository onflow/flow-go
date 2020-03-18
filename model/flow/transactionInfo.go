package flow

type TransactionInfo struct {
	TransactionID Identifier
	// CollectionID of collection containing this transaction
	CollectionID Identifier
	Status       TransactionStatus
}

func (tb TransactionInfo) ID() Identifier {
	return MakeID(tb)
}

func (tb TransactionInfo) Checksum() Identifier {
	return MakeID(tb)
}
