package state_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestView_Get(t *testing.T) {
	t.Run("ValueNotSet", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		b, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Nil(t, b)
	})

	t.Run("ValueNotInCache", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			if bytes.Equal(key, flow.RegisterID("fruit")) {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		b, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			if bytes.Equal(key, flow.RegisterID("fruit")) {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

		b, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b)
	})
}

func TestView_Set(t *testing.T) {
	v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
		return nil, nil
	})

	v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

	b1, err := v.Get(flow.RegisterID("fruit"))
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("apple"), b1)

	v.Set(flow.RegisterID("fruit"), flow.RegisterValue("orange"))

	b2, err := v.Get(flow.RegisterID("fruit"))
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("orange"), b2)

	t.Run("AfterDelete", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))
		v.Delete(flow.RegisterID("fruit"))
		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("orange"))

		b, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)

		delta := v.Delta()
		assert.False(t, delta.HasBeenDeleted(flow.RegisterID("fruit")))
	})
}

func TestView_Delete(t *testing.T) {
	t.Run("ValueNotSet", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		b1, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Nil(t, b1)

		v.Delete(flow.RegisterID("fruit"))

		b2, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Nil(t, b2)

		delta := v.Delta()
		assert.True(t, delta.HasBeenDeleted(flow.RegisterID("fruit")))
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			if bytes.Equal(key, flow.RegisterID("fruit")) {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

		b1, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		v.Delete(flow.RegisterID("fruit"))

		b2, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Nil(t, b2)

		delta := v.Delta()
		assert.True(t, delta.HasBeenDeleted(flow.RegisterID("fruit")))
	})
}

func TestView_ApplyDelta(t *testing.T) {
	t.Run("EmptyView", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		d := state.NewDelta()
		d.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))
		d.Set(flow.RegisterID("vegetable"), flow.RegisterValue("carrot"))

		v.ApplyDelta(d)

		b1, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(flow.RegisterID("vegetable"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("EmptyDelta", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))
		v.Set(flow.RegisterID("vegetable"), flow.RegisterValue("carrot"))

		d := state.NewDelta()

		v.ApplyDelta(d)

		b1, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(flow.RegisterID("vegetable"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("NoCollisions", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

		d := state.NewDelta()
		d.Set(flow.RegisterID("vegetable"), flow.RegisterValue("carrot"))

		v.ApplyDelta(d)

		b1, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(flow.RegisterID("vegetable"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

		d := state.NewDelta()
		d.Set(flow.RegisterID("fruit"), flow.RegisterValue("orange"))

		v.ApplyDelta(d)

		b, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))
		v.Delete(flow.RegisterID("fruit"))

		d := state.NewDelta()
		d.Set(flow.RegisterID("fruit"), flow.RegisterValue("orange"))

		v.ApplyDelta(d)

		b, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

		d := state.NewDelta()
		d.Delete(flow.RegisterID("fruit"))

		v.ApplyDelta(d)

		b, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)
		assert.Nil(t, b)
	})
}

func TestView_Reads(t *testing.T) {
	v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
		return nil, nil
	})

	t.Run("Empty", func(t *testing.T) {
		reads := v.Reads()
		assert.Empty(t, reads)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(flow.RegisterID("fruit"), flow.RegisterValue("apple"))

		// cache reads are not recorded
		_, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)

		// read list should be empty
		reads := v.Reads()
		assert.Empty(t, reads)
	})

	t.Run("ValuesNotInCache", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			if bytes.Equal(key, flow.RegisterID("fruit")) {
				return flow.RegisterValue("orange"), nil
			}

			if bytes.Equal(key, flow.RegisterID("vegetable")) {
				return flow.RegisterValue("carrot"), nil
			}

			return nil, nil
		})

		_, err := v.Get(flow.RegisterID("fruit"))
		assert.NoError(t, err)

		_, err = v.Get(flow.RegisterID("vegetable"))
		assert.NoError(t, err)

		reads := v.Reads()
		assert.Len(t, reads, 2)

		assert.Equal(t, flow.RegisterID("fruit"), reads[0])
		assert.Equal(t, flow.RegisterID("vegetable"), reads[1])
	})
}
