package flow

import (
	"encoding/json"
	"math/bits"
	"math/rand"
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
			expected: RootAddress.Hex(),
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

// an invalid code word used in this test file
const invalidCodeWord = uint64(0xab2ae42382900010)

func TestFlowAdressConstants(t *testing.T) {
	// check n and k fit in 8 and 6 bytes
	assert.LessOrEqual(t, n, 64)
	assert.LessOrEqual(t, k, 48)

	//check the Zero and Root constants
	// Flow
	var expected [AddressLength]byte
	assert.Equal(t, ZeroAddress, Address(expected))
	expected = Uint64ToAddress(generatorMatrixRows[0])
	assert.Equal(t, RootAddress, Address(expected))
	// Flow test
	expected = Uint64ToAddress(constInvalidAddress)
	assert.Equal(t, ZeroTestAddress, Address(expected))
	expected = Uint64ToAddress(generatorMatrixRows[0] ^ constInvalidAddress)
	assert.Equal(t, RootTestAddress, Address(expected))

	// check the transition from account zero to root
	// Flow
	state := AddressState(0)
	address, _, err := AccountAddress(state)
	require.NoError(t, err)
	assert.Equal(t, address, RootAddress)
	// Flow test
	state = AddressState(0)
	address, _, err = TestAccountAddress(state)
	require.NoError(t, err)
	assert.Equal(t, address, RootTestAddress)

	// check high state values
	// Flow
	state = AddressState(maxState)
	_, state, err = AccountAddress(state)
	assert.NoError(t, err)
	_, _, err = AccountAddress(state)
	assert.Error(t, err)
	// Flow test
	state = AddressState(maxState)
	_, state, err = TestAccountAddress(state)
	assert.NoError(t, err)
	_, _, err = TestAccountAddress(state)
	assert.Error(t, err)

	// check ZeroAddress and ZeroTestAddress is invalid
	check := ZeroAddress.CheckAddress()
	assert.False(t, check, "should be invalid")
	check = ZeroTestAddress.CheckTestAddress()
	assert.False(t, check, "should be invalid")
}

func TestFlowAddressGeneration(t *testing.T) {
	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// loops in each test
	const loop = 50

	// sanity check of AccountAddress function consistency
	state := AddressState(0)
	expectedState := AddressState(0)
	for i := 0; i < loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		expectedState++
		expectedAddress := generateAddress(expectedState)
		assert.Equal(t, address, expectedAddress)
	}

	// sanity check of addresses weights in Flow
	// this is only a sanity check of the implementation and not an exhaustive proof
	r := rand.Intn(maxState - loop)
	state = AddressState(r)
	for i := 0; i < loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		weight := bits.OnesCount64(address.Uint64())
		assert.LessOrEqual(t, d, weight)
	}

	// sanity check of address distances
	// this is only a sanity check of the implementation and not an exhaustive proof
	r = rand.Intn(maxState - loop - 1)
	state = AddressState(r)
	refAddress, stateResult, err := AccountAddress(state)
	state = stateResult
	require.NoError(t, err)
	for i := 0; i < loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		distance := bits.OnesCount64(address.Uint64() ^ refAddress.Uint64())
		assert.LessOrEqual(t, d, distance)
	}

	// sanity check of valid account addresses
	r = rand.Intn(maxState - loop)
	state = AddressState(r)
	for i := 0; i < loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		check := address.CheckAddress()
		assert.True(t, check, "account address format should be valid")
	}

	// sanity check of invalid account addresses
	invalidAddress := Uint64ToAddress(invalidCodeWord)
	check := invalidAddress.CheckAddress()
	assert.False(t, check, "account address format should be invalid")
	r = rand.Intn(maxState - loop)
	state = AddressState(r)
	for i := 0; i < loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		invalidAddress = Uint64ToAddress(address.Uint64() ^ invalidCodeWord)
		check := invalidAddress.CheckAddress()
		assert.False(t, check, "account address format should be invalid")
	}
}

func TestFlowTestAddressGeneration(t *testing.T) {
	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// loops in each test
	const loop = 50

	// sanity check of TestAccountAddress function consistency
	state := AddressState(0)
	expectedState := AddressState(0)
	for i := 0; i < loop; i++ {
		testAddress, stateResult, err := TestAccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		expectedState++
		expectedTestAddress := generateTestAddress(expectedState)
		assert.Equal(t, testAddress, expectedTestAddress)
	}

	// sanity check of valid test account addresses
	r := rand.Intn(maxState - loop)
	state = AddressState(r)
	for i := 0; i < loop; i++ {
		address, stateResult, err := TestAccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		check := address.CheckTestAddress()
		assert.True(t, check, "test account address format should be valid")
		// check for Flow address must fail
		check = address.CheckAddress()
		assert.False(t, check, "test account address format should be invalid in Flow")
	}

	// sanity check: Flow addresses should fail the test check
	r = rand.Intn(maxState - loop)
	state = AddressState(r)
	for i := 0; i < loop; i++ {
		invalidAddress, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		check := invalidAddress.CheckTestAddress()
		assert.False(t, check, "account address format should be invalid")
	}

	// sanity check: some addresses that fail c
	r = rand.Intn(maxState - loop)
	state = AddressState(r)
	for i := 0; i < loop; i++ {
		invalidAddress, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		check := invalidAddress.CheckTestAddress()
		assert.False(t, check, "account address format should be invalid")
	}

	// sanity check of invalid account addresses in both Flow and Flow test
	invalidUint64 := uint64(invalidCodeWord ^ constInvalidAddress)
	require.NotEqual(t, invalidUint64, uint64(0))
	invalidAddress := Uint64ToAddress(invalidUint64)
	check := invalidAddress.CheckAddress()
	assert.False(t, check, "account address format should be invalid")
	r = rand.Intn(maxState - loop)
	state = AddressState(r)
	for i := 0; i < loop; i++ {
		address, stateResult, err := AccountAddress(state)
		state = stateResult
		require.NoError(t, err)
		invalidAddress = Uint64ToAddress(address.Uint64() ^ invalidUint64)
		// should fail Flow check
		check := invalidAddress.CheckAddress()
		assert.False(t, check, "account address format should be invalid")
		// should fail Flow test check
		check = invalidAddress.CheckTestAddress()
		assert.False(t, check, "account address format should be invalid")
	}
}

func TestUint48(t *testing.T) {
	// seed random generator
	rand.Seed(time.Now().UnixNano())
	
	const loop = 50
	// test consistensy of putUint48 and uint48
	for i:=0; i<loop; i++ {
		r := uint64(0) //uint64(rand.Intn(1<<k))
		b := make([]byte, AddressStateLength)
		putUint48(b, r)
		res := uint48(b)
		assert.Equal(t, r, res)
	}
}
