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
	PreConditions  *Conditions
	PostConditions *Conditions
}

func (b *FunctionBlock) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionBlock(b)
}

func (b *FunctionBlock) PrependPostConditions(conditions Conditions) {
	if b.PostConditions == nil {
		b.PostConditions = &conditions
	} else {
		postConditions := append(conditions, *b.PostConditions...)
		b.PostConditions = &postConditions
	}
}

func (b *FunctionBlock) PrependPreConditions(conditions Conditions) {
	if b.PreConditions == nil {
		b.PreConditions = &conditions
	} else {
		preConditions := append(conditions, *b.PreConditions...)
		b.PreConditions = &preConditions
	}
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

type Conditions []*Condition

func (c *Conditions) Append(conditions Conditions) {
	if c == nil {
		return
	}
	*c = append(*c, conditions...)
}
