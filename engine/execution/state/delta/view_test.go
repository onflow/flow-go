package delta_test

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/state"
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
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			} else if owner == registerID {
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
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			} else if owner == registerID {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		v.Set(registerID, "", "", flow.RegisterValue("apple"))

		b, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b)
	})
}

func TestView_Set(t *testing.T) {
	registerID := "fruit"

	v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		if key == state.StorageUsedRegisterName {
			return uint64AsBytes(100), nil
		}
		return nil, nil
	})

	v.Set(registerID, "", "", flow.RegisterValue("apple"))

	b1, err := v.Get(registerID, "", "")
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("apple"), b1)

	v.Set(registerID, "", "", flow.RegisterValue("orange"))

	b2, err := v.Get(registerID, "", "")
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("orange"), b2)

	t.Run("AfterDelete", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		v.Set(registerID, "", "", flow.RegisterValue("apple"))
		v.Delete(registerID, "", "")
		v.Set(registerID, "", "", flow.RegisterValue("orange"))

		b, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("SpockSecret", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		t.Run("reflects in the snapshot", func(t *testing.T) {
			assert.Equal(t, v.SpockSecret(), v.Interactions().SpockSecret)
		})

		registerID1 := "reg1"

		registerID2 := "reg2"
		registerID3 := "reg3"

		// this part checks that spocks ordering be based
		// on update orders and not registerIDs
		expSpock := hash.NewSHA3_256()
		v.Set(registerID2, "", "", flow.RegisterValue("1"))
		err = hashIt(expSpock, []byte("1"))
		assert.NoError(t, err)
		err = hashIt(expSpock, uint64AsBytes(100)) // read state.StorageUsedRegisterName
		assert.NoError(t, err)
		err = hashIt(expSpock, uint64AsBytes(uint64(100+len([]byte("1"))))) // set state.StorageUsedRegisterName
		assert.NoError(t, err)
		s := v.SpockSecret()
		assert.Equal(t, s, []uint8(expSpock.SumHash()))

		v.Set(registerID3, "", "", flow.RegisterValue("2"))
		err = hashIt(expSpock, []byte("2"))
		assert.NoError(t, err)
		err = hashIt(expSpock, uint64AsBytes(100)) // read state.StorageUsedRegisterName
		assert.NoError(t, err)
		err = hashIt(expSpock, uint64AsBytes(uint64(100+len([]byte("2"))))) // set state.StorageUsedRegisterName
		assert.NoError(t, err)
		s = v.SpockSecret()
		assert.Equal(t, s, []uint8(expSpock.SumHash()))

		v.Set(registerID1, "", "", flow.RegisterValue("3"))
		err = hashIt(expSpock, []byte("3"))
		assert.NoError(t, err)
		err = hashIt(expSpock, uint64AsBytes(100)) // read state.StorageUsedRegisterName
		assert.NoError(t, err)
		err = hashIt(expSpock, uint64AsBytes(uint64(100+len([]byte("3"))))) // set state.StorageUsedRegisterName
		assert.NoError(t, err)
		s = v.SpockSecret()
		assert.Equal(t, s, []uint8(expSpock.SumHash()))

		b, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("3"))
		assert.NoError(t, err)
		s = v.SpockSecret()
		assert.Equal(t, s, []uint8(expSpock.SumHash()))

		assert.Equal(t, b, flow.RegisterValue("3"))
		// this part checks that delete functionality
		// doesn't impact secret
		v.Delete(registerID1, "", "")
		err = hashIt(expSpock, []byte("3"))
		assert.NoError(t, err)
		err = hashIt(expSpock, uint64AsBytes(100)) // set state.StorageUsedRegisterName
		assert.NoError(t, err)
		s = v.SpockSecret()
		assert.Equal(t, s, []uint8(expSpock.SumHash()))

		// this part checks that it always update the
		// intermediate values and not just the final values
		v.Set(registerID1, "", "", flow.RegisterValue("4"))
		err = hashIt(expSpock, []byte("4"))
		assert.NoError(t, err)
		err = hashIt(expSpock, uint64AsBytes(uint64(100+len([]byte("4"))))) // set state.StorageUsedRegisterName
		assert.NoError(t, err)
		s = v.SpockSecret()
		assert.Equal(t, s, []uint8(expSpock.SumHash()))

		v.Set(registerID1, "", "", flow.RegisterValue("5"))
		err = hashIt(expSpock, []byte("5"))
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("4"))
		assert.NoError(t, err)
		s = v.SpockSecret()
		assert.Equal(t, s, []uint8(expSpock.SumHash()))

		v.Set(registerID3, "", "", flow.RegisterValue("6"))
		err = hashIt(expSpock, []byte("6"))
		assert.NoError(t, err)
		err = hashIt(expSpock, []byte("2"))
		assert.NoError(t, err)

		s = v.SpockSecret()
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
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		b1, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Nil(t, b1)

		v.Delete(registerID, "", "")

		b2, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Nil(t, b2)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			if owner == registerID {
				return flow.RegisterValue("orange"), nil
			}

			return nil, nil
		})

		v.Set(registerID, "", "", flow.RegisterValue("apple"))

		b1, err := v.Get(registerID, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		v.Delete(registerID, "", "")

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
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		chView := v.NewChild()
		chView.Set(registerID1, "", "", flow.RegisterValue("apple"))
		chView.Set(registerID2, "", "", flow.RegisterValue("carrot"))

		v.MergeView(chView)

		b1, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("EmptyDelta", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		v.Set(registerID1, "", "", flow.RegisterValue("apple"))
		v.Set(registerID2, "", "", flow.RegisterValue("carrot"))

		chView := v.NewChild()
		v.MergeView(chView)

		b1, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("NoCollisions", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		v.Set(registerID1, "", "", flow.RegisterValue("apple"))

		chView := v.NewChild()
		chView.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		v.MergeView(chView)

		b1, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		v.Set(registerID1, "", "", flow.RegisterValue("apple"))

		chView := v.NewChild()
		chView.Set(registerID1, "", "", flow.RegisterValue("orange"))
		v.MergeView(chView)

		b, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteDeletedValue", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		v.Set(registerID1, "", "", flow.RegisterValue("apple"))
		v.Delete(registerID1, "", "")

		chView := v.NewChild()
		chView.Set(registerID1, "", "", flow.RegisterValue("orange"))
		v.MergeView(chView)

		b, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("DeleteSetValue", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		v.Set(registerID1, "", "", flow.RegisterValue("apple"))

		chView := v.NewChild()
		chView.Delete(registerID1, "", "")
		v.MergeView(chView)

		b, err := v.Get(registerID1, "", "")
		assert.NoError(t, err)
		assert.Nil(t, b)
	})
	t.Run("SpockDataMerge", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		expSpock1 := hash.NewSHA3_256()
		v.Set(registerID1, "", "", flow.RegisterValue("apple"))

		err := hashIt(expSpock1, []byte("apple"))
		assert.NoError(t, err)
		err = hashIt(expSpock1, uint64AsBytes(100)) // read state.StorageUsedRegisterName
		assert.NoError(t, err)
		err = hashIt(expSpock1, uint64AsBytes(uint64(100+len([]byte("apple"))))) // set state.StorageUsedRegisterName
		assert.NoError(t, err)
		assert.Equal(t, v.SpockSecret(), []uint8(expSpock1.SumHash()))

		expSpock2 := hash.NewSHA3_256()
		chView := v.NewChild()
		chView.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		err = hashIt(expSpock2, []byte("carrot"))
		assert.NoError(t, err)
		err = hashIt(expSpock1, uint64AsBytes(100)) // read state.StorageUsedRegisterName from parent
		assert.NoError(t, err)
		err = hashIt(expSpock2, uint64AsBytes(100)) // read state.StorageUsedRegisterName
		assert.NoError(t, err)
		err = hashIt(expSpock2, uint64AsBytes(uint64(100+len([]byte("carrot"))))) // set state.StorageUsedRegisterName
		assert.NoError(t, err)
		assert.Equal(t, chView.SpockSecret(), []uint8(expSpock2.SumHash()))
		assert.Equal(t, v.SpockSecret(), []uint8(expSpock1.SumHash()))

		v.MergeView(chView)
		err = hashIt(expSpock1, expSpock2.SumHash())
		assert.NoError(t, err)

		s := v.SpockSecret()
		assert.Equal(t, s, []uint8(expSpock1.SumHash()))
	})

	t.Run("RegisterTouchesDataMerge", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
			return nil, nil
		})

		v.Set(registerID1, "", "", flow.RegisterValue("apple"))

		chView := v.NewChild()
		chView.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		chView.Set(registerID3, "", "", flow.RegisterValue("milk"))

		v.MergeView(chView)

		reads := v.Interactions().Reads

		// 6 reads, because 3 storage used are read as well
		require.Len(t, reads, 6)
		assert.ElementsMatch(t, []flow.RegisterID{
			flow.NewRegisterID(registerID1, "", ""),
			flow.NewRegisterID(registerID2, "", ""),
			flow.NewRegisterID(registerID3, "", ""),
			flow.NewRegisterID(registerID1, "", state.StorageUsedRegisterName),
			flow.NewRegisterID(registerID2, "", state.StorageUsedRegisterName),
			flow.NewRegisterID(registerID3, "", state.StorageUsedRegisterName),
		}, reads)
	})

}

