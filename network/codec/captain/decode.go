// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package captain

import (
	"github.com/pkg/errors"
	capnp "zombiezen.com/go/capnproto2"

	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/model/consensus"
	"github.com/dapperlabs/flow-go/model/trickle"
	"github.com/dapperlabs/flow-go/schema/captain"
)

func decode(m *capnp.Message) (interface{}, error) {

	// read into root type
	msg, err := captain.ReadRootMessage(m)
	if err != nil {
		return nil, errors.Wrap(err, "could not read root")
	}

	var v interface{}
	switch msg.Which() {

	// trickle network overlay
	case captain.Message_Which_auth:
		v, err = decodeRootAuth(msg)
	case captain.Message_Which_ping:
		v, err = decodeRootPing(msg)
	case captain.Message_Which_pong:
		v, err = decodeRootPong(msg)
	case captain.Message_Which_announce:
		v, err = decodeRootAnnounce(msg)
	case captain.Message_Which_request:
		v, err = decodeRootRequest(msg)
	case captain.Message_Which_response:
		v, err = decodeRootResponse(msg)

		// collection - collection forwarding
	case captain.Message_Which_guaranteedCollection:
		v, err = decodeRootGuaranteedCollection(msg)

		// consensus - collection propagation
	case captain.Message_Which_snapshotRequest:
		v, err = decodeRootSnapshotRequest(msg)
	case captain.Message_Which_snapshotResponse:
		v, err = decodeRootSnapshotResponse(msg)
	case captain.Message_Which_mempoolRequest:
		v, err = decodeRootMempoolRequest(msg)
	case captain.Message_Which_mempoolResponse:
		v, err = decodeRootMempoolResponse(msg)

	default:
		err = errors.Errorf("invalid decode code (%d)", msg.Which())
	}
	if err != nil {
		return nil, errors.Wrap(err, "could not decode value")
	}

	return v, nil
}

// trickle network overlay

func decodeRootAuth(msg captain.Message) (*trickle.Auth, error) {
	auth, err := msg.Auth()
	if err != nil {
		return nil, errors.Wrap(err, "could not read auth")
	}
	return decodeAuth(auth)
}

func decodeAuth(auth captain.Auth) (*trickle.Auth, error) {
	nodeID, err := auth.NodeId()
	if err != nil {
		return nil, errors.Wrap(err, "could not get node id")
	}
	v := &trickle.Auth{
		NodeID: nodeID,
	}
	return v, nil
}

func decodeRootPing(msg captain.Message) (*trickle.Ping, error) {
	ping, err := msg.Ping()
	if err != nil {
		return nil, errors.Wrap(err, "could not read ping")
	}
	return decodePing(ping)
}

func decodePing(ping captain.Ping) (*trickle.Ping, error) {
	nonce := ping.Nonce()
	v := &trickle.Ping{
		Nonce: nonce,
	}
	return v, nil
}

func decodeRootPong(msg captain.Message) (*trickle.Pong, error) {
	pong, err := msg.Pong()
	if err != nil {
		return nil, errors.Wrap(err, "could not read pong")
	}
	return decodePong(pong)
}

func decodePong(pong captain.Pong) (*trickle.Pong, error) {
	nonce := pong.Nonce()
	v := &trickle.Pong{
		Nonce: nonce,
	}
	return v, nil
}

func decodeRootAnnounce(msg captain.Message) (*trickle.Announce, error) {
	ann, err := msg.Announce()
	if err != nil {
		return nil, errors.Wrap(err, "could not read announce")
	}
	return decodeAnnounce(ann)
}

func decodeAnnounce(ann captain.Announce) (*trickle.Announce, error) {
	engineID := ann.EngineId()
	eventID, err := ann.EventId()
	if err != nil {
		return nil, errors.Wrap(err, "could not get event id")
	}
	v := &trickle.Announce{
		EngineID: engineID,
		EventID:  eventID,
	}
	return v, nil
}

func decodeRootRequest(msg captain.Message) (*trickle.Request, error) {
	req, err := msg.Request()
	if err != nil {
		return nil, errors.Wrap(err, "could not read request")
	}
	return decodeRequest(req)
}

func decodeRequest(req captain.Request) (*trickle.Request, error) {
	engineID := req.EngineId()
	eventID, err := req.EventId()
	if err != nil {
		return nil, errors.Wrap(err, "could not get event id")
	}
	v := &trickle.Request{
		EngineID: engineID,
		EventID:  eventID,
	}
	return v, nil
}

func decodeRootResponse(msg captain.Message) (*trickle.Response, error) {
	response, err := msg.Response()
	if err != nil {
		return nil, errors.Wrap(err, "could not read response")
	}
	return decodeResponse(response)
}

