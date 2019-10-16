// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package message

// Event represents an opaque system layer entity.
type Event struct {
	Engine  uint8
	Origin  string
	Payload []byte
}
