package request

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetScript_InvalidParse verifies that parseGetScripts correctly returns errors
// for invalid input combinations and malformed request parameters.
func TestGetScript_InvalidParse(t *testing.T) {
	validScript := fmt.Sprintf(`{ "script": "%s", "arguments": [] }`, util.ToBase64([]byte(`access(all) fun main() {}`)))
	tests := []struct {
		height                  string
		id                      string
		script                  string
		agreeingExecutorsCount  string
		agreeingExecutorsIds    []string
		includeExecutorMetadata string
		err                     string
	}{
		{
			"",
			"",
			"",
			"",
			[]string{},
			"",
			"request body must not be empty",
		},
		{
			"1",
			"7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7",
			validScript,
			"",
			[]string{},
			"",
			"can not provide both block ID and block height",
		},
		{
			"final",
			"7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7",
			validScript,
			"",
			[]string{},
			"",
			"can not provide both block ID and block height",
		},
		{
			"",
			"2",
			validScript,
			"",
			[]string{},
			"", "invalid ID format",
		},

		{
			"1",
			"",
			`{ "foo": "zoo" }`,
			"",
			[]string{},
			"",
			`request body contains unknown field "foo"`,
		},
		{
			"1",
			"",
			validScript,
			"abc",
			unittest.IdentifierListFixture(1).Strings(),
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			"1",
			"",
			validScript,
			"-5",
			unittest.IdentifierListFixture(2).Strings(),
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			"1",
			"",
			validScript,
			"4",
			[]string{"not-a-valid-id"},
			"false",
			"invalid ID format",
		},
		{
			"1",
			"",
			validScript,
			"4",
			unittest.IdentifierListFixture(2).Strings(),
			"not-bool",
			"invalid includeExecutorMetadata",
		},
	}

	for i, test := range tests {
		request, err := parseGetScripts(
			test.height,
			test.id,
			test.agreeingExecutorsCount,
			test.agreeingExecutorsIds,
			test.includeExecutorMetadata,
			strings.NewReader(test.script),
		)
		require.Nil(t, request)
		require.ErrorContains(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

// TestGetScript_ValidParse verifies that parseGetScripts successfully parses
// valid GetScript requests and populates the request fields as expected.
func TestGetScript_ValidParse(t *testing.T) {
	source := "access(all) fun main() {}"
	validScript := strings.NewReader(fmt.Sprintf(`{ "script": "%s", "arguments": [] }`, util.ToBase64([]byte(source))))
	agreeingExecutorsCount := uint64(2)

	validAgreeingExecutorsIds := unittest.IdentifierListFixture(2)
	agreeingExecutorsCountStr := fmt.Sprintf("%d", agreeingExecutorsCount)
	validAgreeingExecutorsIdsStr := validAgreeingExecutorsIds.Strings()

	request, err := parseGetScripts("1", "", agreeingExecutorsCountStr, validAgreeingExecutorsIdsStr, "true", validScript)
	require.NoError(t, err)
	require.Equal(t, request.BlockHeight, uint64(1))
	require.Equal(t, string(request.Script.Source), source)
	require.Equal(t, request.ExecutionState.AgreeingExecutorsCount, agreeingExecutorsCount)
	require.EqualValues(t, request.ExecutionState.RequiredExecutorIDs, validAgreeingExecutorsIds)
	require.True(t, request.ExecutionState.IncludeExecutorMetadata)

	validScript1 := strings.NewReader(fmt.Sprintf(`{ "script": "%s", "arguments": [] }`, util.ToBase64([]byte(source))))
	request, err = parseGetScripts("", "", agreeingExecutorsCountStr, validAgreeingExecutorsIdsStr, "false", validScript1)
	require.NoError(t, err)
	require.Equal(t, request.BlockHeight, SealedHeight)
}
