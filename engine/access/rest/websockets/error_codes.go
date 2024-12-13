package websockets

type Code int

const (
	Ok Code = iota
	ConnectionRead
	ConnectionWrite
	InvalidMessage
	NotFound
	InvalidArgument
	RunError
	InternalError
)
