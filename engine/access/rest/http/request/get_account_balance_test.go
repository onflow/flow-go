package request

import (
	"fmt"
	"testing"

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
		name                    string
		address                 string
		height                  string
		agreeingExecutorsCount  string
		agreeingExecutorsIds    []string
		includeExecutorMetadata string
		err                     string
	}{
		{
			"parse with invalid address",
			"",
			"",
			"2",
			validAgreeingExecutorsIds,
			"false",
			"invalid address",
		},
		{
			"parse with invalid height format",
			validAddress,
			"-1",
			"2",
			validAgreeingExecutorsIds,
			"false",
			"invalid height format",
		},
		{
			"parse with invalid agreeingExecutorCount value",
			validAddress,
			"",
			"abc",
			validAgreeingExecutorsIds,
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			"parse with invalid agreeingExecutorCount number",
			validAddress,
			"",
			"-5",
			validAgreeingExecutorsIds,
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			"parse with invalid agreeingExecutorsIds",
			validAddress,
			"",
			"2",
			[]string{"not-a-valid-id"},
			"false",
			"invalid ID format",
		},
		{
			"parse with invalid includeExecutorMetadata",
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
		t.Run(test.name, func(t *testing.T) {
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
		})
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
	require.NoError(t, err)
	require.Equal(t, request.Address.String(), validAddress)
	require.Equal(t, request.Height, SealedHeight)

	request, err = parseGetAccountBalanceRequest(validAddress, "100", "2", validAgreeingExecutorsIds, "false", chain)
	require.NoError(t, err)
	require.Equal(t, request.Height, uint64(100))

	request, err = parseGetAccountBalanceRequest(validAddress, sealed, "2", validAgreeingExecutorsIds, "false", chain)
	require.NoError(t, err)
	require.Equal(t, request.Height, SealedHeight)

	request, err = parseGetAccountBalanceRequest(validAddress, final, "2", validAgreeingExecutorsIds, "false", chain)
	require.NoError(t, err)
	require.Equal(t, request.Height, FinalHeight)
}
