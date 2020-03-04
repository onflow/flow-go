package sema

//go:generate stringer -type=TypeAnnotationState

type TypeAnnotationState int

const (
	TypeAnnotationStateUnknown TypeAnnotationState = iota
	TypeAnnotationStateValid
	TypeAnnotationStateInvalidResourceAnnotation
	TypeAnnotationStateMissingResourceAnnotation
)
