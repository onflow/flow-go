package ast

// Transfer represents the operation in variable declarations
// and assignments
//
type Transfer struct {
	Operation TransferOperation
	Pos       Position
}

func (f Transfer) StartPosition() Position {
	return f.Pos
}

func (f Transfer) EndPosition() Position {
	length := len(f.Operation.Operator())
	return f.Pos.Shifted(length - 1)
}
