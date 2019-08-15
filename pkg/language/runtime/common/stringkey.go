package common

import (
	"github.com/raviqqe/hamt"
	"github.com/segmentio/fasthash/fnv1a"
)

type StringKey string

func (key StringKey) Hash() uint32 {
	return fnv1a.HashString32(string(key))
}

func (key StringKey) Equal(other hamt.Entry) bool {
	otherKey, isPointerKey := other.(StringKey)
	return isPointerKey && string(otherKey) == string(key)
}
