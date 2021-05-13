package delta_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
)

func TestView_Get(t *testing.T) {
	registerID := "fruit"

	t.Run("ValueNotSet", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		b, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Nil(t, b)
	})

	t.Run("ValueNotInCache", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if owner == registerID {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})
		b, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if owner == registerID {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		err := v.Set(registerID, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		b, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b)
	})
}

func TestView_Set(t *testing.T) {
	registerID := "fruit"

	v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		return nil, nil
	})

	err := v.Set(registerID, "", "", flow.RegisterValue("apple"))
	assert.NoError(t, err)

	b1, err := v.Get(registerID, "", "")
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("apple"), b1)

	err = v.Set(registerID, "", "", flow.RegisterValue("orange"))
	assert.NoError(t, err)

	b2, err := v.Get(registerID, "", "")
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("orange"), b2)

	t.Run("AfterDelete", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		err := v.Set(registerID, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = v.Delete(registerID, "", "")
		assert.NoError(t, err)
		err = v.Set(registerID, "", "", flow.RegisterValue("orange"))
		assert.NoError(t, err)

		b, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("SpockSecret", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		t.Run("reflects in the snapshot", func(t *testing.T) {
			assert.Equal(t, v.SpockSecret(), v.Interactions().SpockSecret)
		})

		v = delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		registerID1 := "reg1"

		registerID2 := "reg2"
		registerID3 := "reg3"

		// this part checks that spocks ordering be based
		// on update orders and not registerIDs
		expSpock := hash.NewSHA3_256()
		err = v.Set(registerID2, "", "", flow.RegisterValue("1"))
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("1"))
		assert.NoError(t, err)

		err = v.Set(registerID3, "", "", flow.RegisterValue("2"))
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("2"))
		assert.NoError(t, err)

		err = v.Set(registerID1, "", "", flow.RegisterValue("3"))
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("3"))
		assert.NoError(t, err)

		b, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("3"))
		assert.NoError(t, err)

		assert.Equal(t, b, flow.RegisterValue("3"))
		// this part checks that delete functionality
		// doesn't impact secret
		err = v.Delete(registerID1, "", "")
		assert.NoError(t, err)

		// this part checks that it always update the
		// intermediate values and not just the final values
		err = v.Set(registerID1, "", "", flow.RegisterValue("4"))
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("4"))
		assert.NoError(t, err)

		err = v.Set(registerID1, "", "", flow.RegisterValue("5"))
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("5"))
		assert.NoError(t, err)

		err = v.Set(registerID3, "", "", flow.RegisterValue("6"))
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("6"))
		assert.NoError(t, err)

		s := v.SpockSecret()
		assert.Equal(t, s, []uint8(expSpock.SumHash()))

		t.Run("reflects in the snapshot", func(t *testing.T) {
			assert.Equal(t, v.SpockSecret(), v.Interactions().SpockSecret)
		})
	})
}

func TestView_Delete(t *testing.T) {
	registerID := "fruit"

	t.Run("ValueNotSet", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		b1, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Nil(t, b1)

		err = v.Delete(registerID, "", "")
		assert.NoError(t, err)

		b2, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Nil(t, b2)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if owner == registerID {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		err := v.Set(registerID, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		b1, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		err = v.Delete(registerID, "", "")
		assert.NoError(t, err)

		b2, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Nil(t, b2)
	})
}

func TestView_MergeView(t *testing.T) {
	registerID1 := "fruit"

	registerID2 := "vegetable"

	registerID3 := "diary"

	t.Run("EmptyView", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		chView := v.NewChild()
		err := chView.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = chView.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		assert.NoError(t, err)

		err = v.MergeView(chView)
		assert.NoError(t, err)

		b1, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("EmptyDelta", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		err := v.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = v.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = v.MergeView(chView)
		assert.NoError(t, err)

		b1, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("NoCollisions", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		err := v.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = chView.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		assert.NoError(t, err)

		err = v.MergeView(chView)
		assert.NoError(t, err)

		b1, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		err := v.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = chView.Set(registerID1, "", "", flow.RegisterValue("orange"))
		assert.NoError(t, err)
		err = v.MergeView(chView)
		assert.NoError(t, err)

		b, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		err := v.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = v.Delete(registerID1, "", "")
		assert.NoError(t, err)

		chView := v.NewChild()
		err = chView.Set(registerID1, "", "", flow.RegisterValue("orange"))
		assert.NoError(t, err)
		err = v.MergeView(chView)
		assert.NoError(t, err)

		b, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		err := v.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = chView.Delete(registerID1, "", "")
		assert.NoError(t, err)
		err = v.MergeView(chView)
		assert.NoError(t, err)

		b, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Nil(t, b)
	})
	t.Run("SpockDataMerge", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		expSpock1 := hash.NewSHA3_256()
		err := v.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = hashIt(expSpock1, []byte("apple"))
		assert.NoError(t, err)

		expSpock2 := hash.NewSHA3_256()
		chView := v.NewChild()
		err = chView.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		assert.NoError(t, err)
		err = hashIt(expSpock2, []byte("carrot"))
		assert.NoError(t, err)

		hash2 := expSpock2.SumHash()
		assert.Equal(t, chView.(*delta.View).SpockSecret(), []uint8(hash2))

		err = v.MergeView(chView)
		assert.NoError(t, err)

		err = hashIt(expSpock1, hash2)
		assert.NoError(t, err)
		assert.Equal(t, v.SpockSecret(), []uint8(expSpock1.SumHash()))
	})

	t.Run("RegisterTouchesDataMerge", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		err := v.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = chView.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		assert.NoError(t, err)
		err = chView.Set(registerID3, "", "", flow.RegisterValue("milk"))
		assert.NoError(t, err)

		err = v.MergeView(chView)
		assert.NoError(t, err)

		reads := v.Interactions().Reads

		require.Len(t, reads, 3)

		r1 := flow.NewRegisterID(registerID1, "", "")
		r2 := flow.NewRegisterID(registerID2, "", "")
		r3 := flow.NewRegisterID(registerID3, "", "")

		assert.Equal(t, map[string]flow.RegisterID{
			r1.String(): r1,
			r2.String(): r2,
			r3.String(): r3,
		}, reads)
	})

}

