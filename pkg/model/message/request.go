// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package message

// Request is a request for an item.
type Request struct {
	Engine uint8
	ID     []byte
}
