package ast

type Expression interface {
	Element
	isExpression()
}

// BoolExpression

type BoolExpression bool

func (BoolExpression) isExpression() {}

func (e BoolExpression) Accept(v Visitor) Repr {
	return v.VisitBoolExpression(e)
}

// IntExpression

type IntExpression interface {
	Expression
	isIntExpression()
	IntValue() int
}

// Int8Expression

type Int8Expression int8

func (Int8Expression) isExpression() {}

func (e Int8Expression) Accept(v Visitor) Repr {
	return v.VisitInt8Expression(e)
}

func (Int8Expression) isIntExpression() {}

func (e Int8Expression) IntValue() int {
	return int(e)
}

// Int16Expression

type Int16Expression int16

func (Int16Expression) isExpression() {}

func (e Int16Expression) Accept(v Visitor) Repr {
	return v.VisitInt16Expression(e)
}

func (Int16Expression) isIntExpression() {}

func (e Int16Expression) IntValue() int {
	return int(e)
}

// Int32Expression

type Int32Expression int32

func (Int32Expression) isExpression() {}

func (e Int32Expression) Accept(v Visitor) Repr {
	return v.VisitInt32Expression(e)
}

func (Int32Expression) isIntExpression() {}

func (e Int32Expression) IntValue() int {
	return int(e)
}

// Int64Expression

type Int64Expression int64

func (Int64Expression) isExpression() {}

func (e Int64Expression) Accept(v Visitor) Repr {
	return v.VisitInt64Expression(e)
}

func (Int64Expression) isIntExpression() {}

func (e Int64Expression) IntValue() int {
	return int(e)
}

// UInt8Expression

type UInt8Expression uint8

func (UInt8Expression) isExpression() {}

func (e UInt8Expression) Accept(v Visitor) Repr {
	return v.VisitUInt8Expression(e)
}

func (UInt8Expression) isIntExpression() {}

func (e UInt8Expression) IntValue() int {
	return int(e)
}

// UInt16Expression

type UInt16Expression uint16

func (UInt16Expression) isExpression() {}

func (e UInt16Expression) Accept(v Visitor) Repr {
	return v.VisitUInt16Expression(e)
}

func (UInt16Expression) isIntExpression() {}

func (e UInt16Expression) IntValue() int {
	return int(e)
}

// UInt32Expression

type UInt32Expression uint32

func (UInt32Expression) isExpression() {}

func (e UInt32Expression) Accept(v Visitor) Repr {
	return v.VisitUInt32Expression(e)
}

func (UInt32Expression) isIntExpression() {}

func (e UInt32Expression) IntValue() int {
	return int(e)
}

// UInt64Expression

type UInt64Expression uint64

func (UInt64Expression) isExpression() {}

func (e UInt64Expression) Accept(v Visitor) Repr {
	return v.VisitUInt64Expression(e)
}

func (UInt64Expression) isIntExpression() {}

func (e UInt64Expression) IntValue() int {
	return int(e)
}

// ArrayExpression

type ArrayExpression struct {
	Values []Expression
}

func (ArrayExpression) isExpression() {}

func (e ArrayExpression) Accept(v Visitor) Repr {
	return v.VisitArrayExpression(e)
}

// IdentifierExpression

type IdentifierExpression struct {
	Identifier string
}

func (IdentifierExpression) isExpression() {}

func (e IdentifierExpression) Accept(v Visitor) Repr {
	return v.VisitIdentifierExpression(e)
}

// InvocationExpression

type InvocationExpression struct {
	Expression Expression
	Arguments  []Expression
}

func (InvocationExpression) isExpression() {}

func (e InvocationExpression) Accept(v Visitor) Repr {
	return v.VisitInvocationExpression(e)
}

// AccessExpression

type AccessExpression interface {
	isAccessExpression()
}

// MemberExpression

type MemberExpression struct {
	Expression Expression
	Identifier string
}

func (MemberExpression) isExpression()       {}
func (MemberExpression) isAccessExpression() {}

func (e MemberExpression) Accept(v Visitor) Repr {
	return v.VisitMemberExpression(e)
}

// IndexExpression

type IndexExpression struct {
	Expression Expression
	Index      Expression
}

func (IndexExpression) isExpression()       {}
func (IndexExpression) isAccessExpression() {}

func (e IndexExpression) Accept(v Visitor) Repr {
	return v.VisitIndexExpression(e)
}

// ConditionalExpression

type ConditionalExpression struct {
	Test Expression
	Then Expression
	Else Expression
}

func (ConditionalExpression) isExpression() {}

func (e ConditionalExpression) Accept(v Visitor) Repr {
	return v.VisitConditionalExpression(e)
}

// BinaryExpression

type BinaryExpression struct {
	Operation Operation
	Left      Expression
	Right     Expression
}

func (BinaryExpression) isExpression() {}

func (e BinaryExpression) Accept(v Visitor) Repr {
	return v.VisitBinaryExpression(e)
}

// FunctionExpression

type FunctionExpression struct {
	Parameters []Parameter
	ReturnType Type
	Block      Block
}

func (FunctionExpression) isExpression() {}

func (f FunctionExpression) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionExpression(f)
}
