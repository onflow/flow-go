package common

import (
	"github.com/raviqqe/hamt"
	"github.com/segmentio/fasthash/fnv1a"
)

type StringEntry string

func (key StringEntry) Hash() uint32 {
	return fnv1a.HashString32(string(key))
}

func (key StringEntry) Equal(other hamt.Entry) bool {
	otherKey, isPointerKey := other.(StringEntry)
	return isPointerKey && string(otherKey) == string(key)
}
