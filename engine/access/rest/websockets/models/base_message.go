package models

// BaseMessageRequest represents a base structure for incoming messages.
type BaseMessageRequest struct {
	Action string `json:"action"` // Action type of the request
}

// BaseMessageResponse represents a base structure for outgoing messages.
type BaseMessageResponse struct {
	Action       string `json:"action,omitempty"`        // Action type of the response
	Success      bool   `json:"success"`                 // Indicates success or failure
	ErrorMessage string `json:"error_message,omitempty"` // Error message, if any
}

// BaseDataProvidersResponse represents a base structure for responses from subscriptions.
type BaseDataProvidersResponse struct {
	ID    string      `json:"id"`    // Unique subscription ID
	Topic string      `json:"topic"` // Topic of the subscription
	Data  interface{} `json:"data"`  // Data that's being returned within a subscription.
}
