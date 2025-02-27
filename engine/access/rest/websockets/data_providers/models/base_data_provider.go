package models

// BaseDataProvidersResponse represents a base structure for responses from subscriptions.
type BaseDataProvidersResponse struct {
	SubscriptionID string      `json:"subscription_id"` // Unique subscriptionID
	Topic          string      `json:"topic"`           // Topic of the subscription
	Payload        interface{} `json:"payload"`         // Payload that's being returned within a subscription.
}
