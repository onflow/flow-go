package data

// BlockStatus represents the current status of a Block.
type BlockStatus int

const (
	BlockPending BlockStatus = iota
	BlockSealed
)

// TxStatus represents the current status of a Transaction.
type TxStatus int

const (
	TxPending TxStatus = iota
	TxFinalized
	TxReverted
	TxSealed
)

func (s BlockStatus) String() string {
	return [...]string{"PENDING", "SEALED"}[s]
}

func (s TxStatus) String() string {
	return [...]string{"PENDING", "FINALIZED", "REVERTED", "SEALED"}[s]
}
