package translator

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	crypto_pb "github.com/libp2p/go-libp2p/core/crypto/pb"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
)

// PublicNetworkIDTranslator implements an `p2p.IDTranslator` which translates IDs for peers
// on the unstaked network.
// On the unstaked network, a Flow ID is derived from a peer ID by extracting the public
// key from the peer ID, dropping the first byte (parity byte), and using the remaining
// 32 bytes as the Flow ID.
// Network keys for unstaked nodes must be generated using the Secp256k1 curve, and must
// be positive. It is assumed that these requirements are enforced during key generation,
// and any peer ID's which don't follow these conventions are considered invalid.
type PublicNetworkIDTranslator struct{}

// TODO Rename this one to NewPublicNetworkIDTranslator once observer changes are merged
func NewPublicNetworkIDTranslator() *PublicNetworkIDTranslator {
	return &PublicNetworkIDTranslator{}
}

var _ p2p.IDTranslator = (*PublicNetworkIDTranslator)(nil)

// GetPeerID returns the peer ID for the given Flow ID.
// TODO: implement BFT-compliant error handling -> https://github.com/onflow/flow-go/blob/master/CodingConventions.md
func (t *PublicNetworkIDTranslator) GetPeerID(flowID flow.Identifier) (peer.ID, error) {
	data := append([]byte{0x02}, flowID[:]...)

	um := crypto.PubKeyUnmarshallers[crypto_pb.KeyType_Secp256k1]
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

// GetFlowID returns the Flow ID for the given peer ID.
// TODO: implement BFT-compliant error handling -> https://github.com/onflow/flow-go/blob/master/CodingConventions.md
func (t *PublicNetworkIDTranslator) GetFlowID(peerID peer.ID) (flow.Identifier, error) {
	pk, err := peerID.ExtractPublicKey()
	if err != nil {
		return flow.ZeroID, fmt.Errorf("cannot generate an unstaked FlowID for peerID %v: corresponding libp2p key is not extractible from PeerID: %w", peerID, err)
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
