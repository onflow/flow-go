// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package xdr

import (
	"io"

	xdr "github.com/davecgh/go-xdr/xdr2"
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/pkg/model/flow"
	"github.com/dapperlabs/flow-go/pkg/model/hotstuff"
	"github.com/dapperlabs/flow-go/pkg/model/message"
)

func decode(r io.Reader) (interface{}, error) {

	// decode the code
	var code uint8
	_, err := xdr.Unmarshal(r, &code)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode code")
	}

	// create the desired message
	var v interface{}
	switch code {
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
		return nil, errors.Errorf("invalid decode code (%d)", code)
	}

	// decode the value
	_, err = xdr.Unmarshal(r, &v)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode value")
	}

	return &v, nil
}
