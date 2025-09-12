package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/access"
)

func ExecutorMetadataToMessage(metadata *access.ExecutorMetadata) *entities.ExecutorMetadata {
	if metadata == nil {
		return nil
	}

	return &entities.ExecutorMetadata{
		ExecutionResultId: IdentifierToMessage(metadata.ExecutionResultID),
		ExecutorId:        IdentifiersToMessages(metadata.ExecutorIDs),
	}
}

func MessageToExecutorMetadata(metadata *entities.ExecutorMetadata) *access.ExecutorMetadata {
	if metadata == nil {
		return nil
	}

	return &access.ExecutorMetadata{
		ExecutionResultID: MessageToIdentifier(metadata.ExecutionResultId),
		ExecutorIDs:       MessagesToIdentifiers(metadata.ExecutorId),
	}
}
