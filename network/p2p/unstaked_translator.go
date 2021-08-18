package p2p

import (
	"fmt"

	crypto_pb "github.com/libp2p/go-libp2p-core/crypto/pb"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multihash"

	"github.com/onflow/flow-go/model/flow"
)

type UnstakedNetworkIDTranslator struct{}

func NewUnstakedNetworkIDTranslator() *UnstakedNetworkIDTranslator {
	return &UnstakedNetworkIDTranslator{}
}

func (t *UnstakedNetworkIDTranslator) GetPeerID(flowID flow.Identifier) (peer.ID, error) {
	data := append([]byte{0x02}, flowID[:]...)
	mh, err := multihash.Sum(data, multihash.IDENTITY, -1)
	if err != nil {
		return "", fmt.Errorf("failed to compute multihash: %w", err)
	}

	return peer.ID(mh), nil
}

func (t *UnstakedNetworkIDTranslator) GetFlowID(peerID peer.ID) (flow.Identifier, error) {
	pk, err := peerID.ExtractPublicKey()
	if err != nil {
		return flow.ZeroID, fmt.Errorf("cannot generate an unstaked FlowID for peerID %v: corresponding libp2p key is not extractible from PeerID", peerID)
	}

	if pk.Type() != crypto_pb.KeyType_Secp256k1 {
		return flow.ZeroID, fmt.Errorf("cannot generate an unstaked FlowID for peerID %v: corresponding libp2p key is not a %v key", peerID, crypto_pb.KeyType_name[(int32)(crypto_pb.KeyType_Secp256k1)])
	}

	data, err := pk.Raw()
	if err != nil || data[0] != 0x02 {
		return flow.ZeroID, fmt.Errorf("cannot generate an unstaked FlowID for peerID %v: corresponding libp2p key is invalid or negative", peerID)
	}

	return flow.HashToID(data[1:]), nil
}
