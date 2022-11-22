package delta_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
)

func TestBlockViewGet(t *testing.T) {
	registerID := "fruit"

	t.Run("ValueNotSet", func(t *testing.T) {
		v := delta.NewBlockView(func(owner, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		b, err := v.Get(registerID, "")
		assert.NoError(t, err)
		assert.Nil(t, b)
	})

	t.Run("ValueNotInCache", func(t *testing.T) {
		v := delta.NewBlockView(func(owner, key string) (flow.RegisterValue, error) {
			if owner == registerID {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})
		b, err := v.Get(registerID, "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := delta.NewBlockView(func(owner, key string) (flow.RegisterValue, error) {
			if owner == registerID {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		err := v.Set(registerID, "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		b, err := v.Get(registerID, "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b)
	})
}

func TestBlockViewSet(t *testing.T) {
	registerID := "fruit"

	v := delta.NewBlockView(func(owner, key string) (flow.RegisterValue, error) {
		return nil, nil
	})

	err := v.Set(registerID, "", flow.RegisterValue("apple"))
	assert.NoError(t, err)

	b1, err := v.Get(registerID, "")
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("apple"), b1)

	err = v.Set(registerID, "", flow.RegisterValue("orange"))
	assert.NoError(t, err)

	b2, err := v.Get(registerID, "")
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("orange"), b2)

	t.Run("AfterDelete", func(t *testing.T) {
		v := delta.NewBlockView(func(owner, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		err := v.Set(registerID, "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = v.Delete(registerID, "")
		assert.NoError(t, err)
		err = v.Set(registerID, "", flow.RegisterValue("orange"))
		assert.NoError(t, err)

		b, err := v.Get(registerID, "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})
}

func TestBlockView_Delete(t *testing.T) {
	registerID := "fruit"

	t.Run("ValueNotSet", func(t *testing.T) {
		v := delta.NewBlockView(func(owner, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		b1, err := v.Get(registerID, "")
		assert.NoError(t, err)
		assert.Nil(t, b1)

		err = v.Delete(registerID, "")
		assert.NoError(t, err)

		b2, err := v.Get(registerID, "")
		assert.NoError(t, err)
		assert.Nil(t, b2)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := delta.NewBlockView(func(owner, key string) (flow.RegisterValue, error) {
			if owner == registerID {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		err := v.Set(registerID, "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		b1, err := v.Get(registerID, "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		err = v.Delete(registerID, "")
		assert.NoError(t, err)

		b2, err := v.Get(registerID, "")
		assert.NoError(t, err)
		assert.Nil(t, b2)
	})
}
