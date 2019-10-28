// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package capnp

import (
	"github.com/pkg/errors"
	capnp "zombiezen.com/go/capnproto2"

	"github.com/dapperlabs/flow-go/pkg/model/collection"
	"github.com/dapperlabs/flow-go/pkg/model/consensus"
	"github.com/dapperlabs/flow-go/pkg/model/trickle"
	"github.com/dapperlabs/flow-go/schema/captain"
)

func encode(vv interface{}) (*capnp.Message, error) {

	// create capnproto message & segment
	m, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize message")
	}

	// initialize the root message
	msg, err := captain.NewRootMessage(seg)
	if err != nil {
		return nil, errors.Wrap(err, "could not create root")
	}

	switch v := vv.(type) {

	// trickle overlay network
	case *trickle.Auth:
		err = encodeRootAuth(msg, v)
	case *trickle.Ping:
		err = encodeRootPing(msg, v)
	case *trickle.Pong:
		err = encodeRootPong(msg, v)
	case *trickle.Announce:
		err = encodeRootAnnounce(msg, v)
	case *trickle.Request:
		err = encodeRootRequest(msg, v)
	case *trickle.Response:
		err = encodeRootGossip(msg, v)

		// collection - collection forwarding
	case *collection.GuaranteedCollection:
		err = encodeRootGuaranteedCollection(msg, v)

	// consensus - collection propagation
	case *consensus.SnapshotRequest:
		err = encodeRootSnapshotRequest(msg, v)
	case *consensus.SnapshotResponse:
		err = encodeRootSnapshotResponse(msg, v)
	case *consensus.MempoolRequest:
		err = encodeRootMempoolRequest(msg, v)
	case *consensus.MempoolResponse:
		err = encodeRootMempoolResponse(msg, v)

	default:
		err = errors.Errorf("invalid encode type (%T)", vv)
	}
	if err != nil {
		return nil, errors.Wrap(err, "could not encode value")
	}

	return m, nil
}

// network overlay layer messages

func encodeRootAuth(msg captain.Message, v *trickle.Auth) error {
	auth, err := msg.NewAuth()
	if err != nil {
		return errors.Wrap(err, "could not create auth")
	}
	return encodeAuth(auth, v)
}

func encodeAuth(auth captain.Auth, v *trickle.Auth) error {
	err := auth.SetNodeId(v.NodeID)
	if err != nil {
		return errors.Wrap(err, "could not set node id")
	}
	return nil
}

func encodeRootPing(msg captain.Message, v *trickle.Ping) error {
	ping, err := msg.NewPing()
	if err != nil {
		return errors.Wrap(err, "could not create ping")
	}
	return encodePing(ping, v)
}

func encodePing(ping captain.Ping, v *trickle.Ping) error {
	ping.SetNonce(v.Nonce)
	return nil
}

func encodeRootPong(msg captain.Message, v *trickle.Pong) error {
	pong, err := msg.NewPong()
	if err != nil {
		return errors.Wrap(err, "could not create pong")
	}
	return encodePong(pong, v)
}

func encodePong(pong captain.Pong, v *trickle.Pong) error {
	pong.SetNonce(v.Nonce)
	return nil
}

func encodeRootAnnounce(msg captain.Message, v *trickle.Announce) error {
	ann, err := msg.NewAnnounce()
	if err != nil {
		return errors.Wrap(err, "could not create announce")
	}
	return encodeAnnounce(ann, v)
}

func encodeAnnounce(ann captain.Announce, v *trickle.Announce) error {
	ann.SetEngineId(v.EngineID)
	err := ann.SetEventId(v.EventID)
	if err != nil {
		return errors.Wrap(err, "could not set event id")
	}
	return nil
}

func encodeRootRequest(msg captain.Message, v *trickle.Request) error {
	req, err := msg.NewRequest()
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	return encodeRequest(req, v)
}

func encodeRequest(req captain.Request, v *trickle.Request) error {
	req.SetEngineId(v.EngineID)
	err := req.SetEventId(v.EventID)
	if err != nil {
		return errors.Wrap(err, "could not set event id")
	}
	return nil
}

func encodeRootGossip(msg captain.Message, v *trickle.Response) error {
	response, err := msg.NewResponse()
	if err != nil {
		return errors.Wrap(err, "could not create response")
	}
	return encodeGossip(response, v)
}

