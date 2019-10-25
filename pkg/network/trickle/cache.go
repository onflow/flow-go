// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

import (
	"github.com/dapperlabs/flow-go/pkg/model/trickle"
)

// Cache is responsible for caching events on the overlay layer, so that we
// don't need to look them up on the application layer for each request.
type Cache interface {
	Add(engineID uint8)
	Has(engineID uint8, eventID []byte) bool
	Set(engineID uint8, eventID []byte, res *trickle.Response)
	Get(engineID uint8, eventID []byte) (*trickle.Response, bool)
}
