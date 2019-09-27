package sema

//go:generate stringer -type=initializerKind

type initializerKind int

const (
	initializerKindUnknown initializerKind = iota
	initializerKindComposite
	initializerKindInterface
)
