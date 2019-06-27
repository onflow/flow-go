package ast

type Parameter struct {
	Identifier    string
	Type          Type
	StartPosition Position
	EndPosition   Position
}

func (p Parameter) GetStartPosition() Position {
	return p.StartPosition
}

func (p Parameter) GetEndPosition() Position {
	return p.EndPosition
}
