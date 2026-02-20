package channels

import (
	"regexp"
	"slices"
)

// Channel specifies a virtual and isolated communication medium.
// Nodes subscribed to the same channel can disseminate epidemic messages among
// each other, i.e: multicast and publish.
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

// Contains returns true if the ChannelList contains the given channel.
func (cl ChannelList) Contains(channel Channel) bool {
	return slices.Contains(cl, channel)
}

// ExcludeChannels returns list of channels that are in the ChannelList but not in the other list.
func (cl ChannelList) ExcludeChannels(other ChannelList) ChannelList {
	var result ChannelList
	for _, c := range cl {
		if !other.Contains(c) {
			result = append(result, c)
		}
	}
	return result
}

// ExcludePattern returns a new ChannelList excluding the Channels that satisfy the given predicate.
func (cl ChannelList) ExcludePattern(regexp *regexp.Regexp) ChannelList {
	var result ChannelList
	for _, c := range cl {
		if regexp.MatchString(c.String()) {
			continue
		}
		result = append(result, c)
	}
	return result
}

func (cl ChannelList) String() []string {
	var result []string
	for _, c := range cl {
		result = append(result, c.String())
	}
	return result
}
