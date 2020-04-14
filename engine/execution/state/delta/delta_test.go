package delta_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestDelta_Get(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	t.Run("ValueNotSet", func(t *testing.T) {
		d := delta.NewDelta()

		b, exists := d.Get(registerID1)
		assert.Nil(t, b)
		assert.False(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := delta.NewDelta()

		d.Set(registerID1, flow.RegisterValue("apple"))

		b, exists := d.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("apple"), b)
		assert.True(t, exists)
	})
}

func TestDelta_Set(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	d := delta.NewDelta()

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
		d := delta.NewDelta()

		d.Delete(registerID1)

		b, exists := d.Get(registerID1)
		assert.Nil(t, b)
		assert.True(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := delta.NewDelta()

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
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, flow.RegisterValue("apple"))
		d2.Set(registerID2, flow.RegisterValue("carrot"))

		d1.MergeWith(d2)

		b1, _ := d1.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, _ := d1.Get(registerID2)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, flow.RegisterValue("apple"))
		d2.Set(registerID1, flow.RegisterValue("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, flow.RegisterValue("apple"))
		d1.Delete(registerID1)

		d2.Set(registerID1, flow.RegisterValue("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, flow.RegisterValue("apple"))

		d2.Delete(registerID1)

		d1.MergeWith(d2)

		b, exists := d1.Get(registerID1)
		assert.Nil(t, b)
		assert.True(t, exists)
		assert.True(t, d1.HasBeenDeleted(registerID1))
	})
}

func TestDelta_RegisterUpdatesAreSorted(t *testing.T) {

	d := delta.NewDelta()

	key := make([]flow.RegisterID, 5)
	value := make([]flow.RegisterValue, 5)

	key[0] = []byte{0, 0, 11}
	key[1] = []byte{1}
	key[2] = []byte{2}
	key[3] = []byte{3}
	key[4] = []byte{11, 0, 0}

	value[0] = flow.RegisterValue("a")
	value[1] = flow.RegisterValue("b")
	value[2] = flow.RegisterValue("c")
	value[3] = flow.RegisterValue("d")
	value[4] = flow.RegisterValue("e")

	// set in random order
	d.Set(key[2], value[2])
	d.Set(key[1], value[1])
	d.Set(key[3], value[3])
	d.Set(key[0], value[0])
	d.Set(key[4], value[4])

	retKeys, retValues := d.RegisterUpdates()

	assert.Equal(t, key, retKeys)
	assert.Equal(t, value, retValues)
}
