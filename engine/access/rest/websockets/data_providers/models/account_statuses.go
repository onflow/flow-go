package models

import (
	"strconv"

	"github.com/onflow/flow-go/engine/access/state_stream/backend"
)

// AccountStatusesResponse is the response message for 'events' topic.
type AccountStatusesResponse struct {
	BlockID       string        `json:"block_id"`
	Height        string        `json:"height"`
	AccountEvents AccountEvents `json:"account_events"`
	MessageIndex  uint64        `json:"message_index"`
}

func NewAccountStatusesResponse(accountStatusesResponse *backend.AccountStatusesResponse, index uint64) *AccountStatusesResponse {
	accountEvents := NewAccountEvents(accountStatusesResponse.AccountEvents)

	return &AccountStatusesResponse{
		BlockID:       accountStatusesResponse.BlockID.String(),
		Height:        strconv.FormatUint(accountStatusesResponse.Height, 10),
		AccountEvents: accountEvents,
		MessageIndex:  index,
	}
}
