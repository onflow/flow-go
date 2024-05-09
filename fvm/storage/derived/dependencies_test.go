package derived_test

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/derived"
)

func TestProgramDependencies_Count(t *testing.T) {
	d := derived.NewProgramDependencies()

	require.Equal(t, 0, d.Count())

	d.Add(common.StringLocation("test"))
	require.Equal(t, 1, d.Count())
}

func TestProgramDependencies_Add(t *testing.T) {
	d := derived.NewProgramDependencies()

	d.Add(common.StringLocation("test"))
	require.Equal(t, 1, d.Count())

	address, _ := common.HexToAddress("0xa")
	addressLocation := common.AddressLocation{Address: address}
	d.Add(addressLocation)
	require.True(t, d.ContainsAddress(address))
}

func TestProgramDependencies_Merge(t *testing.T) {
	d1 := derived.NewProgramDependencies()
	d1.Add(common.StringLocation("test1"))

	d2 := derived.NewProgramDependencies()
	d2.Add(common.StringLocation("test2"))

	d1.Merge(d2)
	require.Equal(t, 2, d1.Count())
}

func TestProgramDependencies_ContainsAddress(t *testing.T) {
	d := derived.NewProgramDependencies()

	address, _ := common.HexToAddress("0xa")
	addressLocation := common.AddressLocation{Address: address}
	d.Add(addressLocation)

	require.True(t, d.ContainsAddress(address))
}

func TestProgramDependencies_ContainsLocation(t *testing.T) {
	d := derived.NewProgramDependencies()
	location := common.StringLocation("test")
	d.Add(location)

	require.True(t, d.ContainsLocation(location))
}
