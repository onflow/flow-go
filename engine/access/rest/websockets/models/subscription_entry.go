package models

// SubscriptionEntry represents an active subscription entry.
type SubscriptionEntry struct {
	ID    string `json:"id"`    // ID is a client generated UUID for subscription
	Topic string `json:"topic"` // Topic of the subscription
	//TODO: maybe we should add arguments for readability?
}
