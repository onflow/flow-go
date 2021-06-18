package binstat

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRng(t *testing.T) {
	os.Setenv("BINSTAT_ENABLE", "1")
	os.Setenv("BINSTAT_VERBOSE", "1")
	os.Setenv("BINSTAT_LEN_WHAT", "~f=99;~eg=99")

	p1 := New("~2egNew", "")
	EndVal(p1, 123)

	p2 := NewTimeVal("~2egNewTimeVal", "", 123)
	Pnt(p2, "myPnt")
	EndVal(p2, 123)

	p3 := newTimeValInternal("~2egnewTimeValInternal", 123)
	pntInternal(p3, "myPnt")
	endInternal(p3)

	var isIntIsFalse bool = false

	require.Equal(t, "100.000000-199.999999", rng(123.456789, isIntIsFalse))
	require.Equal(t, "10.000000-19.999999", rng(12.345678, isIntIsFalse))
	require.Equal(t, "1.000000-1.999999", rng(1.234567, isIntIsFalse))
	require.Equal(t, "0.100000-0.199999", rng(0.123456, isIntIsFalse))
	require.Equal(t, "0.010000-0.019999", rng(0.012345, isIntIsFalse))
	require.Equal(t, "0.001000-0.001999", rng(0.001234, isIntIsFalse))
	require.Equal(t, "0.000100-0.000199", rng(0.000123, isIntIsFalse))
	require.Equal(t, "0.000010-0.000019", rng(0.000012, isIntIsFalse))
	require.Equal(t, "0.000001-0.000001", rng(0.000001, isIntIsFalse))
	require.Equal(t, "0.000000-0.000000", rng(0.000000, isIntIsFalse))

	require.Equal(t, "200.000000-299.999999", rng(234.567890, isIntIsFalse))
	require.Equal(t, "20.000000-29.999999", rng(23.456789, isIntIsFalse))
	require.Equal(t, "2.000000-2.999999", rng(2.345678, isIntIsFalse))
	require.Equal(t, "0.200000-0.299999", rng(0.234567, isIntIsFalse))
	require.Equal(t, "0.020000-0.029999", rng(0.023456, isIntIsFalse))
	require.Equal(t, "0.002000-0.002999", rng(0.002345, isIntIsFalse))
	require.Equal(t, "0.000200-0.000299", rng(0.000234, isIntIsFalse))
	require.Equal(t, "0.000020-0.000029", rng(0.000023, isIntIsFalse))
	require.Equal(t, "0.000002-0.000002", rng(0.000002, isIntIsFalse))

	var isIntIsTrue bool = true

	require.Equal(t, "100-199", rng(123.456789, isIntIsTrue))
	require.Equal(t, "10-19", rng(12.345678, isIntIsTrue))
	require.Equal(t, "1", rng(1.234567, isIntIsTrue))
	require.Equal(t, "0", rng(0.123456, isIntIsTrue))
	require.Equal(t, "0", rng(0.012345, isIntIsTrue))
	require.Equal(t, "0", rng(0.001234, isIntIsTrue))
	require.Equal(t, "0", rng(0.000123, isIntIsTrue))
	require.Equal(t, "0", rng(0.000012, isIntIsTrue))
	require.Equal(t, "0", rng(0.000001, isIntIsTrue))
	require.Equal(t, "0", rng(0.000000, isIntIsTrue))

	require.Equal(t, "200-299", rng(234.567890, isIntIsTrue))
	require.Equal(t, "20-29", rng(23.456789, isIntIsTrue))
	require.Equal(t, "2", rng(2.345678, isIntIsTrue))
	require.Equal(t, "0", rng(0.234567, isIntIsTrue))
	require.Equal(t, "0", rng(0.023456, isIntIsTrue))
	require.Equal(t, "0", rng(0.002345, isIntIsTrue))
	require.Equal(t, "0", rng(0.000234, isIntIsTrue))
	require.Equal(t, "0", rng(0.000023, isIntIsTrue))
	require.Equal(t, "0", rng(0.000002, isIntIsTrue))

	// todo: add more tests? which tests?
}