func encodeGossip(response captain.Response, v *trickle.Response) error {
	response.SetEngineId(v.EngineID)
	err := response.SetEventId(v.EventID)
	if err != nil {
		return errors.Wrap(err, "could not set event ID")
	}
	err = response.SetOriginId(v.OriginID)
	if err != nil {
		return errors.Wrap(err, "could not set origin id")
	}
	targetIDs, err := response.NewTargetIds(int32(len(v.TargetIDs)))
	if err != nil {
		return errors.Wrap(err, "could not create target id list")
	}
	for i, vv := range v.TargetIDs {
		err = targetIDs.Set(i, vv)
		if err != nil {
			return errors.Wrapf(err, "could not set target id (%d)", i)
		}
	}
	err = response.SetPayload(v.Payload)
	if err != nil {
		return errors.Wrap(err, "could not set payload")
	}
	return nil
}

// collection - collection forwarding

func encodeRootGuaranteedCollection(msg captain.Message, v *collection.GuaranteedCollection) error {
	coll, err := msg.NewGuaranteedCollection()
	if err != nil {
		return errors.Wrap(err, "could not create guaranteed collection")
	}
	return encodeGuaranteedCollection(coll, v)
}

func encodeGuaranteedCollection(coll captain.GuaranteedCollection, v *collection.GuaranteedCollection) error {
	err := coll.SetHash(v.Hash)
	if err != nil {
		return errors.Wrap(err, "could not set hash")
	}
	err = coll.SetSignature(v.Signature)
	if err != nil {
		return errors.Wrap(err, "could not set signature")
	}
	return nil
}

// consensus - collection propagation

func encodeRootSnapshotRequest(msg captain.Message, v *consensus.SnapshotRequest) error {
	req, err := msg.NewSnapshotRequest()
	if err != nil {
		return errors.Wrap(err, "could not create snapshot request")
	}
	return encodeSnapshotRequest(req, v)
}

func encodeSnapshotRequest(req captain.SnapshotRequest, v *consensus.SnapshotRequest) error {
	req.SetNonce(v.Nonce)
	err := req.SetMempoolHash(v.MempoolHash)
	if err != nil {
		return errors.Wrap(err, "could not set mempool hash")
	}
	return nil
}

func encodeRootSnapshotResponse(msg captain.Message, v *consensus.SnapshotResponse) error {
	req, err := msg.NewSnapshotResponse()
	if err != nil {
		return errors.Wrap(err, "could not create snapshot response")
	}
	return encodeSnapshotResponse(req, v)
}

func encodeSnapshotResponse(res captain.SnapshotResponse, v *consensus.SnapshotResponse) error {
	res.SetNonce(v.Nonce)
	err := res.SetMempoolHash(v.MempoolHash)
	if err != nil {
		return errors.Wrap(err, "could not set mempool hash")
	}
	return nil
}

func encodeRootMempoolRequest(msg captain.Message, v *consensus.MempoolRequest) error {
	req, err := msg.NewMempoolRequest()
	if err != nil {
		return errors.Wrap(err, "could not create mempool request")
	}
	return encodeMempoolRequest(req, v)
}

func encodeMempoolRequest(req captain.MempoolRequest, v *consensus.MempoolRequest) error {
	req.SetNonce(v.Nonce)
	return nil
}

func encodeRootMempoolResponse(msg captain.Message, v *consensus.MempoolResponse) error {
	res, err := msg.NewMempoolResponse()
	if err != nil {
		return errors.Wrap(err, "could not create mempool response")
	}
	return encodeMempoolResponse(res, v)
}

func encodeMempoolResponse(res captain.MempoolResponse, v *consensus.MempoolResponse) error {
	res.SetNonce(v.Nonce)
	fingerprints, err := res.NewCollections(int32(len(v.Collections)))
	if err != nil {
		return errors.Wrap(err, "could not create guaranteed collection list")
	}
	for i, vv := range v.Collections {
		fp, err := captain.NewGuaranteedCollection(res.Segment())
		if err != nil {
			return errors.Wrapf(err, "could not create guaranteed collection (%d)", i)
		}
		err = encodeGuaranteedCollection(fp, vv)
		if err != nil {
			return errors.Wrapf(err, "could not encode guaranteed collection (%d)", i)
		}
		err = fingerprints.Set(i, fp)
		if err != nil {
			return errors.Wrapf(err, "could not set guaranteed collection (%d)", i)
		}
	}
	return nil
}
