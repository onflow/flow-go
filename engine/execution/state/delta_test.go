package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestDelta_Get(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	t.Run("ValueNotSet", func(t *testing.T) {
		d := state.NewDelta()

		b, exists := d.Get(registerID1)
		assert.Nil(t, b)
		assert.False(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := state.NewDelta()

		d.Set(registerID1, flow.RegisterValue("apple"))

		b, exists := d.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("apple"), b)
		assert.True(t, exists)
	})
}

func TestDelta_Set(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	d := state.NewDelta()

	d.Set(registerID1, flow.RegisterValue("apple"))

	b1, exists := d.Get(registerID1)
	assert.Equal(t, flow.RegisterValue("apple"), b1)
	assert.True(t, exists)

	d.Set(registerID1, flow.RegisterValue("orange"))

	b2, exists := d.Get(registerID1)
	assert.Equal(t, flow.RegisterValue("orange"), b2)
	assert.True(t, exists)
}

func TestDelta_Delete(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	t.Run("ValueNotSet", func(t *testing.T) {
		d := state.NewDelta()

		d.Delete(registerID1)

		b, exists := d.Get(registerID1)
		assert.Nil(t, b)
		assert.True(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := state.NewDelta()

		d.Set(registerID1, flow.RegisterValue("apple"))
		d.Delete(registerID1)

		b, exists := d.Get(registerID1)
		assert.Nil(t, b)
		assert.True(t, exists)
	})
}

func TestDelta_MergeWith(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	registerID2 := make([]byte, 32)
	copy(registerID2, "vegetable")

	t.Run("NoCollisions", func(t *testing.T) {
		d1 := state.NewDelta()
		d2 := state.NewDelta()

		d1.Set(registerID1, flow.RegisterValue("apple"))
		d2.Set(registerID2, flow.RegisterValue("carrot"))

		d1.MergeWith(d2)

		b1, _ := d1.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, _ := d1.Get(registerID2)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		d1 := state.NewDelta()
		d2 := state.NewDelta()

		d1.Set(registerID1, flow.RegisterValue("apple"))
		d2.Set(registerID1, flow.RegisterValue("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		d1 := state.NewDelta()
		d2 := state.NewDelta()

		d1.Set(registerID1, flow.RegisterValue("apple"))
		d1.Delete(registerID1)

		d2.Set(registerID1, flow.RegisterValue("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		d1 := state.NewDelta()
		d2 := state.NewDelta()

		d1.Set(registerID1, flow.RegisterValue("apple"))

		d2.Delete(registerID1)

		d1.MergeWith(d2)

		b, exists := d1.Get(registerID1)
		assert.Nil(t, b)
		assert.True(t, exists)
		assert.True(t, d1.HasBeenDeleted(registerID1))
	})
}
