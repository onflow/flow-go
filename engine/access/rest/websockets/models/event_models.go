package models

import "github.com/onflow/flow-go/model/flow"

// EventResponse is the response message for 'events' topic.
type EventResponse struct {
	Event *flow.Event `json:"event"`
}
