package types

import (
	"fmt"
	"github.com/dapperlabs/flow-go/model/flow"
)

type BlockProposal struct {
	Block            *Block
	ConsensusPayload *ConsensusPayload
	Signature        *Signature // sign(View, BlockMRH)
}

func NewBlockProposal(block *Block, consensusPayload *ConsensusPayload, sig *Signature) *BlockProposal {
	return &BlockProposal{
		Block:            block,
		ConsensusPayload: consensusPayload,
		Signature:        sig,
	}
}

func (b BlockProposal) QC() *QuorumCertificate    { return b.Block.QC }
func (b BlockProposal) View() uint64              { return b.Block.View }
func (b BlockProposal) BlockMRH() flow.Identifier { return b.Block.BlockMRH() }
func (b BlockProposal) BlockMRHStr() string       { return fmt.Sprintf("%x", b.Block.BlockMRH()) }
func (b BlockProposal) Height() uint64            { return b.Block.Height }

func (b BlockProposal) BytesForSig() []byte {
	// the bytes for signing a block proposal is the same as the bytes for signing a vote
	return voteBytesForSig(b.View(), b.BlockMRH())
}
