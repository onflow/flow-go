// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/model/messages"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/libp2p/message"
	"github.com/dapperlabs/flow-go/model/trickle"
)

func encode(v interface{}) (*Envelope, error) {

	// determine the message type
	var code uint8
	switch v.(type) {

	case *trickle.Ping:
		code = CodePing
	case *trickle.Pong:
		code = CodePong
	case *trickle.Auth:
		code = CodeAuth
	case *trickle.Announce:
		code = CodeAnnounce
	case *trickle.Request:
		code = CodeRequest
	case *trickle.Response:
		code = CodeResponse
	case *message.Echo:
		code = CodeEcho

	case *flow.CollectionGuarantee:
		code = CodeCollectionGuarantee
	case *flow.Transaction:
		code = CodeTransaction

	case *flow.Block:
		code = CodeBlock

	case *messages.CollectionRequest:
		code = CodeCollectionRequest
	case *messages.CollectionResponse:
		code = CodeCollectionResponse

	case *flow.ExecutionReceipt:
		code = CodeExecutionRecipt
	case *messages.ExecutionStateRequest:
		code = CodeExecutionStateRequest
	case *messages.ExecutionStateResponse:
		code = CodeExecutionStateResponse

	case *execution.CompleteBlock:
		code = CodeExecutionCompleteBlock
	case *execution.ComputationOrder:
		code = CodeExecutionExecutionOrder

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
