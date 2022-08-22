package programs

// TODO(patrick): rm
type Programs = BlockPrograms

// TODO(patrick): rm
type ModifiedSets = ModifiedSetsInvalidator

// TODO(patrick): rm
func (p *BlockPrograms) ChildPrograms() *BlockPrograms {
	return p.NewChildBlockPrograms()
}

// TODO(patrick): rm
func (p *BlockPrograms) HasChanges() bool {
	return true
}

// DEPRECATED.  DO NOT USE
//
// TODO(patrick): remove after emulator is updated
func NewEmptyPrograms() *TransactionPrograms {
	blockProgs := NewEmptyBlockPrograms()
	txnProgs, err := blockProgs.NewTransactionPrograms(0, 0)
	if err != nil {
		panic(err)
	}
	return txnProgs
}
