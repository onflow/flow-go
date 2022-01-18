package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEvents_InvalidParse(t *testing.T) {
	var getEvents GetEvents

	tests := []struct {
		eventType string
		start     string
		end       string
		ids       []string
		err       string
	}{
		{"flow.AccountCreated", "", "", nil, "must provide either block IDs or start and end height range"},
		{"flow.AccountCreated", "10", "", nil, "must provide either block IDs or start and end height range"},
		{"flow.AccountCreated", "", "10", nil, "must provide either block IDs or start and end height range"},
		{"flow.AccountCreated", "5", "10", []string{"7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7"}, "can only provide either block IDs or start and end height range"},
		{"foo", "5", "10", nil, "invalid event type format"},
		{"A.123.Foo.Bar", "5", "10", nil, "invalid event type format"},
		{"A.f8d6e0586b0a20c7.Foo.Bar", "20", "10", nil, "start height must be less than or equal to end height"},
		{"A.f8d6e0586b0a20c7.Foo.Bar", "0", "500", nil, "height range 500 exceeds maximum allowed of 50"},
		{"A.f8d6e0586b0a20c7.Foo.Bar", "0", "", make([]string, 100), "at most 50 IDs can be requested at a time"},
	}

	for i, test := range tests {
		err := getEvents.Parse(test.eventType, test.start, test.end, test.ids)
		assert.EqualError(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

func TestGetEvents_ValidParse(t *testing.T) {
	var getEvents GetEvents

	event := "A.f8d6e0586b0a20c7.Foo.Bar"
	err := getEvents.Parse(event, "5", "10", nil)
	assert.NoError(t, err)
	assert.Equal(t, getEvents.Type, event)
	assert.Equal(t, getEvents.StartHeight, uint64(5))
	assert.Equal(t, getEvents.EndHeight, uint64(10))
	assert.Equal(t, len(getEvents.BlockIDs), 0)

	event = "flow.AccountCreated"
	err = getEvents.Parse(event, "", "", []string{
		"7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7",
		"7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7", // intentional duplication
		"2ab81061b12d95fb81f2923001e340bc808e67e1eaae3c62479057cc14eb57fd",
	})
	assert.NoError(t, err)
	assert.Equal(t, getEvents.Type, event)
	assert.Equal(t, getEvents.StartHeight, EmptyHeight)
	assert.Equal(t, getEvents.EndHeight, EmptyHeight)
	assert.Equal(t, len(getEvents.BlockIDs), 2)
	assert.Equal(t, getEvents.BlockIDs[0].String(), "7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7")
	assert.Equal(t, getEvents.BlockIDs[1].String(), "2ab81061b12d95fb81f2923001e340bc808e67e1eaae3c62479057cc14eb57fd")

}
