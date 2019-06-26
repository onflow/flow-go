package ast

type Block struct {
	Statements    []Statement
	StartPosition Position
	EndPosition   Position
}

func (b Block) Accept(visitor Visitor) Repr {
	return visitor.VisitBlock(b)
}
