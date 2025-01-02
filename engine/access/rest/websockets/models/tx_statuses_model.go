package models

import (
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
)

// TransactionStatusesResponse is the response message for 'events' topic.
type TransactionStatusesResponse struct {
	TransactionResult *commonmodels.TransactionResult `json:"transaction_result"`
	MessageIndex      uint64                          `json:"message_index"`
}
