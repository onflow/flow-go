package models

// ListSubscriptionsMessageRequest represents a request to list active subscriptions.
type ListSubscriptionsMessageRequest struct {
	BaseMessageRequest
}

// ListSubscriptionsMessageResponse is the structure used to respond to list_subscriptions requests.
// It contains a list of active subscriptions for the current WebSocket connection.
type ListSubscriptionsMessageResponse struct {
	// Subscription list might be empty in case of no active subscriptions
	Subscriptions []*SubscriptionEntry `json:"subscriptions"`
	Action        string               `json:"action"`
}
