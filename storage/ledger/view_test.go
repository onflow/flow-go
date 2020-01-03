package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestView_Get(t *testing.T) {
	t.Run("ValueNotSet", func(t *testing.T) {
		v := NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Nil(t, b)
	})

	t.Run("ValueNotInCache", func(t *testing.T) {
		v := NewView(func(key string) ([]byte, error) {
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
		v := NewView(func(key string) ([]byte, error) {
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
	v := NewView(func(key string) ([]byte, error) {
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
		v := NewView(func(key string) ([]byte, error) {
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
		v := NewView(func(key string) ([]byte, error) {
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
		v := NewView(func(key string) ([]byte, error) {
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
		v := NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		d := NewDelta()
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
		v := NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))
		v.Set("vegetable", []byte("carrot"))

		d := NewDelta()

		v.ApplyDelta(d)

		b1, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("apple"), b1)

		b2, err := v.Get("vegetable")
		assert.NoError(t, err)
		assert.Equal(t, []byte("carrot"), b2)
	})

	t.Run("NoCollisions", func(t *testing.T) {
		v := NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))

		d := NewDelta()
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
		v := NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))

		d := NewDelta()
		d.Set("fruit", []byte("orange"))

		v.ApplyDelta(d)

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		v := NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))
		v.Delete("fruit")

		d := NewDelta()
		d.Set("fruit", []byte("orange"))

		v.ApplyDelta(d)

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Equal(t, []byte("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		v := NewView(func(key string) ([]byte, error) {
			return nil, nil
		})

		v.Set("fruit", []byte("apple"))

		d := NewDelta()
		d.Delete("fruit")

		v.ApplyDelta(d)

		b, err := v.Get("fruit")
		assert.NoError(t, err)
		assert.Nil(t, b)
	})
}
