package models

type Arguments map[string]interface{}

// SubscribeMessageRequest represents a request to subscribe to a topic.
type SubscribeMessageRequest struct {
	BaseMessageRequest
	Topic     string    `json:"topic"`     // Topic to subscribe to
	Arguments Arguments `json:"arguments"` // Arguments are the arguments for the subscribed topic
}

// SubscribeMessageResponse represents the response to a subscription request.
type SubscribeMessageResponse struct {
	BaseMessageResponse
}
