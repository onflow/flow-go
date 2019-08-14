package ast

import "strings"

type Argument struct {
	Label         string
	LabelStartPos *Position
	LabelEndPos   *Position
	Expression    Expression
}

func (a Argument) String() string {
	var builder strings.Builder
	if a.Label != "" {
		builder.WriteString(a.Label)
		builder.WriteString(": ")
	}
	builder.WriteString(a.Expression.String())
	return builder.String()
}
