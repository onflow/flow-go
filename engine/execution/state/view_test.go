package state_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestView_Get(t *testing.T) {
	registerID := make([]byte, 32)
	copy(registerID, "fruit")

	t.Run("ValueNotSet", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		b, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Nil(t, b)
	})

	t.Run("ValueNotInCache", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			if bytes.Equal(key, registerID) {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})
		b, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			if bytes.Equal(key, registerID) {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		v.Set(registerID, flow.RegisterValue("apple"))

		b, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b)
	})
}

func TestView_Set(t *testing.T) {
	registerID := make([]byte, 32)
	copy(registerID, "fruit")

	v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
		return nil, nil
	})

	v.Set(registerID, flow.RegisterValue("apple"))

	b1, err := v.Get(registerID)
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("apple"), b1)

	v.Set(registerID, flow.RegisterValue("orange"))

	b2, err := v.Get(registerID)
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("orange"), b2)

	t.Run("AfterDelete", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID, flow.RegisterValue("apple"))
		v.Delete(registerID)
		v.Set(registerID, flow.RegisterValue("orange"))

		b, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)

		delta := v.Delta()
		assert.False(t, delta.HasBeenDeleted(registerID))
	})
}

func TestView_Delete(t *testing.T) {
	registerID := make([]byte, 32)
	copy(registerID, "fruit")

	t.Run("ValueNotSet", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		b1, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Nil(t, b1)

		v.Delete(registerID)

		b2, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Nil(t, b2)

		delta := v.Delta()
		assert.True(t, delta.HasBeenDeleted(registerID))
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			if bytes.Equal(key, registerID) {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		v.Set(registerID, flow.RegisterValue("apple"))

		b1, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		v.Delete(registerID)

		b2, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Nil(t, b2)

		delta := v.Delta()
		assert.True(t, delta.HasBeenDeleted(registerID))
	})
}

func TestView_ApplyDelta(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	registerID2 := make([]byte, 32)
	copy(registerID2, "vegetable")

	t.Run("EmptyView", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		d := state.NewDelta()
		d.Set(registerID1, flow.RegisterValue("apple"))
		d.Set(registerID2, flow.RegisterValue("carrot"))

		v.ApplyDelta(d)

		b1, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("EmptyDelta", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))
		v.Set(registerID2, flow.RegisterValue("carrot"))

		d := state.NewDelta()

		v.ApplyDelta(d)

		b1, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("NoCollisions", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))

		d := state.NewDelta()
		d.Set(registerID2, flow.RegisterValue("carrot"))

		v.ApplyDelta(d)

		b1, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))

		d := state.NewDelta()
		d.Set(registerID1, flow.RegisterValue("orange"))

		v.ApplyDelta(d)

		b, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))
		v.Delete(registerID1)

		d := state.NewDelta()
		d.Set(registerID1, flow.RegisterValue("orange"))

		v.ApplyDelta(d)

		b, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))

		d := state.NewDelta()
		d.Delete(registerID1)

		v.ApplyDelta(d)

		b, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Nil(t, b)
	})
}

func TestView_Reads(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	registerID2 := make([]byte, 32)
	copy(registerID2, "vegetable")

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

		v.Set(registerID1, flow.RegisterValue("apple"))

		// cache reads are not recorded
		_, err := v.Get(registerID1)
		assert.NoError(t, err)

		// read list should be empty
		reads := v.Reads()
		assert.Empty(t, reads)
	})

	t.Run("ValuesNotInCache", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			if bytes.Equal(key, registerID1) {
				return flow.RegisterValue("orange"), nil
			}

			if bytes.Equal(key, registerID2) {
				return flow.RegisterValue("carrot"), nil
			}

			return nil, nil
		})

		_, err := v.Get(registerID1)
		assert.NoError(t, err)

		_, err = v.Get(registerID2)
		assert.NoError(t, err)

		reads := v.Reads()
		assert.Len(t, reads, 2)

		assert.Equal(t, registerID1, reads[0])
		assert.Equal(t, registerID2, reads[1])
	})
}
