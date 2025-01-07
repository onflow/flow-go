package websockets

type Code int

const (
	InvalidMessage      Code = 400
	NotFound            Code = 404
	InternalServerError Code = 500
)
