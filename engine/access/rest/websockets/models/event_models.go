package models

import (
	"time"

	"github.com/onflow/flow-go/engine/access/rest/common/models"
)

// EventResponse is the response message for 'events' topic.
type EventResponse struct {
	BlockId        string        `json:"block_id"`
	BlockHeight    string        `json:"block_height"`
	BlockTimestamp time.Time     `json:"block_timestamp"`
	Events         models.Events `json:"events"`
	MessageIndex   string        `json:"message_index"`
}
