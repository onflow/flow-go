package ast

type Expression interface {
	isExpression()
}

type BoolExpression struct {
	Value bool
}

func (BoolExpression) isExpression() {}

type IntExpression struct {
	Value int64
}

func (IntExpression) isExpression() {}

type ArrayExpression struct {
	Values []Expression
}

func (ArrayExpression) isExpression() {}

type IdentifierExpression struct {
	Identifier string
}

func (IdentifierExpression) isExpression() {}

type InvocationExpression struct {
	Identifier string
	Arguments  []Expression
}

func (InvocationExpression) isExpression() {}

type MemberExpression struct {
	Expression Expression
	Identifier string
}

func (MemberExpression) isExpression() {}

type IndexExpression struct {
	Expression Expression
	Index      Expression
}

func (IndexExpression) isExpression() {}

type ConditionalExpression struct {
	Test Expression
	Then Expression
	Else Expression
}

func (ConditionalExpression) isExpression() {}

type BinaryExpression struct {
	Operation Operation
	Left      Expression
	Right     Expression
}

func (BinaryExpression) isExpression() {}
