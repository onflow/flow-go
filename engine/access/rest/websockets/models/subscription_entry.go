package models

// SubscriptionEntry represents an active subscription entry.
type SubscriptionEntry struct {
	SubscriptionID string    `json:"subscription_id"` // ID is a client generated UUID for subscription
	Topic          string    `json:"topic"`     // Topic of the subscription
	Arguments      Arguments `json:"arguments"`
}
