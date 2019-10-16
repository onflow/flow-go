package gnode

//tracker encloses the messageTracker type, which tracks the messages that are gossiped.

import (
	"fmt"
)

// returnParams are the return parameters associated with a functions invocation in the node registry
type returnParams struct {
	err   error
	reply []byte
	done  chan bool
}

// messageTracker tracks GossipMessage sent via SyncQueue, so we can return
// back the results to the caller
type messageTracker map[string]*returnParams

// TrackMessage assigns the uuid of a gossip message to get tracked.
func (m *messageTracker) TrackMessage(messageUUID string) error {
	if m.ContainsID(messageUUID) {
		return fmt.Errorf("cannot track message %v: Message already being tracked", messageUUID)
	}
	(*m)[messageUUID] = &returnParams{done: make(chan bool, 1)}
	return nil
}

// FillMessageReply fills the content of the reply of a gossip message
func (m *messageTracker) FillMessageReply(messageUUID string, reply []byte, err error) error {
	if !m.ContainsID(messageUUID) {
		return fmt.Errorf("cannot fill message %v: No tracking record registered for this message", messageUUID)
	}
	(*m)[messageUUID].err = err
	(*m)[messageUUID].reply = reply
	(*m)[messageUUID].done <- true
	return nil
}

// Done returns a channel. The channel is used to track the return values of a gossip message.
func (m *messageTracker) Done(messageUUID string) chan bool {
	if !m.ContainsID(messageUUID) {
		return nil
	}
	return (*m)[messageUUID].done
}

// GetReply returns the return results of a gossip message and then deletes the entry
func (m *messageTracker) GetReply(messageUUID string) ([]byte, error, error) {
	if !m.ContainsID(messageUUID) {
		return nil, nil, fmt.Errorf("no valid registered tracker for message %v", messageUUID)
	}

	reply := (*m)[messageUUID].reply
	err := (*m)[messageUUID].err

	//deleting message from tracker
	delete(*m, messageUUID)
	return reply, err, nil
}

// ContainsID checks whether the given uuid is being tracked or not
func (m *messageTracker) ContainsID(uuid string) bool {
	if _, ok := (*m)[uuid]; ok {
		return true
	}
	return false
}