func TestView_RegisterTouches(t *testing.T) {
	registerID1 := "fruit"
	registerID2 := "vegetable"

	v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		if key == state.StorageUsedRegisterName {
			return uint64AsBytes(100), nil
		}
		return nil, nil
	})

	t.Run("Empty", func(t *testing.T) {
		touches := v.Interactions().RegisterTouches()
		assert.Empty(t, touches)
	})

	t.Run("Set and Get", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}

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

		v.Set(registerID2, "", "", flow.RegisterValue("apple"))

		touches := v.Interactions().RegisterTouches()
		// 3 touches, because 1 storage used in touched as well
		assert.Len(t, touches, 3)
	})
}

func TestView_AllRegisters(t *testing.T) {
	v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		if key == state.StorageUsedRegisterName {
			return uint64AsBytes(100), nil
		}
		return nil, nil
	})

	t.Run("Empty", func(t *testing.T) {
		regs := v.Interactions().AllRegisters()
		assert.Empty(t, regs)
	})

	t.Run("Set and Get", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}

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

		v.Set("c", "", "", flow.RegisterValue("c_value"))
		v.Set("d", "", "", flow.RegisterValue("d_value"))

		v.Touch("e", "", "")
		v.Touch("f", "", "")

		allRegs := v.Interactions().AllRegisters()
		// 8 touches, because 2 storage used are touched as well
		assert.Len(t, allRegs, 8)
	})
	t.Run("With Merge", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}

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

		vv.Set("c", "", "", flow.RegisterValue("c_value"))
		vv.Set("d", "", "", flow.RegisterValue("d_value"))

		vv.Touch("e", "", "")
		vv.Touch("f", "", "")

		v.MergeView(vv)
		allRegs := v.Interactions().AllRegisters()
		// 8 touches, because 2 storage used are touched as well
		assert.Len(t, allRegs, 8)
	})
}

func TestView_Reads(t *testing.T) {
	registerID1 := "fruit"
	registerID2 := "vegetable"

	v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		if key == state.StorageUsedRegisterName {
			return uint64AsBytes(100), nil
		}
		return nil, nil
	})

	t.Run("Empty", func(t *testing.T) {
		reads := v.Interactions().Reads
		assert.Empty(t, reads)
	})

	t.Run("Set and Get", func(t *testing.T) {
		v := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.StorageUsedRegisterName {
				return uint64AsBytes(100), nil
			}
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
		assert.ElementsMatch(t, []flow.RegisterID{
			flow.NewRegisterID(registerID1, "", ""),
			flow.NewRegisterID(registerID2, "", ""),
		}, touches)
	})
}

func hashIt(spock hash.Hasher, value []byte) error {
	_, err := spock.Write(value)
	return err
}

func uint64AsBytes(u uint64) []byte {
	buffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(buffer, u)
	return buffer
}
