package channels_test

import (
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

func TestChannelList_Exclude(t *testing.T) {
	list := channels.ChannelList{"a", "b", "c"}
	require.ElementsMatch(t, channels.ChannelList{"a", "b"}, list.Exclude(channels.ChannelList{"c"}))
	require.ElementsMatch(t, channels.ChannelList{"a", "c"}, list.Exclude(channels.ChannelList{"b"}))
	require.ElementsMatch(t, channels.ChannelList{"b", "c"}, list.Exclude(channels.ChannelList{"a"}))
	require.ElementsMatch(t, channels.ChannelList{"a", "b", "c"}, list.Exclude(channels.ChannelList{"d"}))
}
