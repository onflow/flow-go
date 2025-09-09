package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/access"
)

func ExecutorMetadataToMessage(metadata *access.ExecutorMetadata) (*entities.ExecutorMetadata, error) {
	if metadata == nil {
		return nil, ErrEmptyMessage
	}

	return &entities.ExecutorMetadata{
		ExecutionResultId: IdentifierToMessage(metadata.ExecutionResultID),
		ExecutorIds:       IdentifiersToMessages(metadata.ExecutorIDs),
	}, nil
}

func MessageToExecutorMetadata(metadata *entities.ExecutorMetadata) (*access.ExecutorMetadata, error) {
	if metadata == nil {
		return nil, ErrEmptyMessage
	}

	return &access.ExecutorMetadata{
		ExecutionResultID: MessageToIdentifier(metadata.ExecutionResultId),
		ExecutorIDs:       MessagesToIdentifiers(metadata.ExecutorIds),
	}, nil
}
