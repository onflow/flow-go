package interface_entry

import (
	"unsafe"

	"github.com/raviqqe/hamt"
)

// InterfaceEntry allows using any pointer as an entry in a `hamt` structure
//
type InterfaceEntry struct {
	Interface interface{}
}

func (e InterfaceEntry) Hash() uint32 {
	// get the interface's inner pointer and use the address as the hash
	return uint32(uintptr((*struct {
		_       unsafe.Pointer
		pointer unsafe.Pointer
	})(unsafe.Pointer(&e.Interface)).pointer))
}

func (e InterfaceEntry) Equal(other hamt.Entry) bool {
	otherE, ok := other.(InterfaceEntry)
	if !ok {
		return false
	}

	return otherE == e
}
