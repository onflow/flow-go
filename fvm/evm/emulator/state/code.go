package state

import (
	"encoding/binary"
	"fmt"
)

// CodeContainer contains codes and keeps
// track of reference counts
type CodeContainer struct {
	code     []byte
	refCount uint64
}

// NewCodeContainer constructs a new code container
func NewCodeContainer(code []byte) *CodeContainer {
	return &CodeContainer{
		code:     code,
		refCount: 1,
	}
}

// CodeContainerFromEncoded constructs a code container from the encoded data
func CodeContainerFromEncoded(encoded []byte) (*CodeContainer, error) {
	if len(encoded) < 8 {
		return nil, fmt.Errorf("invalid length for the encoded code container")
	}
	return &CodeContainer{
		refCount: binary.BigEndian.Uint64(encoded[:8]),
		code:     encoded[8:],
	}, nil
}

// Code returns the code part of the code container
func (cc *CodeContainer) Code() []byte {
	return cc.code
}

// RefCount returns the ref count
func (cc *CodeContainer) RefCount() uint64 {
	return cc.refCount
}

// IncRefCount increment the ref count
func (cc *CodeContainer) IncRefCount() {
	cc.refCount++
}

// DecRefCount decrement the ref count and
// returns true if the ref has reached to zero
func (cc *CodeContainer) DecRefCount() bool {
	cc.refCount--
	return cc.refCount == 0
}

// Encoded returns the encoded content of the code container
func (cc *CodeContainer) Encoded() []byte {
	encoded := make([]byte, 8+len(cc.code))
	binary.BigEndian.PutUint64(encoded[:8], cc.refCount)
	copy(encoded[8:], cc.code)
	return encoded
}
