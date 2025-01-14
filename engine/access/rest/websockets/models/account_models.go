package models

// AccountStatusesResponse is the response message for 'events' topic.
type AccountStatusesResponse struct {
	BlockID       string        `json:"block_id"`
	Height        string        `json:"height"`
	AccountEvents AccountEvents `json:"account_events"`
	MessageIndex  uint64        `json:"message_index"`
}
