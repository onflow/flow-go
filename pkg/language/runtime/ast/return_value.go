package ast

type ReturnValue struct {
	TypeAnnotation *TypeAnnotation
	StartPos       Position
	EndPos         Position
}

func (f *ReturnValue) StartPosition() Position {
	return f.StartPos
}

func (f *ReturnValue) EndPosition() Position {
	return f.EndPos
}
