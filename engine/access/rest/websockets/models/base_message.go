package models

const (
	SubscribeAction         = "subscribe"
	UnsubscribeAction       = "unsubscribe"
	ListSubscriptionsAction = "list_subscription"
)

// BaseMessageRequest represents a base structure for incoming messages.
type BaseMessageRequest struct {
	Action          string `json:"action"`     // subscribe, unsubscribe or list_subscriptions
	ClientMessageID string `json:"message_id"` // ClientMessageID is a uuid generated by client to identify request/response uniquely
}

// BaseMessageResponse represents a base structure for outgoing messages.
type BaseMessageResponse struct {
	SubscriptionID  string       `json:"subscription_id"`
	ClientMessageID string       `json:"message_id,omitempty"` // ClientMessageID may be empty in case we send msg by ourselves (e.g. error occurred)
	Success         bool         `json:"success"`
	Error           ErrorMessage `json:"error,omitempty"`
}
