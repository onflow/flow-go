// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

// BlockCommit is a coldstuff consensus event to commit a block.
type BlockCommit struct {
	Hash []byte
}
