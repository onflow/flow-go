package models

// ListSubscriptionsMessageRequest represents a request to list active subscriptions.
type ListSubscriptionsMessageRequest struct {
	BaseMessageRequest
}

// ListSubscriptionsMessageResponse is the structure used to respond to list_subscriptions requests.
// It contains a list of active subscriptions for the current WebSocket connection.
type ListSubscriptionsMessageResponse struct {
	ClientMessageID string               `json:"message_id"`
	Success         bool                 `json:"success"`
	Error           ErrorMessage         `json:"error,omitempty"`
	Subscriptions   []*SubscriptionEntry `json:"subscriptions,omitempty"`
}
