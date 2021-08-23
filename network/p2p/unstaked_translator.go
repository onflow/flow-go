package p2p

import (
	"fmt"

	lcrypto "github.com/libp2p/go-libp2p-core/crypto"
	crypto_pb "github.com/libp2p/go-libp2p-core/crypto/pb"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// UnstakedNetworkIDTranslator implements an IDTranslator which translates IDs for peers
// on the unstaked network.
// On the unstaked network, a Flow ID is derived from a peer ID by extracting the public
// key from the peer ID, dropping the first byte (parity byte), and using the remaining
// 32 bytes as the Flow ID.
// Network keys for unstaked nodes must be generated using the Secp256k1 curve, and must
// be positive. It is assumed that these requirements are enforced during key generation,
// and any peer ID's which don't follow these conventions are considered invalid.
type UnstakedNetworkIDTranslator struct{}

func NewUnstakedNetworkIDTranslator() *UnstakedNetworkIDTranslator {
	return &UnstakedNetworkIDTranslator{}
}

func (t *UnstakedNetworkIDTranslator) GetPeerID(flowID flow.Identifier) (peer.ID, error) {
	data := append([]byte{0x02}, flowID[:]...)

	um := lcrypto.PubKeyUnmarshallers[crypto_pb.KeyType_Secp256k1]
	key, err := um(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert flow ID to libp2p public key: %w", err)
	}

	pid, err := peer.IDFromPublicKey(key)
	if err != nil {
		return "", fmt.Errorf("failed to get peer ID from libp2p public key: %w", err)
	}

	return pid, nil
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
