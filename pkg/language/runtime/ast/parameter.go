package ast

type Parameter struct {
	Label          string
	Identifier     Identifier
	TypeAnnotation *TypeAnnotation
	Range
}

type ParameterList struct {
	Parameters []*Parameter
	Range
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
