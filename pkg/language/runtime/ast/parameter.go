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

type Parameters []*Parameter

func (parameters Parameters) ArgumentLabels() []string {
	argumentLabels := make([]string, len(parameters))

	for i, parameter := range parameters {
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

func (parameters Parameters) StartPosition() Position {
	if len(parameters) == 0 {
		return Position{}
	}
	firstParameter := parameters[0]
	return firstParameter.StartPos
}

func (parameters Parameters) EndPosition() Position {
	count := len(parameters)
	if count == 0 {
		return Position{}
	}
	lastParameter := parameters[count-1]
	return lastParameter.EndPos
}
