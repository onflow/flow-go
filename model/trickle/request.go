// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

// Request is a request for an item.
type Request struct {
	EngineID uint8
	EventID  []byte
}
