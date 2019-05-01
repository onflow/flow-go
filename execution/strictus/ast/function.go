package ast

type Function struct {
	IsPublic   bool
	Identifier string
	Parameters []Parameter
	ReturnType Type
	Statements []Statement
}
