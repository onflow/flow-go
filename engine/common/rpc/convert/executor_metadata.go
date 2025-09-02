package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

func ExecutorMetadataToMessage(metadata flow.ExecutorMetadata) *entities.ExecutorMetadata {
	return &entities.ExecutorMetadata{
		ExecutionResultId: IdentifierToMessage(metadata.ExecutionResultID),
		ExecutorId:        IdentifiersToMessages(metadata.ExecutorIDs),
	}
}
