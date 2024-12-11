package models

// AccountStatusesResponse is the response message for 'events' topic.
type AccountStatusesResponse struct {
	BlockID       string        `json:"blockID"`
	Height        string        `json:"height"`
	AccountEvents AccountEvents `json:"account_events"`
	MessageIndex  string        `json:"message_index"`
}
