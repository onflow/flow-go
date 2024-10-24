package subscription_handlers

import (
	"fmt"
)

const (
	EventsTopic              = "events"
	AccountStatusesTopic     = "account_statuses"
	BlocksTopic              = "blocks"
	BlockHeadersTopic        = "block_headers"
	BlockDigestsTopic        = "block_digests"
	TransactionStatusesTopic = "transaction_statuses"
)

type SubscriptionHandler interface {
	Close() error
}

func CreateSubscriptionHandler(topic string, arguments map[string]interface{}, broadcastMessage func([]byte) error) (SubscriptionHandler, error) {
	switch topic {
	// TODO: Implemented handlers for each topic should be added in respective case
	case EventsTopic,
		AccountStatusesTopic,
		BlocksTopic,
		BlockHeadersTopic,
		BlockDigestsTopic,
		TransactionStatusesTopic:
		return nil, fmt.Errorf("topic \"%s\" not implemented yet", topic)
	default:
		return nil, fmt.Errorf("unsupported topic \"%s\"", topic)
	}
}
