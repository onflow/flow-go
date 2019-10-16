// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package message

// Announce tells our neighbours about a new entity we received.
type Announce struct {
	Engine uint8
	ID     []byte
}
