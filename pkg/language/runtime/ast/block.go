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
