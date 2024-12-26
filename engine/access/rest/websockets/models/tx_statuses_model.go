package models

import (
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
)

// TransactionStatusesResponse is the response message for 'events' topic.
type TransactionStatusesResponse struct {
	TransactionResults []*commonmodels.TransactionResult `json:"transaction_results"`
	MessageIndex       uint64                            `json:"message_index"`
}
