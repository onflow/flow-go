package network

import (
	"sort"

	"github.com/onflow/flow-go/model/flow"
)

// Channel specifies a virtual and isolated communication medium.
// Nodes subscribed to the same channel can disseminate epidemic messages among
// each other, i.e.. multicast and publish.
type Channel string
type ChannelList []Channel

func (c Channel) String() string {
	return string(c)
}

// Len returns length of the ChannelList in the number of stored Channels.
// It satisfies the sort.Interface making the ChannelList sortable.
func (cl ChannelList) Len() int {
	return len(cl)
}

// Less returns true if element i in the ChannelList  is less than j based on the numerical value of its Channel.
// Otherwise it returns true.
// It satisfies the sort.Interface making the ChannelList sortable.
func (cl ChannelList) Less(i, j int) bool {
	return cl[i] < cl[j]
}

// Swap swaps the element i and j in the ChannelList.
// It satisfies the sort.Interface making the ChannelList sortable.
func (cl ChannelList) Swap(i, j int) {
	cl[i], cl[j] = cl[j], cl[i]
}

// ID returns hash of the content of ChannelList. It first sorts the ChannelList and then takes its
// hash value.
func (cl ChannelList) ID() flow.Identifier {
	sort.Sort(cl)
	return flow.MakeID(cl)
}

// Contains retuns true if the ChannelList contains the given channel.
func (cl ChannelList) Contains(channel Channel) bool {
	for _, c := range cl {
		if c == channel {
			return true
		}
	}
	return false
}
