package delta_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
)

type testStorage map[flow.RegisterID]string

func (storage testStorage) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	return flow.RegisterValue(storage[id]), nil
}

func TestViewGet(t *testing.T) {
	registerID := flow.NewRegisterID("fruit", "")

	t.Run("ValueNotSet", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		b, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Nil(t, b)
	})

	t.Run("ValueNotInCache", func(t *testing.T) {
		v := delta.NewDeltaView(
			testStorage{
				registerID: "orange",
			})
		b, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("ValueInCache", func(t *testing.T) {
		v := delta.NewDeltaView(
			testStorage{
				registerID: "orange",
			})
		err := v.Set(registerID, flow.RegisterValue("apple"))
		assert.NoError(t, err)

		b, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b)
	})
}

func TestViewSet(t *testing.T) {
	registerID := flow.NewRegisterID("fruit", "")

	v := delta.NewDeltaView(nil)

	err := v.Set(registerID, flow.RegisterValue("apple"))
	assert.NoError(t, err)

	b1, err := v.Get(registerID)
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("apple"), b1)

	err = v.Set(registerID, flow.RegisterValue("orange"))
	assert.NoError(t, err)

	b2, err := v.Get(registerID)
	assert.NoError(t, err)
	assert.Equal(t, flow.RegisterValue("orange"), b2)

	t.Run("Overwrite register", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		err := v.Set(registerID, flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = v.Set(registerID, flow.RegisterValue("orange"))
		assert.NoError(t, err)

		b, err := v.Get(registerID)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("SpockSecret", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		t.Run("reflects in the snapshot", func(t *testing.T) {
			assert.Equal(t, v.SpockSecret(), v.Interactions().SpockSecret)
		})

		v = delta.NewDeltaView(nil)

		registerID1 := flow.NewRegisterID("reg1", "")
		registerID2 := flow.NewRegisterID("reg2", "")
		registerID3 := flow.NewRegisterID("reg3", "")

		// prepare the registerID bytes
		registerID1Bytes := registerID1.Bytes()
		registerID2Bytes := registerID2.Bytes()
		registerID3Bytes := registerID3.Bytes()

		// this part checks that spocks ordering be based
		// on update orders and not registerIDs
		expSpock := hash.NewSHA3_256()
		err = v.Set(registerID2, flow.RegisterValue("1"))
		require.NoError(t, err)
		hashIt(t, expSpock, registerID2Bytes)
		hashIt(t, expSpock, []byte("1"))

		err = v.Set(registerID3, flow.RegisterValue("2"))
		require.NoError(t, err)
		hashIt(t, expSpock, registerID3Bytes)
		hashIt(t, expSpock, []byte("2"))

		err = v.Set(registerID1, flow.RegisterValue("3"))
		require.NoError(t, err)
		hashIt(t, expSpock, registerID1Bytes)
		hashIt(t, expSpock, []byte("3"))

		_, err := v.Get(registerID1)
		require.NoError(t, err)
		hashIt(t, expSpock, registerID1Bytes)

		// this part checks that it always update the
		// intermediate values and not just the final values
		err = v.Set(registerID1, flow.RegisterValue("4"))
		require.NoError(t, err)
		hashIt(t, expSpock, registerID1Bytes)
		hashIt(t, expSpock, []byte("4"))

		err = v.Set(registerID1, flow.RegisterValue("5"))
		require.NoError(t, err)
		hashIt(t, expSpock, registerID1Bytes)
		hashIt(t, expSpock, []byte("5"))

		err = v.Set(registerID3, flow.RegisterValue("6"))
		require.NoError(t, err)
		hashIt(t, expSpock, registerID3Bytes)
		hashIt(t, expSpock, []byte("6"))

		s := v.SpockSecret()
		assert.Equal(t, hash.Hash(s), expSpock.SumHash())

		t.Run("reflects in the snapshot", func(t *testing.T) {
			assert.Equal(t, v.SpockSecret(), v.Interactions().SpockSecret)
		})
	})
}

