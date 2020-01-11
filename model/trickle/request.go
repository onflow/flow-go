// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

// Request is a request for an item.
type Request struct {
	ChannelID uint8
	EventID   []byte
}
