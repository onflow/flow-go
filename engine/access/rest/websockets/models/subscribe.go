package models

// SubscribeMessageRequest represents a request to subscribe to a topic.
type SubscribeMessageRequest struct {
	BaseMessageRequest
	Topic     string                 `json:"topic"`     // Topic to subscribe to
	Arguments map[string]interface{} `json:"arguments"` // Additional arguments for subscription
}

// SubscribeMessageResponse represents the response to a subscription request.
type SubscribeMessageResponse struct {
	BaseMessageResponse
	Topic string `json:"topic"` // Topic of the subscription
	ID    string `json:"id"`    // Unique subscription ID
}
