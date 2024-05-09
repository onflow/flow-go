package flow

import (
	"encoding/json"
	"math/bits"
	"math/rand"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type addressWrapper struct {
	Address Address
}

func TestConvertAddress(t *testing.T) {
	expected := BytesToAddress([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	cadenceAddress := cadence.BytesToAddress([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	runtimeAddress := common.MustBytesToAddress([]byte{1, 2, 3, 4, 5, 6, 7, 8})

	assert.NotEqual(t, cadenceAddress, runtimeAddress)

	assert.Equal(t, expected, ConvertAddress(cadenceAddress))
	assert.Equal(t, expected, ConvertAddress(runtimeAddress))
}

func TestBytesToAddress(t *testing.T) {
	type testCase struct {
		expected string
		value    string
	}

	for _, test := range []testCase{
		{string([]byte{0, 0, 0, 0, 0, 0, 0, 0}), ""},
		{string([]byte{0, 0, 0, 0}) + "1234", "1234"},
		{"12345678", "12345678"},
		{"12345678", "trim12345678"},
	} {
		address := BytesToAddress([]byte(test.value))
		assert.Equal(t, test.expected, string(address[:]))
	}
}

func TestHexToAddress(t *testing.T) {

	type testCase struct {
		literal string
		value   []byte
	}

	for _, test := range []testCase{
		{"123", []byte{0x1, 0x23}},
		{"1", []byte{0x1}},
		// leading zero
		{"01", []byte{0x1}},
		{"001", []byte{0x1}},
		{"0001", []byte{0x1}},
	} {

		expected := BytesToAddress(test.value)

		assert.Equal(t, expected, HexToAddress(test.literal))
		assert.Equal(t, expected, HexToAddress("0x"+test.literal))
	}
}

func TestAddressJSON(t *testing.T) {
	addr := Mainnet.Chain().ServiceAddress()
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
			addr:     Mainnet.Chain().ServiceAddress(),
			expected: Mainnet.Chain().ServiceAddress().Hex(),
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

func TestAccountAddress(t *testing.T) {
	t.Run("address constants", testAddressConstants)
	t.Run("address generation", testAddressGeneration)
	t.Run("chain address intersections", testAddressesIntersection)
	t.Run("index from address", testIndexFromAddress)
}

func testAddressConstants(t *testing.T) {
	// check n and k fit in 8 and 6 bytes
	assert.LessOrEqual(t, linearCodeN, 8*8)
	assert.LessOrEqual(t, linearCodeK, 6*8)

	// Test addresses for all type of networks
	chainIDs := []ChainID{
		Mainnet,
		Testnet,
		Emulator,
		Sandboxnet,
		Previewnet,
	}

	for _, chainID := range chainIDs {

		chain := chainID.Chain()

		// check the Zero and Root constants
		expected := uint64ToAddress(uint64(chainID.getChainCodeWord()))

		assert.Equal(t, chain.zeroAddress(), expected)
		expected = uint64ToAddress(generatorMatrixRows[0] ^ uint64(chainID.getChainCodeWord()))
		assert.Equal(t, chain.ServiceAddress(), expected)

		// check the transition from account zero to root
		state := chain.NewAddressGenerator()
		address, err := state.NextAddress()
		require.NoError(t, err)
		assert.Equal(t, address, chain.ServiceAddress())

		// check high state values: generation should fail for high value states
		state = chain.newAddressGeneratorAtIndex(maxIndex - 1)
		_, err = state.NextAddress()
		assert.NoError(t, err)
		_, err = state.NextAddress()
		assert.Error(t, err)

		// check zeroAddress() is an invalid address
		z := chain.zeroAddress()
		check := chain.IsValid(z)
		assert.False(t, check, "should be invalid")
	}
}

// invalid code word for all networks
const invalidCodeWord = uint64(0xab2ae42382900010)

func testAddressGeneration(t *testing.T) {
	// loops in each test
	const loop = 50

	// Test addresses for all type of networks
	chainIDs := []ChainID{
		Mainnet,
		Testnet,
		Emulator,
		Sandboxnet,
		Previewnet,
	}

	for _, chainID := range chainIDs {

		chain := chainID.Chain()

		// sanity check of NextAddress function consistency
		state := chain.NewAddressGenerator()
		expectedIndex := uint64(0)
		for i := 0; i < loop; i++ {
			expectedIndex++
			address, err := state.NextAddress()
			require.NoError(t, err)
			expectedAddress, err := chain.AddressAtIndex(expectedIndex)
			require.NoError(t, err)
			assert.Equal(t, address, expectedAddress)
			require.Equal(t, state.CurrentAddress(), expectedAddress)
		}

		// sanity check of addresses weights in Flow.
		// All addresses hamming weights must be larger than d.
		// this is only a sanity check of the implementation and not an exhaustive proof
		if chainID == Mainnet {
			r := uint64(rand.Intn(maxIndex - loop))
			state = chain.newAddressGeneratorAtIndex(r)
			for i := 0; i < loop; i++ {
				address, err := state.NextAddress()
				require.NoError(t, err)
				weight := bits.OnesCount64(address.uint64())
				assert.LessOrEqual(t, linearCodeD, weight)
			}
		}

		// sanity check of address distances.
		// All distances between any two addresses must be larger than d.
		// this is only a sanity check of the implementation and not an exhaustive proof
		r := uint64(rand.Intn(maxIndex - loop - 1))
		state = chain.newAddressGeneratorAtIndex(r)
		refAddress, err := state.NextAddress()
		require.NoError(t, err)
		for i := 0; i < loop; i++ {
			address, err := state.NextAddress()
			require.NoError(t, err)
			distance := bits.OnesCount64(address.uint64() ^ refAddress.uint64())
			assert.LessOrEqual(t, linearCodeD, distance)
		}

		// sanity check of valid account addresses.
		// All valid addresses must pass IsValid.
		r = uint64(rand.Intn(maxIndex - loop))
		state = chain.newAddressGeneratorAtIndex(r)
		for i := 0; i < loop; i++ {
			address, err := state.NextAddress()
			require.NoError(t, err)
			check := chain.IsValid(address)
			assert.True(t, check, "account address format should be valid")
		}

		// sanity check of invalid account addresses.
		// All invalid addresses must fail IsValid.
		invalidAddress := uint64ToAddress(invalidCodeWord)
		check := chain.IsValid(invalidAddress)
		assert.False(t, check, "account address format should be invalid")
		r = uint64(rand.Intn(maxIndex - loop))

		state = chain.newAddressGeneratorAtIndex(r)
		for i := 0; i < loop; i++ {
			address, err := state.NextAddress()
			require.NoError(t, err)
			invalidAddress = uint64ToAddress(address.uint64() ^ invalidCodeWord)
			check := chain.IsValid(invalidAddress)
			assert.False(t, check, "account address format should be invalid")
		}
	}
}

func testAddressesIntersection(t *testing.T) {
	// loops in each test
	const loop = 25

	// Test addresses for all type of networks
	chainIDs := []ChainID{
		Mainnet,
		Testnet,
		Emulator,
		Sandboxnet,
		Previewnet,
	}

	for _, chainID := range chainIDs {

		chain := chainID.Chain()

		// a valid address in one network must be invalid in all other networks
		r := uint64(rand.Intn(maxIndex - loop))
		state := chain.newAddressGeneratorAtIndex(r)
		for k := 0; k < loop; k++ {
			address, err := state.NextAddress()
			require.NoError(t, err)
			for _, otherChain := range chainIDs {
				if chainID != otherChain {
					check := otherChain.Chain().IsValid(address)
					assert.False(t, check, "address %s belongs to %s and should be invalid in %s",
						address, chainID, otherChain)
				} else {
					sameChainCheck := chain.IsValid(address)
					require.True(t, sameChainCheck)
				}
			}
		}

		// `invalidCodeWord` must be invalid in all networks
		// for the remaining section of the test
		require.NotEqual(t, invalidCodeWord, uint64(0))
		invalidAddress := uint64ToAddress(invalidCodeWord)
		check := chain.IsValid(invalidAddress)
		require.False(t, check, "account address format should be invalid")

		// build invalid addresses using `invalidCodeWord` and make sure they all
		// fail the check for all networks
		r = uint64(rand.Intn(maxIndex - loop))
		state = chain.newAddressGeneratorAtIndex(r)
		for k := 0; k < loop; k++ {
			address, err := state.NextAddress()
			require.NoError(t, err)
			invalidAddress = uint64ToAddress(address.uint64() ^ invalidCodeWord)
			check = chain.IsValid(invalidAddress)
			assert.False(t, check, "account address format should be invalid")
		}
	}
}

func testIndexFromAddress(t *testing.T) {
	// loops in each test
	const loop = 50

	// Test addresses for all type of networks
	chains := []Chain{
		mainnet,
		testnet,
		emulator,
		sandboxnet,
		previewnet,
	}

	for _, chain := range chains {
		for i := 0; i < loop; i++ {
			// check the correctness of IndexFromAddress

			// random valid index
			r := uint64(rand.Intn(maxIndex)) + 1
			// generate the address
			address := chain.newAddressGeneratorAtIndex(r).CurrentAddress()
			// extract the index and compare
			index, err := chain.IndexFromAddress(address)
			assert.NoError(t, err) // address should be valid
			assert.Equal(t, r, index, "wrong extracted address %d, should be %d", index, r)

			// check for invalid addresses

			// alter one bit in the address to obtain an invalid address
			address[0] ^= 1
			_, err = chain.IndexFromAddress(address)
			assert.Error(t, err)
		}
		// check the zero address error
		_, err := chain.IndexFromAddress(chain.zeroAddress())
		assert.Error(t, err)
	}
}

func TestUint48(t *testing.T) {
	const loop = 50
	// test consistensy of putUint48 and uint48
	for i := 0; i < loop; i++ {
		r := uint64(rand.Intn(1 << linearCodeK))
		b := make([]byte, addressIndexLength)
		putUint48(b, r)
		res := uint48(b)
		assert.Equal(t, r, res)
	}
}
