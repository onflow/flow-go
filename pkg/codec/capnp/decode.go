// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package capnp

import (
	"github.com/dapperlabs/flow-go/pkg/model/flow"
	"github.com/dapperlabs/flow-go/pkg/model/hotstuff"
	"github.com/dapperlabs/flow-go/pkg/model/message"
	"github.com/pkg/errors"
	capnp "zombiezen.com/go/capnproto2"
)

func decode(msg *capnp.Message) (interface{}, error) {

	// read into root type
	z, err := ReadRootZ(msg)
	if err != nil {
		return nil, errors.Wrap(err, "could not read root")
	}

	// retrieve the embedded type
	var v interface{}
	switch z.Which() {
	case Z_Which_ping:
		v, err = decodePing(z)
	case Z_Which_pong:
		v, err = decodePong(z)
	case Z_Which_auth:
		v, err = decodeAuth(z)
	case Z_Which_announce:
		v, err = decodeAnnounce(z)
	case Z_Which_request:
		v, err = decodeRequest(z)
	case Z_Which_event:
		v, err = decodeEvent(z)

	case Z_Which_collection:
		v, err = decodeCollection(z)
	case Z_Which_receipt:
		v, err = decodeReceipt(z)
	case Z_Which_approval:
		v, err = decodeApproval(z)
	case Z_Which_seal:
		v, err = decodeSeal(z)

	case Z_Which_block:
		v, err = decodeBlock(z)
	case Z_Which_vote:
		v, err = decodeVote(z)
	case Z_Which_timeout:
		v, err = decodeTimeout(z)

	default:
		err = errors.Errorf("invalid decode code (%d)", z.Which())
	}
	if err != nil {
		return nil, errors.Wrap(err, "could not decode value")
	}

	return v, nil
}

func decodePing(z Z) (*message.Ping, error) {
	ping, err := z.Ping()
	if err != nil {
		return nil, errors.Wrap(err, "could not read ping")
	}
	v := &message.Ping{
		Nonce: ping.Nonce(),
	}
	return v, nil
}

func decodePong(z Z) (*message.Pong, error) {
	pong, err := z.Pong()
	if err != nil {
		return nil, errors.Wrap(err, "could not read pong")
	}
	v := &message.Pong{
		Nonce: pong.Nonce(),
	}
	return v, nil
}

func decodeAuth(z Z) (*message.Auth, error) {
	auth, err := z.Auth()
	if err != nil {
		return nil, errors.Wrap(err, "could not read auth")
	}
	node, err := auth.Node()
	if err != nil {
		return nil, errors.Wrap(err, "could not read id")
	}
	v := &message.Auth{
		Node: node,
	}
	return v, nil
}

func decodeAnnounce(z Z) (*message.Announce, error) {
	announce, err := z.Announce()
	if err != nil {
		return nil, errors.Wrap(err, "could not read announce")
	}
	id, err := announce.Id()
	if err != nil {
		return nil, errors.Wrap(err, "could not read hash")
	}
	v := &message.Announce{
		Engine: announce.Engine(),
		ID:     id,
	}
	return v, nil
}

func decodeRequest(z Z) (*message.Request, error) {
	request, err := z.Request()
	if err != nil {
		return nil, errors.Wrap(err, "could not read request")
	}
	id, err := request.Id()
	if err != nil {
		return nil, errors.Wrap(err, "could not read hash")
	}
	v := &message.Request{
		Engine: request.Engine(),
		ID:     id,
	}
	return v, nil
}

func decodeEvent(z Z) (*message.Event, error) {
	event, err := z.Event()
	if err != nil {
		return nil, errors.Wrap(err, "could not read event")
	}
	payload, err := event.Payload()
	if err != nil {
		return nil, errors.Wrap(err, "could not read payload")
	}
	v := &message.Event{
		Engine:  event.Engine(),
		Payload: payload,
	}
	return v, nil
}

func decodeCollection(z Z) (*flow.Collection, error) {
	collection, err := z.Collection()
	if err != nil {
		return nil, errors.Wrap(err, "could not read collection")
	}
	_ = collection
	v := &flow.Collection{}
	return v, nil
}

func decodeReceipt(z Z) (*flow.Receipt, error) {
	receipt, err := z.Receipt()
	if err != nil {
		return nil, errors.Wrap(err, "could not read receipt")
	}
	_ = receipt
	v := &flow.Receipt{}
	return v, nil
}

func decodeApproval(z Z) (*flow.Approval, error) {
	approval, err := z.Approval()
	if err != nil {
		return nil, errors.Wrap(err, "could not read approval")
	}
	_ = approval
	v := &flow.Approval{}
	return v, nil
}

func decodeSeal(z Z) (*flow.Seal, error) {
	seal, err := z.Seal()
	if err != nil {
		return nil, errors.Wrap(err, "could not read seal")
	}
	_ = seal
	v := &flow.Seal{}
	return v, nil
}

func decodeBlock(z Z) (*hotstuff.Block, error) {
	block, err := z.Block()
	if err != nil {
		return nil, errors.Wrap(err, "could not read block")
	}
	_ = block
	v := &hotstuff.Block{}
	return v, nil
}

func decodeVote(z Z) (*hotstuff.Vote, error) {
	vote, err := z.Vote()
	if err != nil {
		return nil, errors.Wrap(err, "could not read vote")
	}
	_ = vote
	v := &hotstuff.Vote{}
	return v, nil
}

func decodeTimeout(z Z) (*hotstuff.Timeout, error) {
	timeout, err := z.Timeout()
	if err != nil {
		return nil, errors.Wrap(err, "could not read timeout")
	}
	_ = timeout
	v := &hotstuff.Timeout{}
	return v, nil
}
