package network

// MessageQueue is the interface of the inbound message queue
type MessageQueue interface {
	// Insert inserts the message in queue
	Insert(message interface{}) error
	// Remove removes the message from the queue in priority order. If no message is found, this call blocks.
	// If two messages have the same priority, items are de-queued in insertion order
	Remove() interface{}
	// Len gives the current length of the queue
	Len() int
}
