// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"time"
)

// BlockHeader represents the header of a black.
type BlockHeader struct {
	Height    uint64
	Nonce     uint64
	Timestamp time.Time
	Parent    []byte
	Payload   []byte
}
