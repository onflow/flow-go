package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestDelta_Get(t *testing.T) {
	t.Run("ValueNotSet", func(t *testing.T) {
		d := state.NewDelta()

		b, exists := d.Get(flow.RegisterID("fruit"))
		assert.Nil(t, b)
		assert.False(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := state.NewDelta()

		d.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

		b, exists := d.Get(flow.RegisterID("fruit"))
		assert.Equal(t, flow.RegisterValue("apple"), b)
		assert.True(t, exists)
	})
}

func TestDelta_Set(t *testing.T) {
	d := state.NewDelta()

	d.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

	b1, exists := d.Get(flow.RegisterID("fruit"))
	assert.Equal(t, flow.RegisterValue("apple"), b1)
	assert.True(t, exists)

	d.Set(flow.RegisterID("fruit"), flow.RegisterValue("orange"))

	b2, exists := d.Get(flow.RegisterID("fruit"))
	assert.Equal(t, flow.RegisterValue("orange"), b2)
	assert.True(t, exists)
}

func TestDelta_Delete(t *testing.T) {
	t.Run("ValueNotSet", func(t *testing.T) {
		d := state.NewDelta()

		d.Delete(flow.RegisterID("fruit"))

		b, exists := d.Get(flow.RegisterID("fruit"))
		assert.Nil(t, b)
		assert.True(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := state.NewDelta()

		d.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))
		d.Delete(flow.RegisterID("fruit"))

		b, exists := d.Get(flow.RegisterID("fruit"))
		assert.Nil(t, b)
		assert.True(t, exists)
	})
}

func TestDelta_MergeWith(t *testing.T) {
	t.Run("NoCollisions", func(t *testing.T) {
		d1 := state.NewDelta()
		d2 := state.NewDelta()

		d1.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))
		d2.Set(flow.RegisterID("vegetable"), flow.RegisterValue("carrot"))

		d1.MergeWith(d2)

		b1, _ := d1.Get(flow.RegisterID("fruit"))
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, _ := d1.Get(flow.RegisterID("vegetable"))
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		d1 := state.NewDelta()
		d2 := state.NewDelta()

		d1.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))
		d2.Set(flow.RegisterID("fruit"), flow.RegisterValue("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get(flow.RegisterID("fruit"))
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		d1 := state.NewDelta()
		d2 := state.NewDelta()

		d1.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))
		d1.Delete(flow.RegisterID("fruit"))

		d2.Set(flow.RegisterID("fruit"), flow.RegisterValue("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get(flow.RegisterID("fruit"))
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		d1 := state.NewDelta()
		d2 := state.NewDelta()

		d1.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

		d2.Delete(flow.RegisterID("fruit"))

		d1.MergeWith(d2)

		b, exists := d1.Get(flow.RegisterID("fruit"))
		assert.Nil(t, b)
		assert.True(t, exists)
		assert.True(t, d1.HasBeenDeleted(flow.RegisterID("fruit")))
	})
}
