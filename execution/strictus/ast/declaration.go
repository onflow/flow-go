package ast

type Declaration interface {
	Element
	isDeclaration()
	DeclarationName() string
}
