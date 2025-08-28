package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

func ExecutorMetadataToMessage(metadata flow.ExecutorMetadata) *entities.ExecutorMetadata {
	executorIDs := make([][]byte, 0)
	for _, id := range metadata.ExecutorIDs {
		executorIDs = append(executorIDs, []byte(id.String()))
	}

	return &entities.ExecutorMetadata{
		ExecutionResultId: []byte(metadata.ExecutionResultID.String()),
		ExecutorId:        executorIDs,
	}
}
