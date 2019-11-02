package sema

//go:generate stringer -type=ContainerKind

type ContainerKind int

const (
	ContainerKindUnknown ContainerKind = iota
	ContainerKindInterface
	ContainerKindComposite
)
