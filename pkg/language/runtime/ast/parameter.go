package ast

type Parameter struct {
	Label          string
	Identifier     Identifier
	TypeAnnotation *TypeAnnotation
	StartPos       Position
	EndPos         Position
}

func (p *Parameter) StartPosition() Position {
	return p.StartPos
}

func (p *Parameter) EndPosition() Position {
	return p.EndPos
}

type ParameterList struct {
	Parameters []*Parameter
	StartPos   Position
	EndPos     Position
}

func (l *ParameterList) ArgumentLabels() []string {
	argumentLabels := make([]string, len(l.Parameters))

	for i, parameter := range l.Parameters {
		argumentLabel := parameter.Label
		// if no argument label is given, the parameter name
		// is used as the argument labels and is required
		if argumentLabel == "" {
			argumentLabel = parameter.Identifier.Identifier
		}
		argumentLabels[i] = argumentLabel
	}

	return argumentLabels
}

func (l *ParameterList) StartPosition() Position {
	if len(l.Parameters) == 0 {
		return Position{}
	}
	firstParameter := l.Parameters[0]
	return firstParameter.StartPos
}

func (l *ParameterList) EndPosition() Position {
	count := len(l.Parameters)
	if count == 0 {
		return Position{}
	}
	lastParameter := l.Parameters[count-1]
	return lastParameter.EndPos
}
