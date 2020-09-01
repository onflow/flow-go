package queue

import "fmt"

type QueueEmptyError struct {
	err error
}
func (e QueueEmptyError) Error() string {
	return fmt.Sprintf("queue empty")
}

type QueueFullError struct {
	err error
}
func (e QueueFullError) Error() string {
	return fmt.Sprintf("queue full")
}