func decodeResponse(response captain.Response) (*trickle.Response, error) {
	engineID := response.EngineId()
	eventID, err := response.EventId()
	if err != nil {
		return nil, errors.Wrap(err, "could not get event id")
	}
	originID, err := response.OriginId()
	if err != nil {
		return nil, errors.Wrap(err, "could not get origin id")
	}
	targetIDs, err := response.TargetIds()
	if err != nil {
		return nil, errors.Wrap(err, "could not get target id list")
	}
	vvs := make([]string, 0, targetIDs.Len())
	for i := 0; i < targetIDs.Len(); i++ {
		vv, err := targetIDs.At(i)
		if err != nil {
			return nil, errors.Wrapf(err, "could not get target id (%d)", i)
		}
		vvs = append(vvs, vv)
	}
	payload, err := response.Payload()
	if err != nil {
		return nil, errors.Wrap(err, "could not get payload")
	}
	v := &trickle.Response{
		EngineID:  engineID,
		EventID:   eventID,
		OriginID:  originID,
		TargetIDs: vvs,
		Payload:   payload,
	}
	return v, nil
}

// collection - collection forwarding

func decodeRootGuaranteedCollection(msg captain.Message) (*collection.GuaranteedCollection, error) {
	coll, err := msg.GuaranteedCollection()
	if err != nil {
		return nil, errors.Wrap(err, "could not read fingerprint")
	}
	return decodeGuaranteedCollection(coll)
}

func decodeGuaranteedCollection(coll captain.GuaranteedCollection) (*collection.GuaranteedCollection, error) {
	hash, err := coll.Hash()
	if err != nil {
		return nil, errors.Wrap(err, "could not get hash")
	}
	sig, err := coll.Signature()
	if err != nil {
		return nil, errors.Wrap(err, "could not get signature")
	}
	v := &collection.GuaranteedCollection{
		Hash:      hash,
		Signature: sig,
	}
	return v, nil
}

// consensus - collection propagation

func decodeRootSnapshotRequest(msg captain.Message) (*consensus.SnapshotRequest, error) {
	req, err := msg.SnapshotRequest()
	if err != nil {
		return nil, errors.Wrap(err, "could not read snapshot request")
	}
	return decodeSnapshotRequest(req)
}

func decodeSnapshotRequest(req captain.SnapshotRequest) (*consensus.SnapshotRequest, error) {
	nonce := req.Nonce()
	hash, err := req.MempoolHash()
	if err != nil {
		return nil, errors.Wrap(err, "could not get mempool hash")
	}
	v := &consensus.SnapshotRequest{
		Nonce:       nonce,
		MempoolHash: hash,
	}
	return v, nil
}

func decodeRootSnapshotResponse(msg captain.Message) (*consensus.SnapshotResponse, error) {
	res, err := msg.SnapshotResponse()
	if err != nil {
		return nil, errors.Wrap(err, "could not read snapshot response")
	}
	return decodeSnapshotResponse(res)
}

func decodeSnapshotResponse(res captain.SnapshotResponse) (*consensus.SnapshotResponse, error) {
	nonce := res.Nonce()
	mempoolHash, err := res.MempoolHash()
	if err != nil {
		return nil, errors.Wrap(err, "could not get mempool hash")
	}
	v := &consensus.SnapshotResponse{
		Nonce:       nonce,
		MempoolHash: mempoolHash,
	}
	return v, nil
}

func decodeRootMempoolRequest(msg captain.Message) (*consensus.MempoolRequest, error) {
	req, err := msg.MempoolRequest()
	if err != nil {
		return nil, errors.Wrap(err, "could not read mempool request")
	}
	return decodeMempoolRequest(req)
}

func decodeMempoolRequest(req captain.MempoolRequest) (*consensus.MempoolRequest, error) {
	nonce := req.Nonce()
	v := &consensus.MempoolRequest{
		Nonce: nonce,
	}
	return v, nil
}

func decodeRootMempoolResponse(msg captain.Message) (*consensus.MempoolResponse, error) {
	res, err := msg.MempoolResponse()
	if err != nil {
		return nil, errors.Wrap(err, "could not read mempool response")
	}
	return decodeMempoolResponse(res)
}

func decodeMempoolResponse(res captain.MempoolResponse) (*consensus.MempoolResponse, error) {
	nonce := res.Nonce()
	fingerprints, err := res.Collections()
	if err != nil {
		return nil, errors.Wrap(err, "could not get fingerprints")
	}
	vvs := make([]*collection.GuaranteedCollection, 0, fingerprints.Len())
	for i := 0; i < fingerprints.Len(); i++ {
		vv, err := decodeGuaranteedCollection(fingerprints.At(i))
		if err != nil {
			return nil, errors.Wrapf(err, "could not get fingerprint (%d)", i)
		}
		vvs = append(vvs, vv)
	}
	v := &consensus.MempoolResponse{
		Nonce:       nonce,
		Collections: vvs,
	}
	return v, nil
}
