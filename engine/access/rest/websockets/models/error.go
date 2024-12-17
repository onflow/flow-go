package models

type ErrorMessage struct {
	Code           int    `json:"code"`
	Message        string `json:"message"`
	Action         string `json:"action,omitempty"`
	SubscriptionID string `json:"subscription_id,omitempty"`
}
