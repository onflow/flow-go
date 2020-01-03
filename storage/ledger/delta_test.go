package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/storage/ledger"
)

func TestDelta_Get(t *testing.T) {
	t.Run("ValueNotSet", func(t *testing.T) {
		d := ledger.NewDelta()

		b, exists := d.Get("fruit")
		assert.Nil(t, b)
		assert.False(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := ledger.NewDelta()

		d.Set("fruit", []byte("apple"))

		b, exists := d.Get("fruit")
		assert.Equal(t, []byte("apple"), b)
		assert.True(t, exists)
	})
}

func TestDelta_Set(t *testing.T) {
	d := ledger.NewDelta()

	d.Set("fruit", []byte("apple"))

	b1, exists := d.Get("fruit")
	assert.Equal(t, []byte("apple"), b1)
	assert.True(t, exists)

	d.Set("fruit", []byte("orange"))

	b2, exists := d.Get("fruit")
	assert.Equal(t, []byte("orange"), b2)
	assert.True(t, exists)
}

func TestDelta_Delete(t *testing.T) {
	t.Run("ValueNotSet", func(t *testing.T) {
		d := ledger.NewDelta()

		d.Delete("fruit")

		b, exists := d.Get("fruit")
		assert.Nil(t, b)
		assert.True(t, exists)
	})

	t.Run("ValueSet", func(t *testing.T) {
		d := ledger.NewDelta()

		d.Set("fruit", []byte("apple"))
		d.Delete("fruit")

		b, exists := d.Get("fruit")
		assert.Nil(t, b)
		assert.True(t, exists)
	})
}

func TestDelta_MergeWith(t *testing.T) {
	t.Run("NoCollisions", func(t *testing.T) {
		d1 := ledger.NewDelta()
		d2 := ledger.NewDelta()

		d1.Set("fruit", []byte("apple"))
		d2.Set("vegetable", []byte("carrot"))

		d1.MergeWith(d2)

		b1, _ := d1.Get("fruit")
		assert.Equal(t, []byte("apple"), b1)

		b2, _ := d1.Get("vegetable")
		assert.Equal(t, []byte("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		d1 := ledger.NewDelta()
		d2 := ledger.NewDelta()

		d1.Set("fruit", []byte("apple"))
		d2.Set("fruit", []byte("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get("fruit")
		assert.Equal(t, []byte("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		d1 := ledger.NewDelta()
		d2 := ledger.NewDelta()

		d1.Set("fruit", []byte("apple"))
		d1.Delete("fruit")

		d2.Set("fruit", []byte("orange"))

		d1.MergeWith(d2)

		b, _ := d1.Get("fruit")
		assert.Equal(t, []byte("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		d1 := ledger.NewDelta()
		d2 := ledger.NewDelta()

		d1.Set("fruit", []byte("apple"))

		d2.Delete("fruit")

		d1.MergeWith(d2)

		b, exists := d1.Get("fruit")
		assert.Nil(t, b)
		assert.True(t, exists)
		assert.True(t, d1.HasBeenDeleted("fruit"))
	})
}
