package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetAccount_InvalidParse verifies that parseGetAccountRequest correctly
// returns errors for invalid input parameters and malformed request values.
//
// Test cases:
//  1. A request with an empty account address.
//  2. A request with an invalid (negative) block height.
//  3. A request with a non-numeric agreeingExecutorsCount value.
//  4. A request with a negative agreeingExecutorsCount value.
//  5. A request containing invalid executor IDs.
//  6. A request with a non-boolean includeExecutorMetadata value.
func TestGetAccount_InvalidParse(t *testing.T) {
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
			"invalid address",
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
			"1",
			"abc",
			validAgreeingExecutorsIds,
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			validAddress,
			"1",
			"-5",
			validAgreeingExecutorsIds,
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			validAddress,
			"1",
			"2",
			[]string{"not-a-valid-id"},
			"false",
			"invalid ID format",
		},
		{
			validAddress,
			"1",
			"2",
			validAgreeingExecutorsIds,
			"not-bool",
			"invalid includeExecutorMetadata",
		},
	}

	chain := flow.Localnet.Chain()
	for i, test := range tests {
		request, err := parseGetAccountRequest(
			test.address,
			test.height,
			test.agreeingExecutorsCount,
			test.agreeingExecutorsIds,
			test.includeExecutorMetadata,
			chain,
		)
		require.Nil(t, request)
		require.ErrorContains(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

// TestGetAccount_ValidParse verifies that parseGetAccountRequest successfully
// parses valid GetAccount requests and populates the request fields as expected.
//
// Test cases:
//  1. A request with a valid account address and no specified height (defaults to sealed height).
//  2. A request with a valid block height.
func TestGetAccount_ValidParse(t *testing.T) {
	validAddress := flow.Localnet.Chain().ServiceAddress().String()
	validAgreeingExecutorsIds := unittest.IdentifierListFixture(2)
	chain := flow.Localnet.Chain()
	agreeingExecutorsCount := uint64(2)

	agreeingExecutorsCountStr := fmt.Sprintf("%d", agreeingExecutorsCount)
	validAgreeingExecutorsIdsStr := validAgreeingExecutorsIds.Strings()

	request, err := parseGetAccountRequest(validAddress, "", agreeingExecutorsCountStr, validAgreeingExecutorsIdsStr, "true", chain)
	require.NoError(t, err)
	require.Equal(t, request.Address.String(), validAddress)
	require.Equal(t, request.Height, SealedHeight)
	require.Equal(t, request.ExecutionState.AgreeingExecutorsCount, agreeingExecutorsCount)
	require.EqualValues(t, request.ExecutionState.RequiredExecutorIDs, validAgreeingExecutorsIds)
	require.True(t, request.ExecutionState.IncludeExecutorMetadata)

	request, err = parseGetAccountRequest(validAddress, "100", agreeingExecutorsCountStr, validAgreeingExecutorsIdsStr, "false", chain)
	require.NoError(t, err)
	require.Equal(t, request.Height, uint64(100))
}
