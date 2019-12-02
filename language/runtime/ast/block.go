package ast

type Block struct {
	Statements []Statement
	Range
}

func (b *Block) Accept(visitor Visitor) Repr {
	return visitor.VisitBlock(b)
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
	Kind    ConditionKind
	Test    Expression
	Message Expression
}

func (c *Condition) Accept(visitor Visitor) Repr {
	return visitor.VisitCondition(c)
}
