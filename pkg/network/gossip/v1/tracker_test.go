package gnode

import (
	"errors"
	"fmt"
	"testing"
)

var errSome = errors.New("A mock error for testing purpose")

// TestTrackMessage tests the tracker with various possible scenarios
func TestTrackMessage(t *testing.T) {
	m := &messageTracker{}
	// assume a message with uuid "abc" is being tracked
	(*m)["abc"] = &returnParams{done: make(chan bool, 1)}

	// tt is the table containing various test cases
	tt := []struct {
		messageUUID string
		outputError error
	}{
		{ // test case when an unregistered message is added
			messageUUID: "cba",
			outputError: nil,
		},
		{ // test case when an already tracked message is called to be tracked again
			messageUUID: "abc",
			outputError: fmt.Errorf("cannot track message %v: Message already being tracked", "abc"),
		},
	}

	for _, tc := range tt {
		err := m.TrackMessage(tc.messageUUID)
		if tc.outputError == nil && err != nil {
			t.Errorf("TrackMessage: Expected %v, Got %v", tc.outputError, err)
		}
		if tc.outputError != nil && (err == nil || err.Error() != tc.outputError.Error()) {
			t.Errorf("TrackMessage: Expected %v, Got %v", tc.outputError, err)
		}
	}
}

// TestFillMessageReply tests message reply with various scenarios
func TestFillMessageReply(t *testing.T) {
	m := &messageTracker{}
	// assume a message with uuid "abc" is being tracked
	(*m)["abc"] = &returnParams{done: make(chan bool, 1)}

	// tt is the table containing various test cases
	tt := []struct {
		messageUUID string
		reply       []byte
		err         error
		outputError error
	}{
		{ // The case where an already tracked message is filled
			messageUUID: "abc",
			reply:       []byte("message"),
			err:         errSome,
			outputError: nil,
		},
		{ // The case where an unregistered message is filled
			messageUUID: "cba",
			reply:       []byte("message"),
			err:         errSome,
			outputError: fmt.Errorf("cannot fill message %v: No tracking record registered for this message", "cba"),
		},
	}

	for _, tc := range tt {
		err := m.FillMessageReply(tc.messageUUID, tc.reply, tc.err)

		if tc.outputError == nil && err != nil {
			t.Errorf("FillMessageReply: Expected %v, Got %v", tc.messageUUID, err)
		}
		if tc.outputError != nil && (err == nil || err.Error() != tc.outputError.Error()) {
			t.Errorf("FillMessageReply: Expected %v, Got %v", tc.messageUUID, err)
		}
	}
}

// TestingDone tests the message tracker in terms of the value of the channel returned
func TestDone(t *testing.T) {
	m := &messageTracker{}
	// assume a message with uuid "abc" is being tracked
	(*m)["abc"] = &returnParams{done: make(chan bool, 1)}

	// tt is the table containing various test cases
	tt := []struct {
		messageUUID   string
		outputChannel chan bool
	}{
		{ // The case where Done() is called for a tracked message
			messageUUID:   "abc",
			outputChannel: make(chan bool, 1),
		},
		{ // The case where Done() is called for an unregistered message
			messageUUID:   "cba",
			outputChannel: nil,
		},
	}

	for _, tc := range tt {
		done := m.Done(tc.messageUUID)
		if tc.outputChannel == nil && done != nil {
			t.Errorf("Done: Expected %v, Got %v", tc.outputChannel, done)
		}
		if tc.outputChannel != nil && done == nil {
			t.Errorf("Done: Expected %v, Got %v", tc.outputChannel, done)
		}
	}
}

// TestGetReply tests GetReply with various inputs
func TestGetReply(t *testing.T) {
	m := &messageTracker{}
	// assume a message with uuid "abc" is being tracked
	(*m)["abc"] = &returnParams{done: make(chan bool, 1)}
	(*m)["tst"] = &returnParams{
		reply: []byte("message"),
		err:   nil,
		done:  make(chan bool, 1),
	}
	// tt is the table containing various test cases
	tt := []struct {
		messageUUID   string
		reply         []byte
		err           error
		expectedError error
	}{
		{ // The case of a tracked but not filled message
			messageUUID:   "abc",
			reply:         nil,
			err:           nil,
			expectedError: nil,
		},
		{ // The case of an registered message
			messageUUID:   "cba",
			reply:         nil,
			err:           nil,
			expectedError: fmt.Errorf("no valid registered tracker for message %v", "cba"),
		},
		{ // The case of a tracked and filled message
			messageUUID:   "tst",
			reply:         []byte("message"),
			err:           nil,
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		reply, err, outputError := m.GetReply(tc.messageUUID)

		// when the filled reply is not the expected reply
		if string(reply) != string(tc.reply) {
			t.Errorf("GetReply: Expected %v, Got: %v",  tc.reply, reply)
		}
		// when the filled error is expected to be nil but the returned value is not nil
		if tc.err == nil && err != nil {
			t.Errorf("GetReply: Expected %v, Got: %v", tc.err, err)
		}
		// when the filled error is expected not to be nil, but the returned value is either nil or not equal to the expected value
		if tc.err != nil && (err == nil || err.Error() != tc.err.Error()) {
			t.Errorf("GetReply: Expected %v, Got: %v", tc.err, err)
		}
		// when the output error is expected to be nil but the returned value is not nil
		if tc.expectedError == nil && outputError != nil {
			t.Errorf("GetReply: Expected %v, Got: %v", tc.expectedError, outputError)
		}
		// when the output error is expected not to be nil, but the returned value is either nil or not equal to the expected value
		if tc.expectedError != nil && (outputError == nil || outputError.Error() != tc.expectedError.Error()) {
			t.Errorf("GetReply: Expected %v, Got: %v", tc.expectedError, outputError)
		}
	}
}

// TestContainsID tests ContainID by a simple test case
func TestContainsID(t *testing.T) {
	m := &messageTracker{}
	// assume a message with uuid "abc" is being tracked
	(*m)["abc"] = &returnParams{}

	// tt is the table containing various test cases
	tt := []struct {
		uuid   string
		output bool
	}{
		{ // The case where it should return true
			uuid:   "abc",
			output: true,
		},
		{ // The case where it should return false
			uuid:   "cba",
			output: false,
		},
	}
	for _, tc := range tt {
		res := m.ContainsID(tc.uuid)
		if res != tc.output {
			t.Errorf("ContainID: Expected %v, Got: %v", tc.output, res)
		}
	}
}
