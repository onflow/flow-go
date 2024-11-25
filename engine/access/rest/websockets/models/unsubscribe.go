package models

// UnsubscribeMessageRequest represents a request to unsubscribe from a topic.
type UnsubscribeMessageRequest struct {
	BaseMessageRequest
	ID string `json:"id"` // Unique subscription ID
}

// UnsubscribeMessageResponse represents the response to an unsubscription request.
type UnsubscribeMessageResponse struct {
	BaseMessageResponse
	ID string `json:"id"` // Unique subscription ID
}
