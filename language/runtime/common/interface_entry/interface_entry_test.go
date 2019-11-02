package interface_entry

import (
	"testing"

	"github.com/raviqqe/hamt"
	"github.com/stretchr/testify/assert"
)

func TestInterfaceEntry(t *testing.T) {
	type X struct{}
	x := X{}

	m := hamt.NewMap()
	m = m.Insert(InterfaceEntry{&x}, 42)

	assert.Equal(t, 42, m.Find(InterfaceEntry{&x}))
}
