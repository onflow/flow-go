package ast

type Argument struct {
	Label         string
	LabelStartPos *Position
	LabelEndPos   *Position
	Expression    Expression
}