func TestViewMerge(t *testing.T) {
	registerID1 := flow.NewRegisterID("fruit", "")
	registerID2 := flow.NewRegisterID("vegetable", "")
	registerID3 := flow.NewRegisterID("diary", "")

	t.Run("EmptyView", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		chView := v.NewChild()
		err := chView.Set(registerID1, flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = chView.Set(registerID2, flow.RegisterValue("carrot"))
		assert.NoError(t, err)

		err = v.Merge(chView.Finalize())
		assert.NoError(t, err)

		b1, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("EmptyDelta", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		err := v.Set(registerID1, flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = v.Set(registerID2, flow.RegisterValue("carrot"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = v.Merge(chView.Finalize())
		assert.NoError(t, err)

		b1, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("NoCollisions", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		err := v.Set(registerID1, flow.RegisterValue("apple"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = chView.Set(registerID2, flow.RegisterValue("carrot"))
		assert.NoError(t, err)

		err = v.Merge(chView.Finalize())
		assert.NoError(t, err)

		b1, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("apple"), b1)

		b2, err := v.Get(registerID2)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	})

	t.Run("OverwriteSetValue", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		err := v.Set(registerID1, flow.RegisterValue("apple"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = chView.Set(registerID1, flow.RegisterValue("orange"))
		assert.NoError(t, err)
		err = v.Merge(chView.Finalize())
		assert.NoError(t, err)

		b, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("OverwriteValue", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		err := v.Set(registerID1, flow.RegisterValue("apple"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = chView.Set(registerID1, flow.RegisterValue("orange"))
		assert.NoError(t, err)
		err = v.Merge(chView.Finalize())
		assert.NoError(t, err)

		b, err := v.Get(registerID1)
		assert.NoError(t, err)
		assert.Equal(t, flow.RegisterValue("orange"), b)
	})

	t.Run("SpockDataMerge", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		registerID1Bytes := registerID1.Bytes()
		registerID2Bytes := registerID2.Bytes()

		expSpock1 := hash.NewSHA3_256()
		err := v.Set(registerID1, flow.RegisterValue("apple"))
		assert.NoError(t, err)
		hashIt(t, expSpock1, registerID1Bytes)
		hashIt(t, expSpock1, []byte("apple"))
		assert.NoError(t, err)

		expSpock2 := hash.NewSHA3_256()
		chView := v.NewChild()
		err = chView.Set(registerID2, flow.RegisterValue("carrot"))
		require.NoError(t, err)
		hashIt(t, expSpock2, registerID2Bytes)
		hashIt(t, expSpock2, []byte("carrot"))

		hash2 := expSpock2.SumHash()
		assert.Equal(t, chView.(*delta.View).SpockSecret(), []uint8(hash2))

		err = v.Merge(chView.Finalize())
		assert.NoError(t, err)

		hashIt(t, expSpock1, hash2)
		assert.Equal(t, v.SpockSecret(), []uint8(expSpock1.SumHash()))
	})

	t.Run("RegisterTouchesDataMerge", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		err := v.Set(registerID1, flow.RegisterValue("apple"))
		assert.NoError(t, err)

		chView := v.NewChild()
		err = chView.Set(registerID2, flow.RegisterValue("carrot"))
		assert.NoError(t, err)
		err = chView.Set(registerID3, flow.RegisterValue("milk"))
		assert.NoError(t, err)

		err = v.Merge(chView.Finalize())
		assert.NoError(t, err)

		reads := v.Interactions().Reads

		require.Len(t, reads, 3)

		assert.Equal(t, map[flow.RegisterID]struct{}{
			registerID1: struct{}{},
			registerID2: struct{}{},
			registerID3: struct{}{},
		}, reads)
	})

}

func TestView_RegisterTouches(t *testing.T) {
	registerID1 := flow.NewRegisterID("fruit", "")
	registerID2 := flow.NewRegisterID("vegetable", "")

	v := delta.NewDeltaView(nil)

	t.Run("Empty", func(t *testing.T) {
		touches := v.Interactions().RegisterTouches()
		assert.Empty(t, touches)
	})

	t.Run("Set and Get", func(t *testing.T) {
		v := delta.NewDeltaView(
			testStorage{
				registerID1: "orange",
				registerID2: "carrot",
			})
		_, err := v.Get(registerID1)
		assert.NoError(t, err)

		err = v.Set(registerID2, flow.RegisterValue("apple"))
		assert.NoError(t, err)

		touches := v.Interactions().RegisterTouches()
		assert.Len(t, touches, 2)
	})
}

func TestView_AllRegisterIDs(t *testing.T) {
	idA := flow.NewRegisterID("a", "")
	idB := flow.NewRegisterID("b", "")
	idC := flow.NewRegisterID("c", "")
	idD := flow.NewRegisterID("d", "")
	idE := flow.NewRegisterID("e", "")
	idF := flow.NewRegisterID("f", "")

	v := delta.NewDeltaView(nil)

	t.Run("Empty", func(t *testing.T) {
		regs := v.Interactions().AllRegisterIDs()
		assert.Empty(t, regs)
	})

	t.Run("Set and Get", func(t *testing.T) {
		v := delta.NewDeltaView(
			testStorage{
				idA: "a_value",
				idB: "b_value",
			})

		_, err := v.Get(idA)
		assert.NoError(t, err)

		_, err = v.Get(idB)
		assert.NoError(t, err)

		err = v.Set(idC, flow.RegisterValue("c_value"))
		assert.NoError(t, err)

		err = v.Set(idD, flow.RegisterValue("d_value"))
		assert.NoError(t, err)

		err = v.Set(idE, flow.RegisterValue("e_value"))
		assert.NoError(t, err)
		err = v.Set(idF, flow.RegisterValue("f_value"))
		assert.NoError(t, err)

		allRegs := v.Interactions().AllRegisterIDs()
		assert.Len(t, allRegs, 6)
	})
	t.Run("With Merge", func(t *testing.T) {
		v := delta.NewDeltaView(
			testStorage{
				idA: "a_value",
				idB: "b_value",
			})

		vv := v.NewChild()
		_, err := vv.Get(idA)
		assert.NoError(t, err)

		_, err = vv.Get(idB)
		assert.NoError(t, err)

		err = vv.Set(idC, flow.RegisterValue("c_value"))
		assert.NoError(t, err)
		err = vv.Set(idD, flow.RegisterValue("d_value"))
		assert.NoError(t, err)

		err = vv.Set(idE, flow.RegisterValue("e_value"))
		assert.NoError(t, err)
		err = vv.Set(idF, flow.RegisterValue("f_value"))
		assert.NoError(t, err)

		err = v.Merge(vv.Finalize())
		assert.NoError(t, err)
		allRegs := v.Interactions().AllRegisterIDs()
		assert.Len(t, allRegs, 6)
	})
}

func TestView_Reads(t *testing.T) {
	registerID1 := flow.NewRegisterID("fruit", "")
	registerID2 := flow.NewRegisterID("vegetable", "")

	v := delta.NewDeltaView(nil)

	t.Run("Empty", func(t *testing.T) {
		reads := v.Interactions().Reads
		assert.Empty(t, reads)
	})

	t.Run("Set and Get", func(t *testing.T) {
		v := delta.NewDeltaView(nil)

		_, err := v.Get(registerID2)
		assert.NoError(t, err)

		_, err = v.Get(registerID1)
		assert.NoError(t, err)

		_, err = v.Get(registerID2)
		assert.NoError(t, err)

		touches := v.Interactions().Reads
		require.Len(t, touches, 2)

		assert.Equal(t, map[flow.RegisterID]struct{}{
			registerID1: struct{}{},
			registerID2: struct{}{},
		}, touches)
	})
}

func hashIt(t *testing.T, spock hash.Hasher, value []byte) {
	_, err := spock.Write(value)
	assert.NoError(t, err, "spock write is not supposed to error")
}
