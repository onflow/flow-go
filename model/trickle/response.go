// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

// Response represents an opaque system layer event.
type Response struct {
	EngineID  uint8
	EventID   []byte
	OriginID  string
	TargetIDs []string
	Payload   []byte
}
