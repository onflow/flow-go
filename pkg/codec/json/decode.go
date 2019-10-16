// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/pkg/model/flow"
	"github.com/dapperlabs/flow-go/pkg/model/hotstuff"
	"github.com/dapperlabs/flow-go/pkg/model/message"
)

// decode will decode the envelope into an entity.
func decode(env Envelope) (interface{}, error) {

	// create the desired message
	var v interface{}
	switch env.Code {
	case CodePing:
		v = message.Ping{}
	case CodePong:
		v = message.Pong{}
	case CodeAuth:
		v = message.Auth{}
	case CodeAnnounce:
		v = message.Announce{}
	case CodeRequest:
		v = message.Request{}
	case CodeEvent:
		v = message.Event{}

	case CodeCollection:
		v = flow.Collection{}
	case CodeReceipt:
		v = flow.Receipt{}
	case CodeApproval:
		v = flow.Approval{}
	case CodeSeal:
		v = flow.Seal{}

	case CodeBlock:
		v = hotstuff.Block{}
	case CodeVote:
		v = hotstuff.Vote{}
	case CodeTimeout:
		v = hotstuff.Timeout{}

	default:
		return nil, errors.Errorf("invalid message code (%d)", env.Code)
	}

	// unmarshal the payload
	err := json.Unmarshal(env.Data, &v)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode payload")
	}

	return &v, nil
}
