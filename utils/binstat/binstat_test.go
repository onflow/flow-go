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
	os.Setenv("BINSTAT_LEN_WHAT", "~f=99;~eg=99")
}

func TestBinstatInternal(t *testing.T) {
	global.enable = false
	tick(100 * time.Millisecond)
	enterGeneric("", false, 0, 0, false)
	pointGeneric(nil, "", 0, 0, false)
	debugGeneric(nil, "", false)
	bs0 := Enter("~2egEnter").DebugParams("foo")
	bs0.Run(func() {
	})
	bs0.LeaveVal(123)
	global.enable = true

	bs1 := Enter("~2egEnter").DebugParams("foo")
	bs1.Run(func() {
	})
	bs1.LeaveVal(123)

	bs2 := EnterTimeVal("~2egEnterTimeVal", 123)
	bs2.Run(func() {
	})
	bs2.Point("myPoint")
	bs2.Run(func() {
	})
	bs2.LeaveVal(123)

	bs3 := enterTimeValInternal("~2egenterTimeValInternal", 123)
	bs3.Run(func() {
	})
	bs3.pointInternal("myPoint")
	bs3.Run(func() {
	})
	bs3.leaveInternal()

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
