package ast

type Block struct {
	Statements []Statement
	StartPos   Position
	EndPos     Position
}

func (b *Block) Accept(visitor Visitor) Repr {
	return visitor.VisitBlock(b)
}

func (b *Block) StartPosition() Position {
	return b.StartPos
}

func (b *Block) EndPosition() Position {
	return b.EndPos
}

// FunctionBlock

type FunctionBlock struct {
	*Block
	PreConditions  []*Condition
	PostConditions []*Condition
}

func (b *FunctionBlock) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionBlock(b)
}

// Condition

type Condition struct {
	Expression
}
