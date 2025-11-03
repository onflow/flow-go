package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetAccountBalance_InvalidParse verifies that parseGetAccountBalanceRequest correctly returns errors
// for invalid input combinations and malformed request parameters.
//
// Test cases:
//  1. A request with an empty account address.
//  2. A request with a negative block height.
//  3. A request with a non-numeric agreeingExecutorsCount value.
//  4. A request with a negative agreeingExecutorsCount value.
//  5. A request containing invalid executor IDs.
//  6. A request with a non-boolean includeExecutorMetadata flag.
func TestGetAccountBalance_InvalidParse(t *testing.T) {
	validAddress := flow.Localnet.Chain().ServiceAddress().String()
	validAgreeingExecutorsIds := unittest.IdentifierListFixture(2).Strings()

	tests := []struct {
		address                 string
		height                  string
		agreeingExecutorsCount  string
		agreeingExecutorsIds    []string
		includeExecutorMetadata string
		err                     string
	}{
		{
			"",
			"",
			"2",
			validAgreeingExecutorsIds,
			"false",
			"invalid validAddress",
		},
		{
			validAddress,
			"-1",
			"2",
			validAgreeingExecutorsIds,
			"false",
			"invalid height format",
		},
		{
			validAddress,
			"",
			"abc",
			validAgreeingExecutorsIds,
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			validAddress,
			"",
			"-5",
			unittest.IdentifierListFixture(2).Strings(),
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			validAddress,
			"",
			"2",
			[]string{"not-a-valid-id"},
			"false",
			"invalid ID format",
		},
		{
			validAddress,
			"",
			"2",
			validAgreeingExecutorsIds,
			"not-bool",
			"invalid includeExecutorMetadata",
		},
	}

	chain := flow.Localnet.Chain()
	for i, test := range tests {
		_, err := parseGetAccountBalanceRequest(
			test.address,
			test.height,
			test.agreeingExecutorsCount,
			test.agreeingExecutorsIds,
			test.includeExecutorMetadata,
			chain,
		)
		//TODO(Uliana):
		//require.Nil(t, request)
		require.ErrorContains(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

// TestGetAccountBalance_ValidParse verifies that parseGetAccountBalanceRequest successfully parses
// valid GetAccountBalance requests and populates the request fields as expected.
//
// Test cases:
//  1. A request with a valid account address and no specified height (defaults to sealed height).
//  2. A request with a valid block height.
//  3. A request using the "sealed" as the block height.
//  4. A request using the "final" as the block height.
func TestGetAccountBalance_ValidParse(t *testing.T) {
	validAddress := flow.Localnet.Chain().ServiceAddress().String()
	validAgreeingExecutorsIds := unittest.IdentifierListFixture(2).Strings()
	chain := flow.Localnet.Chain()

	request, err := parseGetAccountBalanceRequest(validAddress, "", "2", validAgreeingExecutorsIds, "false", chain)
	assert.NoError(t, err)
	assert.Equal(t, request.Address.String(), validAddress)
	assert.Equal(t, request.Height, SealedHeight)

	request, err = parseGetAccountBalanceRequest(validAddress, "100", "2", validAgreeingExecutorsIds, "false", chain)
	assert.NoError(t, err)
	assert.Equal(t, request.Height, uint64(100))

	request, err = parseGetAccountBalanceRequest(validAddress, sealed, "2", validAgreeingExecutorsIds, "false", chain)
	assert.NoError(t, err)
	assert.Equal(t, request.Height, SealedHeight)

	request, err = parseGetAccountBalanceRequest(validAddress, final, "2", validAgreeingExecutorsIds, "false", chain)
	assert.NoError(t, err)
	assert.Equal(t, request.Height, FinalHeight)
}
