package models

// UnsubscribeMessageRequest represents a request to unsubscribe from a topic.
type UnsubscribeMessageRequest struct {
	BaseMessageRequest
}

// UnsubscribeMessageResponse represents the response to an unsubscription request.
type UnsubscribeMessageResponse struct {
	BaseMessageResponse
}
