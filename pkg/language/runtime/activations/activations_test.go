package activations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActivations(t *testing.T) {
	activations := &Activations{}

	activations.Set("a", 1)

	assert.Equal(t, activations.Find("a"), 1)
	assert.Nil(t, activations.Find("b"))

	activations.PushCurrent()

	activations.Set("a", 2)
	activations.Set("b", 3)

	assert.Equal(t, activations.Find("a"), 2)
	assert.Equal(t, activations.Find("b"), 3)
	assert.Nil(t, activations.Find("c"))

	activations.PushCurrent()

	activations.Set("a", 5)
	activations.Set("c", 4)

	assert.Equal(t, activations.Find("a"), 5)
	assert.Equal(t, activations.Find("b"), 3)
	assert.Equal(t, activations.Find("c"), 4)

	activations.Pop()

	assert.Equal(t, activations.Find("a"), 2)
	assert.Equal(t, activations.Find("b"), 3)
	assert.Nil(t, activations.Find("c"))

	activations.Pop()

	assert.Equal(t, activations.Find("a"), 1)
	assert.Nil(t, activations.Find("b"))
	assert.Nil(t, activations.Find("c"))

	activations.Pop()

	assert.Nil(t, activations.Find("a"))
	assert.Nil(t, activations.Find("b"))
	assert.Nil(t, activations.Find("c"))
}
