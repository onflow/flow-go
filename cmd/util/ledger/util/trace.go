package util

import "strings"

type Trace struct {
	value  string
	parent *Trace
}

func NewTrace(value string) *Trace {
	return &Trace{
		value: value,
	}
}

func (t *Trace) Append(value string) *Trace {
	return &Trace{
		value:  value,
		parent: t,
	}
}

func (t *Trace) String() string {
	var sb strings.Builder

	var size int
	var elements []*Trace
	for current := t; current != nil; current = current.parent {
		size += len(current.value)
		elements = append(elements, current)
	}
	sb.Grow(size)

	for i := len(elements) - 1; i >= 0; i-- {
		sb.WriteString(elements[i].value)
	}

	return sb.String()
}
