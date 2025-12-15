package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/access"
)

// ExecutorMetadataToMessage converts an access-layer ExecutorMetadata struct
// into its protobuf message representation. If the provided metadata is nil,
// it returns nil.
func ExecutorMetadataToMessage(metadata *access.ExecutorMetadata) *entities.ExecutorMetadata {
	if metadata == nil {
		return nil
	}

	return &entities.ExecutorMetadata{
		ExecutionResultId: IdentifierToMessage(metadata.ExecutionResultID),
		ExecutorIds:       IdentifiersToMessages(metadata.ExecutorIDs),
	}
}

// MessageToExecutorMetadata converts a protobuf ExecutorMetadata message into
// the access-layer ExecutorMetadata struct. If the provided message is nil, it
// returns nil.
func MessageToExecutorMetadata(metadata *entities.ExecutorMetadata) *access.ExecutorMetadata {
	if metadata == nil {
		return nil
	}

	return &access.ExecutorMetadata{
		ExecutionResultID: MessageToIdentifier(metadata.GetExecutionResultId()),
		ExecutorIDs:       MessagesToIdentifiers(metadata.GetExecutorIds()),
	}
}
