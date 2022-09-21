package channels_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/channels"
)

// TestChannelList_Contain tests that the channel list Contain method returns true if the channel
// is in the list.
func TestChannelList_Contain(t *testing.T) {
	list := channels.ChannelList{"a", "b", "c"}
	require.True(t, list.Contains("a"))
	require.True(t, list.Contains("b"))
	require.True(t, list.Contains("c"))
	require.Equal(t, 3, len(list))

	require.False(t, list.Contains("d"))
}

// TestChannelList_Exclude tests that the channel list Exclude method returns a new channel list
// with the excluded channels removed.
func TestChannelList_Exclude(t *testing.T) {
	list := channels.ChannelList{"a", "b", "c"}
	require.ElementsMatch(t, channels.ChannelList{"a", "b"}, list.Exclude(channels.ChannelList{"c"}))
	require.ElementsMatch(t, channels.ChannelList{"a", "c"}, list.Exclude(channels.ChannelList{"b"}))
	require.ElementsMatch(t, channels.ChannelList{"b", "c"}, list.Exclude(channels.ChannelList{"a"}))
	require.ElementsMatch(t, channels.ChannelList{"a", "b", "c"}, list.Exclude(channels.ChannelList{"d"}))
}

// TestChannelList_Filter tests that the channel list Filter method returns a new channel list
// with the filtered channels.
func TestChannelList_Filter(t *testing.T) {
	list := channels.ChannelList{"test-a", "test-b", "c", "d"}
	require.ElementsMatch(t, channels.ChannelList{"test-a", "test-b"}, list.Filter(regexp.MustCompile("^test-.*")))
	require.ElementsMatch(t, channels.ChannelList{"c", "d"}, list.Filter(regexp.MustCompile("^[^test].*")))
}
