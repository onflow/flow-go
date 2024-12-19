package models

import "github.com/onflow/flow-go/model/flow"

// AccountStatusesResponse is the response message for 'events' topic.
type AccountStatusesResponse struct {
	BlockID       string                     `json:"blockID"`
	Height        string                     `json:"height"`
	AccountEvents map[string]flow.EventsList `json:"account_events"`
	MessageIndex  uint64                     `json:"message_index"`
}
