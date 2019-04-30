package ast

type Statement interface {
	isStatement()
}

type ReturnStatement struct {
	Expression Expression
}

func (ReturnStatement) isStatement() {}

type IfStatement struct {
	Test Expression
	Then Block
	Else Block
}

func (IfStatement) isStatement() {}

type WhileStatement struct {
	Test  Expression
	Block Block
}

func (WhileStatement) isStatement() {}

type VariableDeclaration struct {
	IsConst    bool
	Identifier string
	Type       *Type
	Value      Expression
}

func (VariableDeclaration) isStatement() {}

type Assignment struct {
	Identifier string
	Value      Expression
}

func (Assignment) isStatement() {}

type ExpressionStatement struct {
	Expression Expression
}

func (ExpressionStatement) isStatement() {}
