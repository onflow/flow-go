package gnode

import (
	"fmt"
)

// returnParams are the return parameters of functions in the node registry
type returnParams struct {
	err   error
	reply []byte

	done chan bool
}

// messageTracker tracks GossipMessage sent via SyncQueue, so we can return
// back the results to the caller
type messageTracker map[string]*returnParams

// TrackMessage assigns the uuid of a gossip message to get tracked
func (m *messageTracker) TrackMessage(uuid string) error {
	if m.ContainsID(uuid) {
		return fmt.Errorf("cannot track message %v: Message already being tracked", uuid)
	}

	(*m)[uuid] = &returnParams{done: make(chan bool, 1)}

	return nil
}

// FillMessageReply fills the contect of the reply of a gossip message
func (m *messageTracker) FillMessageReply(uuid string, reply []byte, err error) error {
	if !m.ContainsID(uuid) {
		return fmt.Errorf("cannot fill message %v: Message is not tracked", uuid)
	}

	(*m)[uuid].err = err
	(*m)[uuid].reply = reply
	(*m)[uuid].done <- true

	return nil
}

// Done returns a channel which will send a value once the return params of
// a gossip message get filled
func (m *messageTracker) Done(uuid string) chan bool {
	if !m.ContainsID(uuid) {
		// return fmt.Errorf("cannot wait for message %v: Message is not tracked", uuid)
		return nil
	}

	return (*m)[uuid].done
}

// RetrieveReply returns the reply params of a gossip message and then deletes the entry
func (m *messageTracker) RetrieveReply(uuid string) ([]byte, error, error) {
	if !m.ContainsID(uuid) {
		return nil, nil, fmt.Errorf("message %v is not tracked", uuid)
	}

	var (
		reply = (*m)[uuid].reply
		err   = (*m)[uuid].err
	)

	delete(*m, uuid)
	return reply, err, nil
}

// ContainsID checks weather the given uuid is being tracked or not
func (m *messageTracker) ContainsID(uuid string) bool {
	if _, ok := (*m)[uuid]; ok {
		return true
	}

	return false
}
