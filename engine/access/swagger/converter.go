package swagger

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/swagger/generated"
	"github.com/onflow/flow-go/model/flow"
)

// Converter provides the conversion function to/from the Swagger object to Flow objects

func toBlock(flowBlock *flow.Block) *generated.Block {
	return &generated.Block{
		Header: toBlockHeader(flowBlock.Header),
	}
}

func toBlockHeader(flowHeader *flow.Header) *generated.BlockHeader {
	return &generated.BlockHeader{
		Id:                   flowHeader.ID().String(),
		ParentId:             flowHeader.ParentID.String(),
		Height:               fmt.Sprint(flowHeader.Height),
		Timestamp:            flowHeader.Timestamp,
		ParentVoterSignature: fmt.Sprint(flowHeader.ParentVoterSigData),
	}
}
