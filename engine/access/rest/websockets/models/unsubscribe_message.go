package models

// UnsubscribeMessageRequest represents a request to unsubscribe from a topic.
type UnsubscribeMessageRequest struct {
	BaseMessageRequest
	SubscriptionID string `json:"id"`
}

// UnsubscribeMessageResponse represents the response to an unsubscription request.
type UnsubscribeMessageResponse struct {
	BaseMessageResponse
}
