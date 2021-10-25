package swagger

import "github.com/onflow/flow-go/model/flow"

func toBlock(flowBlock *flow.Block) *Block {
	return &Block{
		Header: toBlockHeader(flowBlock.Header),
	}
}

func toBlockHeader(flowHeader *flow.Header) *BlockHeader {
	return &BlockHeader{
		Id: flowHeader.ID().String(),
		ParentId: flowHeader.ParentID.String(),
		Height: string(flowHeader.Height),
		Timestamp: flowHeader.Timestamp,
		ParentVoterSignature: string(flowHeader.ParentVoterSigData),
	}
}