func TestView_RegisterTouches(t *testing.T) {
	registerID1 := "fruit"
	registerID2 := "vegetable"

	v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		return nil, nil
	})

	t.Run("Empty", func(t *testing.T) {
		touches := v.Interactions().RegisterTouches()
		assert.Empty(t, touches)
	})

	t.Run("Set and Get", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if owner == registerID1 {
				return flow.RegisterValue("orange"), nil
			}

			if owner == registerID2 {
				return flow.RegisterValue("carrot"), nil
			}

			return nil, nil
		})

		_, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)

		err = v.Set(registerID2, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		touches := v.Interactions().RegisterTouches()
		assert.Len(t, touches, 2)
	})
}

func TestView_AllRegisters(t *testing.T) {
	v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		return nil, nil
	})

	t.Run("Empty", func(t *testing.T) {
		regs := v.Interactions().AllRegisters()
		assert.Empty(t, regs)
	})

	t.Run("Set and Get", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if owner == "a" {
				return flow.RegisterValue("a_value"), nil
			}

			if owner == "b" {
				return flow.RegisterValue("b_value"), nil
			}
			return nil, nil
		})

		_, err := v.Get("a", "", "")
		assert.NoError(t, err)

		_, err = v.Get("b", "", "")
		assert.NoError(t, err)

		err = v.Set("c", "", "", flow.RegisterValue("c_value"))
		assert.NoError(t, err)

		err = v.Set("d", "", "", flow.RegisterValue("d_value"))
		assert.NoError(t, err)

		err = v.Touch("e", "", "")
		assert.NoError(t, err)
		err = v.Touch("f", "", "")
		assert.NoError(t, err)

		allRegs := v.Interactions().AllRegisters()
		assert.Len(t, allRegs, 6)
	})
	t.Run("With Merge", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if owner == "a" {
				return flow.RegisterValue("a_value"), nil
			}

			if owner == "b" {
				return flow.RegisterValue("b_value"), nil
			}
			return nil, nil
		})

		vv := v.NewChild()
		_, err := vv.Get("a", "", "")
		assert.NoError(t, err)

		_, err = vv.Get("b", "", "")
		assert.NoError(t, err)

		err = vv.Set("c", "", "", flow.RegisterValue("c_value"))
		assert.NoError(t, err)
		err = vv.Set("d", "", "", flow.RegisterValue("d_value"))
		assert.NoError(t, err)

		err = vv.Touch("e", "", "")
		assert.NoError(t, err)
		err = vv.Touch("f", "", "")
		assert.NoError(t, err)

		err = v.MergeView(vv)
		assert.NoError(t, err)
		allRegs := v.Interactions().AllRegisters()
		assert.Len(t, allRegs, 6)
	})
}

func TestView_Reads(t *testing.T) {
	registerID1 := "fruit"
	registerID2 := "vegetable"

	v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		return nil, nil
	})

	t.Run("Empty", func(t *testing.T) {
		reads := v.Interactions().Reads
		assert.Empty(t, reads)
	})

	t.Run("Set and Get", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		_, err := v.Get(registerID2, "", "")
		assert.NoError(t, err)

		_, err = v.Get(registerID1, "", "")
		assert.NoError(t, err)

		_, err = v.Get(registerID2, "", "")
		assert.NoError(t, err)

		touches := v.Interactions().Reads
		require.Len(t, touches, 2)

		r1 := flow.NewRegisterID(registerID1, "", "")
		r2 := flow.NewRegisterID(registerID2, "", "")

		assert.Equal(t, map[string]flow.RegisterID{
			r1.String(): r1,
			r2.String(): r2,
		}, touches)
	})
}

func hashIt(spock hash.Hasher, value []byte) error {
	_, err := spock.Write(value)
	return err
}
