package models

// UnsubscribeMessageRequest represents a request to unsubscribe from a topic.
type UnsubscribeMessageRequest struct {
	BaseMessageRequest
	//TODO: in this request, subscription_id is mandatory, but we inherit the optional one.
	// should we rewrite args to meet requirements?
}

// UnsubscribeMessageResponse represents the response to an unsubscription request.
type UnsubscribeMessageResponse struct {
	BaseMessageResponse
}
