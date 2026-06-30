package models

type Arguments map[string]any

// SubscribeMessageRequest represents a request to subscribe to a topic.
type SubscribeMessageRequest struct {
	BaseMessageRequest
	Topic     string    `json:"topic"`     // Topic to subscribe to
	Arguments Arguments `json:"arguments"` // Additional arguments for subscription
}

// SubscribeMessageResponse represents the response to a subscription request.
type SubscribeMessageResponse struct {
	BaseMessageResponse
}
