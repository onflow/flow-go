package noop

import "io"

type Closer struct{}

var _ io.Closer = (*Closer)(nil)

func (Closer) Close() error { return nil }
