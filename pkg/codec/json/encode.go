// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/pkg/model/flow"
	"github.com/dapperlabs/flow-go/pkg/model/hotstuff"
	"github.com/dapperlabs/flow-go/pkg/model/message"
)

func encode(v interface{}) (*Envelope, error) {

	// determine the message type
	var code uint8
	switch v.(type) {
	case *message.Ping:
		code = CodePing
	case *message.Pong:
		code = CodePong
	case *message.Auth:
		code = CodeAuth
	case *message.Announce:
		code = CodeAnnounce
	case *message.Request:
		code = CodeRequest
	case *message.Event:
		code = CodeEvent

	case *flow.Collection:
		code = CodeCollection
	case *flow.Receipt:
		code = CodeReceipt
	case *flow.Approval:
		code = CodeApproval
	case *flow.Seal:
		code = CodeSeal

	case *hotstuff.Block:
		code = CodeBlock
	case *hotstuff.Vote:
		code = CodeVote
	case *hotstuff.Timeout:
		code = CodeTimeout

	default:
		return nil, errors.Errorf("invalid encode type (%T)", v)
	}

	// encode the payload
	data, err := json.Marshal(v)
	if err != nil {
		return nil, errors.Wrap(err, "could not encode payload")
	}

	env := Envelope{
		Code: code,
		Data: data,
	}

	return &env, nil
}
