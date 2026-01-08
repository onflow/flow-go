package emulator

import (
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRestrictedEOA(t *testing.T) {
	config := NewConfig(
		WithRestrictedEOAs(RestrictedEOAs),
	)

	t.Run("restricted address 1 should return true", func(t *testing.T) {
		addr := gethCommon.HexToAddress("0x2e7C4b71397f10c93dC0C2ba6f8f179A47F994e1")
		result := config.IsRestrictedEOA(addr)
		assert.True(t, result, "first restricted address should be detected as restricted")
	})

	t.Run("restricted address 2 should return true", func(t *testing.T) {
		addr := gethCommon.HexToAddress("0x9D9247F5C3F3B78F7EE2C480B9CDaB91393Bf4D6")
		result := config.IsRestrictedEOA(addr)
		assert.True(t, result, "second restricted address should be detected as restricted")
	})

	t.Run("non-restricted address should return false", func(t *testing.T) {
		addr := gethCommon.HexToAddress("0x1234567890123456789012345678901234567890")
		result := config.IsRestrictedEOA(addr)
		assert.False(t, result, "non-restricted address should return false")
	})

	t.Run("empty address should return false", func(t *testing.T) {
		addr := gethCommon.Address{}
		result := config.IsRestrictedEOA(addr)
		assert.False(t, result, "empty address should return false")
	})

	t.Run("case sensitivity - same address with different case should match", func(t *testing.T) {
		// Ethereum addresses are case-insensitive in hex representation
		// but gethCommon.HexToAddress normalizes to lowercase
		addr1 := gethCommon.HexToAddress("0x2e7C4b71397f10c93dC0C2ba6f8f179A47F994e1")
		addr2 := gethCommon.HexToAddress("0x2e7c4b71397f10c93dc0c2ba6f8f179a47f994e1")
		require.Equal(t, addr1, addr2, "addresses should be equal regardless of case")

		result1 := config.IsRestrictedEOA(addr1)
		result2 := config.IsRestrictedEOA(addr2)
		assert.Equal(t, result1, result2, "results should be the same for same address")
		assert.True(t, result1, "both should be detected as restricted")
	})

	t.Run("all addresses in restrictedEOAs list should be detected", func(t *testing.T) {
		for _, addr := range RestrictedEOAs {
			result := config.IsRestrictedEOA(addr)
			assert.True(t, result, "address %s should be detected as restricted", addr.Hex())
		}
	})

	t.Run("address not in list should return false", func(t *testing.T) {
		// Test with addresses that are similar but not in the list
		testAddresses := []gethCommon.Address{
			gethCommon.HexToAddress("0x2e7C4b71397f10c93dC0C2ba6f8f179A47F994e0"), // one byte different
			gethCommon.HexToAddress("0x9D9247F5C3F3B78F7EE2C480B9CDaB91393Bf4D7"), // one byte different
			gethCommon.HexToAddress("0x0000000000000000000000000000000000000001"),
			gethCommon.HexToAddress("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"),
		}

		for _, addr := range testAddresses {
			// Skip if the address is actually in the restricted list
			isInList := false
			for _, restrictedAddr := range RestrictedEOAs {
				if addr == restrictedAddr {
					isInList = true
					break
				}
			}
			if isInList {
				continue
			}

			result := config.IsRestrictedEOA(addr)
			assert.False(t, result, "address %s should not be detected as restricted", addr.Hex())
		}
	})
}
