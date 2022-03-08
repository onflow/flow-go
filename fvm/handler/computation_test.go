package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputationMeteringHandler(t *testing.T) {
	const limit = uint64(100)
	const used = uint(7)

	t.Run("Get Limit", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit,
			WithCoumputationWeightFactors(map[uint]uint{
				0: 1,
			}))

		l := h.Limit()

		require.Equal(t, limit, l)
	})

	t.Run("Set/Get Used", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit,
			WithCoumputationWeightFactors(map[uint]uint{
				0: 1,
			}))

		err := h.AddUsed(0, used)
		require.NoError(t, err)

		u := h.Used()

		require.Equal(t, used, u)
	})

	t.Run("Set/Get Used twice", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit,
			WithCoumputationWeightFactors(map[uint]uint{
				0: 1,
			}))

		err := h.AddUsed(0, used)
		require.NoError(t, err)

		err = h.AddUsed(0, used)
		require.NoError(t, err)

		u := h.Used()

		require.Equal(t, 2*used, u)
	})

	t.Run("Add used adds to intensity", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit,
			WithCoumputationWeightFactors(map[uint]uint{
				0: 1,
			}))

		err := h.AddUsed(0, used)
		require.NoError(t, err)

		err = h.AddUsed(0, used)
		require.NoError(t, err)

		i := h.Intensities()

		require.Equal(t, 2*used, i[0])
	})

	t.Run("Add used adds to intensity to sub-meter but not on meter", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit,
			WithCoumputationWeightFactors(map[uint]uint{
				0: 1,
			}))

		subMeter := h.StartSubMeter(2 * limit)

		err := h.AddUsed(0, used)
		require.NoError(t, err)

		err = subMeter.Discard()
		require.NoError(t, err)

		err = h.AddUsed(0, used)
		require.NoError(t, err)

		i := h.Intensities()

		require.Equal(t, used, i[0])
	})

	t.Run("Sub Meter", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit,
			WithCoumputationWeightFactors(map[uint]uint{
				0: 1,
			}))

		subMeter := h.StartSubMeter(2 * limit)

		l := h.Limit()
		require.Equal(t, 2*limit, l)

		err := h.AddUsed(0, used)
		require.NoError(t, err)

		u := h.Used()
		require.Equal(t, used, u)

		err = subMeter.Discard()
		require.NoError(t, err)

		l = h.Limit()
		require.Equal(t, limit, l)

		u = h.Used()
		require.Equal(t, uint64(0), u)
	})

	t.Run("Sub Sub Meter", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit,
			WithCoumputationWeightFactors(map[uint]uint{
				0: 1,
			}))

		subMeter := h.StartSubMeter(2 * limit)

		err := h.AddUsed(0, used)
		require.NoError(t, err)

		subSubMeter := h.StartSubMeter(3 * limit)

		l := h.Limit()
		require.Equal(t, 3*limit, l)

		err = h.AddUsed(0, 2*used)
		require.NoError(t, err)

		u := h.Used()
		require.Equal(t, 2*used, u)

		err = subSubMeter.Discard()
		require.NoError(t, err)

		l = h.Limit()
		require.Equal(t, 2*limit, l)

		u = h.Used()
		require.Equal(t, used, u)

		err = subMeter.Discard()
		require.NoError(t, err)

		l = h.Limit()
		require.Equal(t, limit, l)

		u = h.Used()
		require.Equal(t, uint64(0), u)
	})

	t.Run("Sub Sub Meter - discard in wrong order", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit,
			WithCoumputationWeightFactors(map[uint]uint{
				0: 1,
			}))
		subMeter := h.StartSubMeter(2 * limit)
		_ = h.StartSubMeter(3 * limit)

		err := subMeter.Discard()
		require.Error(t, err)
	})
}
