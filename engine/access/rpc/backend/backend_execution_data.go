package backend

import (
	"context"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

type backendExecutionData struct {
	blocks   storage.Blocks
	execData jobs.ExecutionDataReader
}

func (b *backendExecutionData) GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*entities.BlockExecutionData, error) {
	block, err := b.blocks.ByID(blockID)
	if err != nil {
		return nil, err
	}

	job, err := b.execData.AtIndex(block.Header.Height)
	if err != nil {
		return nil, err
	}

	blockEntry, err := jobs.JobToBlockEntry(job)
	if err != nil {
		return nil, err
	}

	message, err := convert.BlockExecutionDataToMessage(blockEntry.ExecutionData)
	if err != nil {
		return nil, err
	}

	return message, nil
}
