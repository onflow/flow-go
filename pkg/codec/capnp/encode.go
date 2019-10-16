// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package capnp

import (
	"github.com/pkg/errors"
	capnp "zombiezen.com/go/capnproto2"

	"github.com/dapperlabs/flow-go/pkg/model/flow"
	"github.com/dapperlabs/flow-go/pkg/model/hotstuff"
	"github.com/dapperlabs/flow-go/pkg/model/message"
)

func encode(v interface{}) (*capnp.Message, error) {

	// create capnproto message & segment
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize message")
	}

	// initialize the root message
	z, err := NewRootZ(seg)
	if err != nil {
		return nil, errors.Wrap(err, "could not create root")
	}

	// initialize the sub-message
	switch vv := v.(type) {
	case *message.Ping:
		err = encodePing(z, vv)
	case *message.Pong:
		err = encodePong(z, vv)
	case *message.Auth:
		err = encodeAuth(z, vv)
	case *message.Announce:
		err = encodeAnnounce(z, vv)
	case *message.Request:
		err = encodeRequest(z, vv)
	case *message.Event:
		err = encodeEvent(z, vv)

	case *flow.Collection:
		err = encodeCollection(z, vv)
	case *flow.Receipt:
		err = encodeReceipt(z, vv)
	case *flow.Approval:
		err = encodeApproval(z, vv)
	case *flow.Seal:
		err = encodeSeal(z, vv)

	case *hotstuff.Block:
		err = encodeBlock(z, vv)
	case *hotstuff.Vote:
		err = encodeVote(z, vv)
	case *hotstuff.Timeout:
		err = encodeTimeout(z, vv)

	default:
		err = errors.Errorf("invalid encode type (%T)", v)
	}
	if err != nil {
		return nil, errors.Wrap(err, "could not encode value")
	}

	return msg, nil
}

func encodePing(z Z, vv *message.Ping) error {
	ping, err := z.NewPing()
	if err != nil {
		return errors.Wrap(err, "could not create ping")
	}
	ping.SetNonce(vv.Nonce)
	return nil
}

func encodePong(z Z, vv *message.Pong) error {
	pong, err := z.NewPong()
	if err != nil {
		return errors.Wrap(err, "could not create pong")
	}
	pong.SetNonce(vv.Nonce)
	return nil
}

func encodeAuth(z Z, vv *message.Auth) error {
	auth, err := z.NewAuth()
	if err != nil {
		return errors.Wrap(err, "could not create auth")
	}
	err = auth.SetNode(vv.Node)
	if err != nil {
		return errors.Wrap(err, "could not set id")
	}
	return nil
}

func encodeAnnounce(z Z, vv *message.Announce) error {
	announce, err := z.NewAnnounce()
	if err != nil {
		return errors.Wrap(err, "could not create announce")
	}
	err = announce.SetId(vv.ID)
	if err != nil {
		return errors.Wrap(err, "could not set hash")
	}
	return nil
}

func encodeRequest(z Z, vv *message.Request) error {
	request, err := z.NewRequest()
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	err = request.SetId(vv.ID)
	if err != nil {
		return errors.Wrap(err, "could not set hash")
	}
	return nil
}

func encodeEvent(z Z, vv *message.Event) error {
	event, err := z.NewEvent()
	if err != nil {
		return errors.Wrap(err, "could not create event")
	}
	event.SetEngine(vv.Engine)
	err = event.SetPayload(vv.Payload)
	if err != nil {
		return errors.Wrap(err, "could not set bin")
	}
	return nil
}

func encodeCollection(z Z, vv *flow.Collection) error {
	collection, err := z.NewCollection()
	if err != nil {
		return errors.Wrap(err, "could not create collection")
	}
	_ = collection
	return nil
}

func encodeReceipt(z Z, vv *flow.Receipt) error {
	receipt, err := z.NewReceipt()
	if err != nil {
		return errors.Wrap(err, "could not create receipt")
	}
	_ = receipt
	return nil
}

func encodeApproval(z Z, vv *flow.Approval) error {
	approval, err := z.NewApproval()
	if err != nil {
		return errors.Wrap(err, "could not create approval")
	}
	_ = approval
	return nil
}

func encodeSeal(z Z, vv *flow.Seal) error {
	seal, err := z.NewSeal()
	if err != nil {
		return errors.Wrap(err, "could not create seal")
	}
	_ = seal
	return nil
}

func encodeBlock(z Z, vv *hotstuff.Block) error {
	block, err := z.NewBlock()
	if err != nil {
		return errors.Wrap(err, "could not create block")
	}
	_ = block
	return nil
}

func encodeVote(z Z, vv *hotstuff.Vote) error {
	vote, err := z.NewVote()
	if err != nil {
		return errors.Wrap(err, "could not create vote")
	}
	_ = vote
	return nil
}

func encodeTimeout(z Z, vv *hotstuff.Timeout) error {
	timeout, err := z.NewTimeout()
	if err != nil {
		return errors.Wrap(err, "could not create timeout")
	}
	_ = timeout
	return nil
}
