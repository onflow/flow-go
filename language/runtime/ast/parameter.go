package ast

type Parameter struct {
	Label          string
	Identifier     Identifier
	TypeAnnotation *TypeAnnotation
	Range
}

// EffectiveArgumentLabel returns the effective argument label that
// an argument in a call must use:
// If no argument label is declared for parameter,
// the parameter name is used as the argument label
//
func (p Parameter) EffectiveArgumentLabel() string {
	if p.Label != "" {
		return p.Label
	}
	return p.Identifier.Identifier
}

type ParameterList struct {
	Parameters []*Parameter
	Range
}

// EffectiveArgumentLabels returns the effective argument labels that
// the arguments of a call must use:
// If no argument label is declared for parameter,
// the parameter name is used as the argument label
//
func (l *ParameterList) EffectiveArgumentLabels() []string {
	argumentLabels := make([]string, len(l.Parameters))

	for i, parameter := range l.Parameters {
		argumentLabels[i] = parameter.EffectiveArgumentLabel()
	}

	return argumentLabels
}
