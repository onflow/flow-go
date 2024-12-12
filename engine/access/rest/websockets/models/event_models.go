package models

import (
	"github.com/onflow/flow-go/engine/access/rest/common/models"
)

// EventResponse is the response message for 'events' topic.
type EventResponse struct {
	models.BlockEvents        // Embed BlockEvents struct to reuse its fields
	MessageIndex       string `json:"message_index"`
}
