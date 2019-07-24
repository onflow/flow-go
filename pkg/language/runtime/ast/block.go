package ast

type Block struct {
	Statements []Statement
	StartPos   *Position
	EndPos     *Position
}

func (b *Block) Accept(visitor Visitor) Repr {
	return visitor.VisitBlock(b)
}
