package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/execution/state"
)

func TestView_Get(t *testing.T) {
	t.Run("ValueNotSet", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Nil(t, b)
	})

	t.Run("ValueNotInCache", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			if key == "fruit" {
				return []byte("orange"), nil
			}

			return nil, nil
		})

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("orange"), b)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			if key == "fruit" {
				return []byte("orange"), nil
			}

			return nil, nil
		})

		v.Set("fruit", []byte("apple"))

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("apple"), b)
	})
}

func TestView_Set(t *testing.T) {
	v := state.NewView(func(key string) ([]byte, error) {
		return nil, nil
	})

	v.Set("fruit", []byte("apple"))

	b1, err := v.Get("fruit")
	assert.NoError(t, err)
	assert.Equal(t, []byte("apple"), b1)

	v.Set("fruit", []byte("orange"))

	b2, err := v.Get("fruit")
	assert.NoError(t, err)
	assert.Equal(t, []byte("orange"), b2)

	t.Run("AfterDelete", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))
		v.Delete("fruit")
		v.Set("fruit", []byte("orange"))

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("orange"), b)

		delta := v.Delta()
		assert.False(t, delta.HasBeenDeleted("fruit"))
	})
}

func TestView_Delete(t *testing.T) {
	t.Run("ValueNotSet", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		b1, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Nil(t, b1)

		v.Delete("fruit")

		b2, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Nil(t, b2)

		delta := v.Delta()
		assert.True(t, delta.HasBeenDeleted("fruit"))
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			if key == "fruit" {
				return []byte("orange"), nil
			}

			return nil, nil
		})

		v.Set("fruit", []byte("apple"))

		b1, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("apple"), b1)

		v.Delete("fruit")

		b2, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Nil(t, b2)

		delta := v.Delta()
		assert.True(t, delta.HasBeenDeleted("fruit"))
	})
}

func TestView_ApplyDelta(t *testing.T) {
	t.Run("EmptyView", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		d := state.NewDelta()
		d.Set("fruit", []byte("apple"))
		d.Set("vegetable", []byte("carrot"))

		v.ApplyDelta(d)

		b1, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("apple"), b1)

		b2, err := v.Get("vegetable")
		assert.NoError(t, err)
		assert.Equal(t, []byte("carrot"), b2)
	})

	t.Run("EmptyDelta", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))
		v.Set("vegetable", []byte("carrot"))

		d := state.NewDelta()

		v.ApplyDelta(d)

		b1, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("apple"), b1)

		b2, err := v.Get("vegetable")
		assert.NoError(t, err)
		assert.Equal(t, []byte("carrot"), b2)
	})

	t.Run("NoCollisions", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))

		d := state.NewDelta()
		d.Set("vegetable", []byte("carrot"))

		v.ApplyDelta(d)

		b1, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("apple"), b1)

		b2, err := v.Get("vegetable")
		assert.NoError(t, err)
		assert.Equal(t, []byte("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))

		d := state.NewDelta()
		d.Set("fruit", []byte("orange"))

		v.ApplyDelta(d)

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))
		v.Delete("fruit")

		d := state.NewDelta()
		d.Set("fruit", []byte("orange"))

		v.ApplyDelta(d)

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))

		d := state.NewDelta()
		d.Delete("fruit")

		v.ApplyDelta(d)

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Nil(t, b)
	})
}

func TestView_Reads(t *testing.T) {
	v := state.NewView(func(key string) ([]byte, error) {
		return nil, nil
	})

	t.Run("Empty", func(t *testing.T) {
		reads := v.Reads()
		assert.Empty(t, reads)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))

		// cache reads are not recorded
		_, err := v.Get("fruit")
		assert.NoError(t, err)

		// read list should be empty
		reads := v.Reads()
		assert.Empty(t, reads)
	})

	t.Run("ValuesNotInCache", func(t *testing.T) {
		v := state.NewView(func(key string) ([]byte, error) {
			if key == "fruit" {
				return []byte("orange"), nil
			}

			if key == "vegetable" {
				return []byte("carrot"), nil
			}

			return nil, nil
		})

		_, err := v.Get("fruit")
		assert.NoError(t, err)

		_, err = v.Get("vegetable")
		assert.NoError(t, err)

		reads := v.Reads()
		assert.Len(t, reads, 2)

		assert.Equal(t, "fruit", reads[0])
		assert.Equal(t, "vegetable", reads[1])
	})
}
