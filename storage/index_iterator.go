package storage

import "iter"

type IndexIterator[T any, C any] iter.Seq[IteratorEntry[T, C]]

type ReconstructFunc[T any, C any] func(C, []byte, *T) error

type DecodeKeyFunc[C any] func([]byte) (C, error)

type IteratorEntry[T any, C any] interface {
	Cursor() (C, error)
	Value() (T, error)
}
