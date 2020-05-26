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
	addr := ServiceAddress()
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
			addr:     ServiceAddress(),
			expected: ServiceAddress().Hex(),
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

func TestFlowAddressConstants(t *testing.T) {
	// check n and k fit in 8 and 6 bytes
	assert.LessOrEqual(t, linearCodeN, 8*8)
	assert.LessOrEqual(t, linearCodeK, 6*8)

	// Test addresses for all type of networks
	networks := []ChainID{
		Mainnet,
		Testnet,
		Emulator,
	}

	for _, net := range networks {

		// set the network
		setChainID(net)

		// check the Zero and Root constants
		expected := Uint64ToAddress(uint64(getNetworkType()))
		assert.Equal(t, ZeroAddress(), expected)
		expected = Uint64ToAddress(generatorMatrixRows[0] ^ uint64(getNetworkType()))
		assert.Equal(t, ServiceAddress(), expected)

		// check the transition from account zero to root
		state := NewAddressGenerator()
		address, err := state.AccountAddress()
		require.NoError(t, err)
		assert.Equal(t, address, ServiceAddress())

		// check high state values: generation should fail for high value states
		state = newAddressGeneratorAtState(maxState)
		_, err = state.AccountAddress()
		assert.NoError(t, err)
		_, err = state.AccountAddress()
		assert.Error(t, err)

		// check ZeroAddress() is an invalid addresse
		z := ZeroAddress()
		check := z.IsValid()
		assert.False(t, check, "should be invalid")
	}
}

const invalidCodeWord = uint64(0xab2ae42382900010)

func TestAddressGeneration(t *testing.T) {
	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// loops in each test
	const loop = 50

	// Test addresses for all type of networks
	networks := []ChainID{
		Mainnet,
		Testnet,
		Emulator,
	}

	for _, net := range networks {

		// set the network
		setChainID(net)

		// sanity check of AccountAddress function consistency
		state := NewAddressGenerator()
		expectedState := uint64(0)
		for i := 0; i < loop; i++ {
			address, err := state.AccountAddress()
			require.NoError(t, err)
			expectedState++
			expectedAddress := newAddressGeneratorAtState(expectedState).generateAddress()
			assert.Equal(t, address, expectedAddress)
		}

		// sanity check of addresses weights in Flow.
		// All addresses hamming weights must be less than d.
		// this is only a sanity check of the implementation and not an exhaustive proof
		if net == Mainnet {
			r := uint64(rand.Intn(maxState - loop))
			state = newAddressGeneratorAtState(r)
			for i := 0; i < loop; i++ {
				address, err := state.AccountAddress()
				require.NoError(t, err)
				weight := bits.OnesCount64(address.Uint64())
				assert.LessOrEqual(t, linearCodeD, weight)
			}
		}

		// sanity check of address distances.
		// All distances between any two addresses must be less than d.
		// this is only a sanity check of the implementation and not an exhaustive proof
		r := uint64(rand.Intn(maxState - loop - 1))
		state = newAddressGeneratorAtState(r)
		refAddress, err := state.AccountAddress()
		require.NoError(t, err)
		for i := 0; i < loop; i++ {
			address, err := state.AccountAddress()
			require.NoError(t, err)
			distance := bits.OnesCount64(address.Uint64() ^ refAddress.Uint64())
			assert.LessOrEqual(t, linearCodeD, distance)
		}

		// sanity check of valid account addresses.
		// All valid addresses must pass IsValid.
		r = uint64(rand.Intn(maxState - loop))
		state = newAddressGeneratorAtState(r)
		for i := 0; i < loop; i++ {
			address, err := state.AccountAddress()
			require.NoError(t, err)
			check := address.IsValid()
			assert.True(t, check, "account address format should be valid")
		}

		// sanity check of invalid account addresses.
		// All invalid addresses must fail IsValid.
		invalidAddress := Uint64ToAddress(invalidCodeWord)
		check := invalidAddress.IsValid()
		assert.False(t, check, "account address format should be invalid")
		r = uint64(rand.Intn(maxState - loop))
		state = newAddressGeneratorAtState(r)
		for i := 0; i < loop; i++ {
			address, err := state.AccountAddress()
			require.NoError(t, err)
			invalidAddress = Uint64ToAddress(address.Uint64() ^ invalidCodeWord)
			check := invalidAddress.IsValid()
			assert.False(t, check, "account address format should be invalid")
		}
	}
}

func TestAddressesIntersection(t *testing.T) {
	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// loops in each test
	const loop = 50

	// Test addresses for all type of networks
	networks := []ChainID{
		Testnet,
		Emulator,
	}

	for _, net := range networks {

		// set the network
		setChainID(net)

		// All valid test addresses must fail Flow Mainnet check
		r := uint64(rand.Intn(maxState - loop))
		state := newAddressGeneratorAtState(r)
		for i := 0; i < loop; i++ {
			address, err := state.AccountAddress()
			require.NoError(t, err)
			setChainID(Mainnet)
			check := address.IsValid()
			setChainID(net)
			assert.False(t, check, "test account address format should be invalid in Flow")
		}

		// sanity check: mainnet addresses must fail the test check
		r = uint64(rand.Intn(maxState - loop))
		state = newAddressGeneratorAtState(r)
		for i := 0; i < loop; i++ {
			setChainID(Mainnet)
			invalidAddress, err := state.AccountAddress()
			require.NoError(t, err)
			setChainID(net)
			check := invalidAddress.IsValid()
			assert.False(t, check, "account address format should be invalid")
		}

		// sanity check of invalid account addresses in all networks
		require.NotEqual(t, invalidCodeWord, uint64(0))
		invalidAddress := Uint64ToAddress(invalidCodeWord)
		setChainID(net)
		check := invalidAddress.IsValid()
		assert.False(t, check, "account address format should be invalid")
		r = uint64(rand.Intn(maxState - loop))
		state = newAddressGeneratorAtState(r)
		for i := 0; i < loop; i++ {
			setChainID(net)
			address, err := state.AccountAddress()
			require.NoError(t, err)
			invalidAddress = Uint64ToAddress(address.Uint64() ^ invalidCodeWord)
			// must fail test network check
			check = invalidAddress.IsValid()
			assert.False(t, check, "account address format should be invalid")
			// must fail mainnet check
			setChainID(Mainnet)
			check := invalidAddress.IsValid()
			assert.False(t, check, "account address format should be invalid")
		}
	}
}

func TestUint48(t *testing.T) {
	// seed random generator
	rand.Seed(time.Now().UnixNano())

	const loop = 50
	// test consistensy of putUint48 and uint48
	for i := 0; i < loop; i++ {
		r := uint64(rand.Intn(1 << linearCodeK))
		b := make([]byte, addressStateLength)
		putUint48(b, r)
		res := uint48(b)
		assert.Equal(t, r, res)
	}
}
