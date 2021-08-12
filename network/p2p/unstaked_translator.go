package p2p

import (
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
		// TODO: return error
	}

	return peer.ID(mh), nil
}

func (t *UnstakedNetworkIDTranslator) GetFlowID(peerID peer.ID) (flow.Identifier, error) {
	pk, err := peerID.ExtractPublicKey()
	if err != nil {
		// return error
	}

	if pk.Type() != crypto_pb.KeyType_ECDSA {
		// fail
	}

	data, err := pk.Raw()
	if err != nil || data[0] != 0x02 { // TODO: check if this is the right byte to check
		// fail
	}

	return flow.HashToID(data[1:]), nil
}
