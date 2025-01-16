package models

// SubscriptionEntry represents an active subscription entry.
type SubscriptionEntry struct {
	Topic     string    `json:"topic"`     // Topic of the subscription
	ID        string    `json:"id"`        // Unique subscription ID
	Arguments Arguments `json:"arguments"` // Arguments of the subscription
}
