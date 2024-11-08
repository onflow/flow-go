package websockets

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

// UnsubscribeMessageRequest represents a request to unsubscribe from a topic.
type UnsubscribeMessageRequest struct {
	BaseMessageRequest
	Topic string `json:"topic"` // Topic to unsubscribe from
	ID    string `json:"id"`    // Unique subscription ID
}

// UnsubscribeMessageResponse represents the response to an unsubscription request.
type UnsubscribeMessageResponse struct {
	BaseMessageResponse
	Topic string `json:"topic"` // Topic of the unsubscription
	ID    string `json:"id"`    // Unique subscription ID
}

// ListSubscriptionsMessageRequest represents a request to list active subscriptions.
type ListSubscriptionsMessageRequest struct {
	BaseMessageRequest
}

// SubscriptionEntry represents an active subscription entry.
type SubscriptionEntry struct {
	Topic string `json:"topic,omitempty"` // Topic of the subscription
	ID    string `json:"id,omitempty"`    // Unique subscription ID
}

// ListSubscriptionsMessageResponse is the structure used to respond to list_subscriptions requests.
// It contains a list of active subscriptions for the current WebSocket connection.
type ListSubscriptionsMessageResponse struct {
	BaseMessageResponse
	Subscriptions []*SubscriptionEntry `json:"subscriptions,omitempty"`
}
