package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetEvents_InvalidParse verifies that parseGetEvents returns descriptive
// errors for a variety of invalid input combinations, including malformed
// event types, invalid height ranges, excessive IDs, and invalid boolean or
// numeric parameters.
func TestGetEvents_InvalidParse(t *testing.T) {
	tests := []struct {
		eventType               string
		start                   string
		end                     string
		ids                     []string
		agreeingExecutorsCount  string
		agreeingExecutorsIds    []string
		includeExecutorMetadata string
		err                     string
	}{
		{
			"flow.AccountCreated",
			"",
			"",
			nil,
			"",
			[]string{},
			"",
			"must provide either block IDs or start and end height range",
		},
		{
			"flow.AccountCreated",
			"10",
			"",
			nil,
			"",
			[]string{},
			"",
			"must provide either block IDs or start and end height range",
		},
		{
			"flow.AccountCreated",
			"",
			"10",
			nil,
			"",
			[]string{},
			"",
			"must provide either block IDs or start and end height range",
		},
		{
			"flow.AccountCreated",
			"5",
			"10",
			[]string{"7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7"},
			"",
			[]string{},
			"",
			"can only provide either block IDs or start and end height range",
		},
		{
			"foo",
			"5",
			"10",
			nil,
			"",
			[]string{},
			"",
			"invalid event type format",
		},
		{
			"A.123.Foo.Bar",
			"5",
			"10",
			nil,
			"",
			[]string{},
			"",
			"invalid event type format",
		},
		{
			"A.f8d6e0586b0a20c7.Foo.Bar",
			"20",
			"10",
			nil,
			"",
			[]string{},
			"",
			"start height must be less than or equal to end height",
		},
		{
			"A.f8d6e0586b0a20c7.Foo.Bar",
			"0",
			"500",
			nil,
			"",
			[]string{},
			"",
			"height range 500 exceeds maximum allowed of 250",
		},
		{
			"A.f8d6e0586b0a20c7.Foo.Bar",
			"0",
			"",
			make([]string, 100),
			"",
			[]string{},
			"",
			"at most 50 IDs can be requested at a time",
		},
		{
			"A.f8d6e0586b0a20c7.Foo.Bar",
			"0",
			"10",
			[]string{},
			"abc",
			unittest.IdentifierListFixture(1).Strings(),
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			"A.f8d6e0586b0a20c7.Foo.Bar",
			"0",
			"10",
			[]string{},
			"-5",
			unittest.IdentifierListFixture(2).Strings(),
			"false",
			"invalid agreeingExecutorCount",
		},
		{
			"A.f8d6e0586b0a20c7.Foo.Bar",
			"0",
			"10",
			[]string{},
			"4",
			[]string{"not-a-valid-id"},
			"false",
			"invalid ID format",
		},
		{
			"A.f8d6e0586b0a20c7.Foo.Bar",
			"0",
			"10",
			[]string{},
			"4",
			unittest.IdentifierListFixture(2).Strings(),
			"not-bool",
			"invalid includeExecutorMetadata",
		},
	}

	for i, test := range tests {
		_, err := parseGetEvents(
			test.eventType,
			test.start,
			test.end,
			test.ids,
			test.agreeingExecutorsCount,
			test.agreeingExecutorsIds,
			test.includeExecutorMetadata,
		)
		assert.ErrorContains(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

// TestGetEvents_ValidParse ensures parseGetEvents successfully parses valid
// queries for both height range and block ID modes, de-duplicates block IDs,
// and populates all fields as expected.
func TestGetEvents_ValidParse(t *testing.T) {
	event := "A.f8d6e0586b0a20c7.Foo.Bar"
	getEvents, err := parseGetEvents(
		event,
		"5",
		"10",
		nil,
		"2",
		unittest.IdentifierListFixture(2).Strings(),
		"true",
	)
	assert.NoError(t, err)
	assert.Equal(t, getEvents.Type, event)
	assert.Equal(t, getEvents.StartHeight, uint64(5))
	assert.Equal(t, getEvents.EndHeight, uint64(10))
	assert.Equal(t, len(getEvents.BlockIDs), 0)

	event = "flow.AccountCreated"
	getEvents, err = parseGetEvents(
		event,
		"",
		"",
		[]string{
			"7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7",
			"7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7", // intentional duplication
			"2ab81061b12d95fb81f2923001e340bc808e67e1eaae3c62479057cc14eb57fd",
		},
		"2",
		unittest.IdentifierListFixture(2).Strings(),
		"true",
	)
	assert.NoError(t, err)
	assert.Equal(t, getEvents.Type, event)
	assert.Equal(t, getEvents.StartHeight, EmptyHeight)
	assert.Equal(t, getEvents.EndHeight, EmptyHeight)
	assert.Equal(t, len(getEvents.BlockIDs), 2)
	assert.Equal(t, getEvents.BlockIDs[0].String(), "7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7")
	assert.Equal(t, getEvents.BlockIDs[1].String(), "2ab81061b12d95fb81f2923001e340bc808e67e1eaae3c62479057cc14eb57fd")

}
