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

// TestChannelList_ExcludeChannels tests that the channel list ExcludeChannels method returns a new channel list
// with the excluded channels removed.
func TestChannelList_ExcludeChannels(t *testing.T) {
	list := channels.ChannelList{"a", "b", "c"}
	require.ElementsMatch(t, channels.ChannelList{"a", "b"}, list.ExcludeChannels(channels.ChannelList{"c"}))
	require.ElementsMatch(t, channels.ChannelList{"a", "c"}, list.ExcludeChannels(channels.ChannelList{"b"}))
	require.ElementsMatch(t, channels.ChannelList{"b", "c"}, list.ExcludeChannels(channels.ChannelList{"a"}))
	require.ElementsMatch(t, channels.ChannelList{"a", "b", "c"}, list.ExcludeChannels(channels.ChannelList{"d"}))
	require.Empty(t, list.ExcludeChannels(channels.ChannelList{"a", "b", "c"}))
}

// TestChannelList_ExcludePattern tests that the channel list ExcludePattern method returns a new channel list
// with the filtered channels.
func TestChannelList_ExcludePattern(t *testing.T) {
	list := channels.ChannelList{"test-a", "test-b", "c", "d"}
	require.ElementsMatch(t, channels.ChannelList{"c", "d"}, list.ExcludePattern(regexp.MustCompile("^(test).*")))
	require.ElementsMatch(t, channels.ChannelList{"test-a", "test-b"}, list.ExcludePattern(regexp.MustCompile("^[cd].*")))
}
