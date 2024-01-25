package state

import (
	"encoding/binary"
	"fmt"
)

// CodeContainer contains codes and keeps
// track of reference counts
type CodeContainer struct {
	code []byte
	// keeping encoded so we can reuse it later
	buffer   []byte
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
		buffer:   encoded, // keep encoded as buffer for future use
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
	// check if ref is already zero
	// this condition should never happen
	// but better to be here to prevent underflow
	if cc.refCount == 0 {
		return true
	}
	cc.refCount--
	return cc.refCount == 0
}

// Encoded returns the encoded content of the code container
func (cc *CodeContainer) Encode() []byte {
	// try using the buffer if possible to avoid
	// extra allocations
	encodedLen := 8 + len(cc.code)
	var encoded []byte
	if len(cc.buffer) < encodedLen {
		encoded = make([]byte, encodedLen)
	} else {
		encoded = cc.buffer[:encodedLen]
	}
	binary.BigEndian.PutUint64(encoded[:8], cc.refCount)
	copy(encoded[8:], cc.code)
	return encoded
}
