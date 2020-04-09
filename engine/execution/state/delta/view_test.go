package delta_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestView_Get(t *testing.T) {
	registerID := make([]byte, 32)
	copy(registerID, "fruit")

	t.Run("ValueNotSet", func(t *testing.T) {
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		b, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Nil(t, b)
	})

	t.Run("ValueNotInCache", func(t *testing.T) {
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
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
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
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

	v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
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
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
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

	t.Run("SpockSecret", func(t *testing.T) {
		v := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})
		registerID1 := make([]byte, 32)
		copy(registerID1, "reg1")

		registerID2 := make([]byte, 32)
		copy(registerID2, "reg2")

		registerID3 := make([]byte, 32)
		copy(registerID3, "reg3")

		// this part checks that spocks ordering be based
		// on update orders and not registerIDs
		v.Set(registerID2, flow.RegisterValue("1"))
		v.Set(registerID3, flow.RegisterValue("2"))
		v.Set(registerID1, flow.RegisterValue("3"))
		b, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, b, flow.RegisterValue("3"))
		// this part checks that delete functionality
		// doesn't impact secret
		v.Delete(registerID1)
		// this part checks that it always update the
		// intermediate values and not just the final values
		v.Set(registerID1, flow.RegisterValue("4"))
		v.Set(registerID1, flow.RegisterValue("5"))
		v.Set(registerID3, flow.RegisterValue("6"))

		s := v.SpockSecret()
		assert.Equal(t, s, []byte("1233456"))
	})
}

func TestView_Delete(t *testing.T) {
	registerID := make([]byte, 32)
	copy(registerID, "fruit")

	t.Run("ValueNotSet", func(t *testing.T) {
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
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
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
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

func TestView_MergeView(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	registerID2 := make([]byte, 32)
	copy(registerID2, "vegetable")

	t.Run("EmptyView", func(t *testing.T) {
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		chView := v.NewChild()
		chView.Set(registerID1, flow.RegisterValue("apple"))
		chView.Set(registerID2, flow.RegisterValue("carrot"))

		v.MergeView(chView)

		b1, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("EmptyDelta", func(t *testing.T) {
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))
		v.Set(registerID2, flow.RegisterValue("carrot"))

		chView := v.NewChild()
		v.MergeView(chView)

		b1, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("NoCollisions", func(t *testing.T) {
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))

		chView := v.NewChild()
		chView.Set(registerID2, flow.RegisterValue("carrot"))
		v.MergeView(chView)

		b1, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))

		chView := v.NewChild()
		chView.Set(registerID1, flow.RegisterValue("orange"))
		v.MergeView(chView)

		b, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))
		v.Delete(registerID1)

		chView := v.NewChild()
		chView.Set(registerID1, flow.RegisterValue("orange"))
		v.MergeView(chView)

		b, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		v.Set(registerID1, flow.RegisterValue("apple"))

		chView := v.NewChild()
		chView.Delete(registerID1)
		v.MergeView(chView)

		b, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Nil(t, b)
	})
}

func TestView_RegisterTouches(t *testing.T) {
	registerID1 := make([]byte, 32)
	copy(registerID1, "fruit")

	registerID2 := make([]byte, 32)
	copy(registerID2, "vegetable")

	v := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
		return nil, nil
	})

	t.Run("Empty", func(t *testing.T) {
		touches := v.RegisterTouches()
		assert.Empty(t, touches)
	})

	t.Run("Set and Get", func(t *testing.T) {
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

		v.Set(registerID2, flow.RegisterValue("apple"))

		touches := v.RegisterTouches()
		assert.Len(t, touches, 2)
	})
}
