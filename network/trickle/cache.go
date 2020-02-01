// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package trickle

import (
	"github.com/dapperlabs/flow-go/model/trickle"
)

// Deprecated: use libp2p.RcvCache instead
//
// Cache is responsible for caching events on the overlay layer, so that we
// don't need to look them up on the application layer for each request.
type Cache interface {
	Add(channelID uint8)
	Has(channelID uint8, eventID []byte) bool
	Set(channelID uint8, eventID []byte, res *trickle.Response)
	Get(channelID uint8, eventID []byte) (*trickle.Response, bool)
}
