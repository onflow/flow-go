package sema

type Variable struct {
	IsConstant     bool
	Depth          int
	Type           Type
	ArgumentLabels []string
}
