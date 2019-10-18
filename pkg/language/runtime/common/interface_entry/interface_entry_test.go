package interface_entry

import (
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/raviqqe/hamt"
)

func TestInterfaceEntry(t *testing.T) {
	type X struct{}
	x := X{}

	m := hamt.NewMap()
	m = m.Insert(InterfaceEntry{&x}, 42)

	assert.Equal(t, 42, m.Find(InterfaceEntry{&x}))
}
