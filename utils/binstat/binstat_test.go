package binstat

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func init() {
	os.Setenv("BINSTAT_ENABLE", "1")
	os.Setenv("BINSTAT_VERBOSE", "1")
	os.Setenv("BINSTAT_LEN_WHAT", "~f=99;~eg=99;~example=99")

	global.enable = false
	tick(100 * time.Millisecond)
	enterGeneric("", false, 0, 0, false)
	pointGeneric(nil, "", 0, 0, false)
	debugGeneric(nil, "", false)
	bs0 := Enter("~2egEnter")
	DebugParams(bs0, "foo")
	LeaveVal(bs0, 123)
	enterVal("~2egEnterVal", 123)
}

func TestBinstatInternal(t *testing.T) {
	bs1 := Enter("~2egEnter")
	DebugParams(bs1, "foo")
	LeaveVal(bs1, 123)
	bs1.GetSizeRange()
	bs1.GetTimeRange()
	bs1.GetWhat()

	bs2 := EnterTimeVal("~2egEnterTimeVal", 123)
	Point(bs2, "myPoint")
	LeaveVal(bs2, 123)

	bs3 := enterTimeVal("~2egenterTimeValInternal", 123)
	point(bs3, "myPoint")
	leave(bs3)

	var isIntIsFalse bool = false

	require.Equal(t, "100.000000-199.999999", x_2_y(123.456789, isIntIsFalse))
	require.Equal(t, "10.000000-19.999999", x_2_y(12.345678, isIntIsFalse))
	require.Equal(t, "1.000000-1.999999", x_2_y(1.2345678, isIntIsFalse))
	require.Equal(t, "1.000000-1.999999", x_2_y(1.2345671, isIntIsFalse))
	require.Equal(t, "1.000000-1.999999", x_2_y(1.234567, isIntIsFalse))
	require.Equal(t, "0.100000-0.199999", x_2_y(0.123456, isIntIsFalse))
	require.Equal(t, "0.010000-0.019999", x_2_y(0.012345, isIntIsFalse))
	require.Equal(t, "0.001000-0.001999", x_2_y(0.001234, isIntIsFalse))
	require.Equal(t, "0.000100-0.000199", x_2_y(0.000123, isIntIsFalse))
	require.Equal(t, "0.000010-0.000019", x_2_y(0.000012, isIntIsFalse))
	require.Equal(t, "0.000001-0.000001", x_2_y(0.000001, isIntIsFalse))
	require.Equal(t, "0.000000-0.000000", x_2_y(0.000000, isIntIsFalse))

	require.Equal(t, "200.000000-299.999999", x_2_y(234.567890, isIntIsFalse))
	require.Equal(t, "20.000000-29.999999", x_2_y(23.456789, isIntIsFalse))
	require.Equal(t, "2.000000-2.999999", x_2_y(2.345678, isIntIsFalse))
	require.Equal(t, "0.200000-0.299999", x_2_y(0.234567, isIntIsFalse))
	require.Equal(t, "0.020000-0.029999", x_2_y(0.023456, isIntIsFalse))
	require.Equal(t, "0.002000-0.002999", x_2_y(0.002345, isIntIsFalse))
	require.Equal(t, "0.000200-0.000299", x_2_y(0.000234, isIntIsFalse))
	require.Equal(t, "0.000020-0.000029", x_2_y(0.000023, isIntIsFalse))
	require.Equal(t, "0.000002-0.000002", x_2_y(0.000002, isIntIsFalse))

	var isIntIsTrue bool = true

	require.Equal(t, "100-199", x_2_y(123.456789, isIntIsTrue))
	require.Equal(t, "10-19", x_2_y(12.345678, isIntIsTrue))
	require.Equal(t, "1-1", x_2_y(1.234567, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.123456, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.012345, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.001234, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.000123, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.000012, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.000001, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.000000, isIntIsTrue))

	require.Equal(t, "200-299", x_2_y(234.567890, isIntIsTrue))
	require.Equal(t, "20-29", x_2_y(23.456789, isIntIsTrue))
	require.Equal(t, "2-2", x_2_y(2.345678, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.234567, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.023456, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.002345, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.000234, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.000023, isIntIsTrue))
	require.Equal(t, "0-0", x_2_y(0.000002, isIntIsTrue))

	// todo: add more tests? which tests?
}
