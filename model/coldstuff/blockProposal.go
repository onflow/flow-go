// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

// BlockProposal is a coldstuff consensus event to propose a block.
type BlockProposal struct {
	Header *BlockHeader
}
