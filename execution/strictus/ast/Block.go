package ast

type Block struct {
	Statements []Statement
}

func (b Block) Accept(visitor Visitor) Repr {
	return visitor.VisitBlock(b)
}
