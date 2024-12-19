package websockets

type Code int

const (
	InvalidMessage Code = iota
	InvalidArgument
	NotFound
	SubscriptionError
)
