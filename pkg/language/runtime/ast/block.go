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
}

func (b *FunctionBlock) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionBlock(b)
}

func (b *FunctionBlock) StartPosition() Position {
	return b.StartPos
}

func (b *FunctionBlock) EndPosition() Position {
	return b.EndPos
}
