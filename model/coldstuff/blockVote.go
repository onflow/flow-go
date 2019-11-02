// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

// BlockVote is a coldstuff consensus event to vote for a block.
type BlockVote struct {
	Hash []byte
}
