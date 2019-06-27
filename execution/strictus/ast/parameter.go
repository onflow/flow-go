package ast

type Parameter struct {
	Identifier    string
	Type          Type
	StartPosition Position
	EndPosition   Position
}
