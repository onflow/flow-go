package delta_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
)

func TestDelta_Get(t *testing.T) {
	registerID1 := flow.NewRegisterID("fruit", "")

	t.Run("ValueNotSet", func(t *testing.T) {
		d := delta.NewDelta()

		b, exists := d.Get(registerID1)
		assert.Nil(t, b)
		assert.False(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := delta.NewDelta()

		d.Set(registerID1, []byte("apple"))

		b, exists := d.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("apple"), b)
		assert.True(t, exists)
	})
}

func TestDelta_Set(t *testing.T) {
	registerID1 := flow.NewRegisterID("fruit", "")

	d := delta.NewDelta()

	d.Set(registerID1, []byte("apple"))

	b1, exists := d.Get(registerID1)
	assert.Equal(t, []byte("apple"), b1)
	assert.True(t, exists)

	d.Set(registerID1, []byte("orange"))

	b2, exists := d.Get(registerID1)
	assert.Equal(t, []byte("orange"), b2)
	assert.True(t, exists)
}

func TestDelta_MergeWith(t *testing.T) {
	registerID1 := flow.NewRegisterID("fruit", "")

	registerID2 := flow.NewRegisterID("vegetable", "")

	t.Run("NoCollisions", func(t *testing.T) {
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, []byte("apple"))
		d2.Set(registerID2, []byte("carrot"))

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
		d1.Set(registerID1, nil)

		d2.Set(registerID1, flow.RegisterValue("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get(registerID1)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, flow.RegisterValue("apple"))

		d2.Set(registerID1, nil)

		d1.MergeWith(d2)

		b, exists := d1.Get(registerID1)
		assert.Nil(t, b)
		assert.True(t, exists)
	})
}

func TestDelta_UpdatedRegistersAreSorted(t *testing.T) {

	d := delta.NewDelta()

	data := make(flow.RegisterEntries, 5)

	data[0].Key = flow.NewRegisterID("a", "1")
	data[1].Key = flow.NewRegisterID("b", "1")
	data[2].Key = flow.NewRegisterID("c", "1")
	data[3].Key = flow.NewRegisterID("d", "1")
	data[4].Key = flow.NewRegisterID("d", "2")

	data[0].Value = flow.RegisterValue("a")
	data[1].Value = flow.RegisterValue("b")
	data[2].Value = flow.RegisterValue("c")
	data[3].Value = flow.RegisterValue("d")
	data[4].Value = flow.RegisterValue("e")

	sort.Sort(data)

	// set in random order
	d.Set(data[2].Key, data[2].Value)
	d.Set(data[1].Key, data[1].Value)
	d.Set(data[3].Key, data[3].Value)
	d.Set(data[0].Key, data[0].Value)
	d.Set(data[4].Key, data[4].Value)

	ret := d.UpdatedRegisters()

	assert.Equal(t, data, ret)
}
