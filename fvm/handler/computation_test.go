package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputationMeteringHandler(t *testing.T) {
	const limit = uint64(100)
	const used = uint64(7)

	t.Run("Get Limit", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit)

		l := h.Limit()

		require.Equal(t, limit, l)
	})

	t.Run("Set/Get Used", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit)

		h.AddUsed(used, "function_or_loop_call")

		u := h.Used()

		require.Equal(t, used, u)
	})

	t.Run("Set/Get Used twice", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit)

		h.AddUsed(used, "function_or_loop_call")

		h.AddUsed(used, "function_or_loop_call")

		u := h.Used()

		require.Equal(t, 2*used, u)
	})

	t.Run("Sub Meter", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit)

		subMeter := h.StartSubMeter(2 * limit)

		l := h.Limit()
		require.Equal(t, 2*limit, l)

		h.AddUsed(used, "function_or_loop_call")

		u := h.Used()
		require.Equal(t, used, u)

		err := subMeter.Discard()
		require.NoError(t, err)

		l = h.Limit()
		require.Equal(t, limit, l)

		u = h.Used()
		require.Equal(t, uint64(0), u)
	})

	t.Run("Sub Sub Meter", func(t *testing.T) {
		h := NewComputationMeteringHandler(limit)

		subMeter := h.StartSubMeter(2 * limit)

		h.AddUsed(used, "function_or_loop_call")

		subSubMeter := h.StartSubMeter(3 * limit)

		l := h.Limit()
		require.Equal(t, 3*limit, l)

		h.AddUsed(2*used, "function_or_loop_call")

		u := h.Used()
		require.Equal(t, 2*used, u)

		err := subSubMeter.Discard()
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
		h := NewComputationMeteringHandler(limit)
		subMeter := h.StartSubMeter(2 * limit)
		_ = h.StartSubMeter(3 * limit)

		err := subMeter.Discard()
		require.Error(t, err)
	})
}
