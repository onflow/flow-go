package ast

//go:generate stringer -type=Operation

type Operation int

const (
	OperationOr Operation = iota
	OperationAnd
	OperationEqual
	OperationUnequal
	OperationLess
	OperationGreater
	OperationLessEqual
	OperationGreaterEqual
	OperationPlus
	OperationMinus
	OperationMul
	OperationDiv
	OperationMod
)
