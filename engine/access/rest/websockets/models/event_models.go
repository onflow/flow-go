package models

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// EventResponse is the response message for 'events' topic.
type EventResponse struct {
	BlockId        string       `json:"block_id"`
	BlockHeight    string       `json:"block_height"`
	BlockTimestamp time.Time    `json:"block_timestamp"`
	Events         []flow.Event `json:"events"`
	MessageIndex   uint64       `json:"message_index"`
}
