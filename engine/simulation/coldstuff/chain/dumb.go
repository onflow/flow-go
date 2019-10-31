// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package chain

import (
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/coldstuff"
)

// Dumb chain provides a storage for blocks that is super dumb.
type Dumb struct {
	height  uint64
	headers map[uint64]*coldstuff.BlockHeader
}

// NewDumb creates a new dumb blockchain store.
func NewDumb(genesis *coldstuff.BlockHeader) *Dumb {
	d := &Dumb{
		height:  0,
		headers: make(map[uint64]*coldstuff.BlockHeader),
	}
	d.headers[0] = genesis
	return d
}

// Head returns the currently highest block header.
func (d *Dumb) Head() *coldstuff.BlockHeader {
	return d.headers[d.height]
}

// Commit adds the given block header to top of the chain.
func (d *Dumb) Commit(header *coldstuff.BlockHeader) error {
	if header.Height != d.height+1 {
		return errors.New("wrong height for new block")
	}
	d.height++
	d.headers[d.height] = header
	return nil
}
