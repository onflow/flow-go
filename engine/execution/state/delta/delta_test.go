package delta_test

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestDelta_Get(t *testing.T) {
	registerID1 := "fruit"

	t.Run("ValueNotSet", func(t *testing.T) {
		d := delta.NewDelta()

		b, exists := d.Get(registerID1, "", "")
		assert.Nil(t, b)
		assert.False(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := delta.NewDelta()

		d.Set(registerID1, "", "", []byte("apple"))

		b, exists := d.Get(registerID1, "", "")
		assert.Equal(t, flow.RegisterValue("apple"), b)
		assert.True(t, exists)
	})
}

func TestDelta_Set(t *testing.T) {
	registerID1 := "fruit"

	d := delta.NewDelta()

	d.Set(registerID1, "", "", []byte("apple"))

	b1, exists := d.Get(registerID1, "", "")
	assert.Equal(t, []byte("apple"), b1)
	assert.True(t, exists)

	d.Set(registerID1, "", "", []byte("orange"))

	b2, exists := d.Get(registerID1, "", "")
	assert.Equal(t, []byte("orange"), b2)
	assert.True(t, exists)
}

func TestDelta_Delete(t *testing.T) {
	registerID1 := "fruit"

	t.Run("ValueNotSet", func(t *testing.T) {
		d := delta.NewDelta()

		d.Delete(registerID1, "", "")

		b, exists := d.Get(registerID1, "", "")
		assert.Nil(t, b)
		assert.True(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := delta.NewDelta()

		d.Set(registerID1, "", "", []byte("apple"))
		d.Delete(registerID1, "", "")

		b, exists := d.Get(registerID1, "", "")
		assert.Nil(t, b)
		assert.True(t, exists)
	})
}

func TestDelta_MergeWith(t *testing.T) {
	registerID1 := "fruit"

	registerID2 := "vegetable"

	t.Run("NoCollisions", func(t *testing.T) {
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, "", "", []byte("apple"))
		d2.Set(registerID2, "", "", []byte("carrot"))

		d1.MergeWith(d2)

		b1, _ := d1.Get(registerID1, "", "")
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, _ := d1.Get(registerID2, "", "")
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, "", "", flow.RegisterValue("apple"))
		d2.Set(registerID1, "", "", flow.RegisterValue("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get(registerID1, "", "")
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, "", "", flow.RegisterValue("apple"))
		d1.Delete(registerID1, "", "")

		d2.Set(registerID1, "", "", flow.RegisterValue("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get(registerID1, "", "")
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		d1 := delta.NewDelta()
		d2 := delta.NewDelta()

		d1.Set(registerID1, "", "", flow.RegisterValue("apple"))

		d2.Delete(registerID1, "", "")

		d1.MergeWith(d2)

		b, exists := d1.Get(registerID1, "", "")
		assert.Nil(t, b)
		assert.True(t, exists)
		assert.True(t, d1.HasBeenDeleted(registerID1, "", ""))
	})
}

type key struct {
	key    string
	hashed flow.RegisterID
	value  flow.RegisterValue
}

type keys []key

func (s keys) Less(i, j int) bool {
	return bytes.Compare(s[i].hashed, s[j].hashed) < 0
}

func (s keys) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s keys) Len() int {
	return len(s)
}

func (s keys) Hashed() (r []flow.RegisterID) {
	for _, k := range s {
		r = append(r, k.hashed)
	}
	return
}

func (s keys) Value() (r []flow.RegisterValue) {
	for _, k := range s {
		r = append(r, k.value)
	}
	return
}

func TestDelta_RegisterUpdatesAreSorted(t *testing.T) {

	d := delta.NewDelta()

	data := make(keys, 5)

	data[0].key = string([]byte{0, 0, 11})
	data[1].key = string([]byte{1})
	data[2].key = string([]byte{2})
	data[3].key = string([]byte{3})
	data[4].key = string([]byte{11, 0, 0})

	data[0].value = flow.RegisterValue("a")
	data[1].value = flow.RegisterValue("b")
	data[2].value = flow.RegisterValue("c")
	data[3].value = flow.RegisterValue("d")
	data[4].value = flow.RegisterValue("e")

	for i, k := range data {
		data[i].hashed = state.RegisterID(k.key, "", "")
	}

	sort.Sort(data)

	// set in random order
	d.Set(data[2].key, "", "", data[2].value)
	d.Set(data[1].key, "", "", data[1].value)
	d.Set(data[3].key, "", "", data[3].value)
	d.Set(data[0].key, "", "", data[0].value)
	d.Set(data[4].key, "", "", data[4].value)

	retKeys, retValues := d.RegisterUpdates()

	assert.Equal(t, data.Hashed(), retKeys)
	assert.Equal(t, data.Value(), retValues)
}
