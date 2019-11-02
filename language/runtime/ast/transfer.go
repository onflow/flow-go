package ast

// Transfer represents the operation in variable declarations
// and assignments
//
type Transfer struct {
	Operation TransferOperation
	Pos       Position
}
