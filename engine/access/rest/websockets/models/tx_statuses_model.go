package models

import (
	"github.com/onflow/flow-go/access"
)

// TransactionStatusesResponse is the response message for 'events' topic.
type TransactionStatusesResponse struct {
	TransactionResults []*access.TransactionResult `json:"transaction_results"`
	MessageIndex       string                      `json:"message_index"`
}
