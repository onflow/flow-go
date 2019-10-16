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

func encode(v interface{}, w io.Writer) error {

	// determine the code
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
		return errors.Errorf("invalid encode code (%T)", v)
	}

	// write the type
	_, err := xdr.Marshal(w, code)
	if err != nil {
		return errors.Wrap(err, "could not encode code")
	}

	// write the entity
	_, err = xdr.Marshal(w, v)
	if err != nil {
		return errors.Wrap(err, "could not encode value")
	}

	return nil
}
