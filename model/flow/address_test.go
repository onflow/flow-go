package flow

import (
	"encoding/json"
	"math/rand"
	"math/bits"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type addressWrapper struct {
	Address Address
}

func TestAddressJSON(t *testing.T) {
	addr := RootAddress
	data, err := json.Marshal(addressWrapper{Address: addr})
	require.Nil(t, err)

	t.Log(string(data))

	var out addressWrapper
	err = json.Unmarshal(data, &out)
	require.Nil(t, err)
	assert.Equal(t, addr, out.Address)
}

func TestShort(t *testing.T) {
	type testcase struct {
		addr     Address
		expected string
	}

	cases := []testcase{
		{
			addr:     RootAddress,
			expected: "e467b9dd11fa00df",
		},
		{
			addr:     HexToAddress("0000000002"),
			expected: "02",
		},
		{
			addr:     HexToAddress("1f10"),
			expected: "1f10",
		},
		{
			addr:     HexToAddress("0f10"),
			expected: "0f10",
		},
	}

	for _, c := range cases {
		assert.Equal(t, c.addr.Short(), c.expected)
	}
}

func TestConstants(t *testing.T) {

	//check the Zero and Root constants
	var expected [AddressLength]byte
	assert.Equal(t, ZeroAddress, Address(expected))
	expected = Uint64ToAddress(generatorMatrixRows[0])
	assert.Equal(t, RootAddress, Address(expected))

	// check the transition from account zero to root
	state := AddressState(0)
	address, _, err := AccountAddress(state)
	require.NoError(t, err)
	assert.Equal(t, address, RootAddress)

	// check high state values
	state = AddressState(maxState)
	_, state, err = AccountAddress(state)
	assert.NoError(t, err)
	_, _, err = AccountAddress(state)
	assert.Error(t, err)
}

func TestAddressGeneration(t *testing.T) {
	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// loops in each test
	const loop = 50

	// sanity check AccountAddress function consistency
	state := AddressState(0)
	expectedState := AddressState(0)
	for i:=0; i<loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		expectedState++
		expectedAddress := generateAddress(expectedState)
		assert.Equal(t, address, expectedAddress)
	}

	// sanity check of addresses weights
	// this is only a sanity check of the implementation and not an exhaustive proof
	r := rand.Intn(maxState-loop)
	state = AddressState(r)
	for i:=0; i<loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		weight := bits.OnesCount64(address.Uint64())
		assert.LessOrEqual(t, 7, weight)
	}

	// sanity check of address distances
	// this is only a sanity check of the implementation and not an exhaustive proof
	r = rand.Intn(maxState-loop-1)
	state = AddressState(r)
	refAddress, stateResult, err := AccountAddress(state)
	state = stateResult
	require.NoError(t, err)
	for i:=0; i<loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		distance := bits.OnesCount64(address.Uint64() ^ refAddress.Uint64())
		assert.LessOrEqual(t, 7, distance)
	}

	// sanity check of valid account addresses 
	r = rand.Intn(maxState-loop)
	state = AddressState(r)
	for i:=0; i<loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		check := address.CheckAddress()
		assert.True(t, check, "account address format should be valid")
	}

	// sanity check of invalid account addresses 
	invalidUint64 := uint64(0xab2ae42382900010)
	invalidAddress := Uint64ToAddress(invalidUint64)
	check := invalidAddress.CheckAddress()
	assert.False(t, check, "account address format should be invalid")
	r = rand.Intn(maxState-loop)
	state = AddressState(r)
	for i:=0; i<loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		invalidAddress = Uint64ToAddress(address.Uint64() ^ invalidUint64)
		check := invalidAddress.CheckAddress()
		assert.False(t, check, "account address format should be invalid")
	}

	
}


