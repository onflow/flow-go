package gossip

import (
	"fmt"

	"github.com/golang/protobuf/proto"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// extractHashMsgInfo receives the bytes of a HashMessage instance, unmarshals it and returns
// its components
func extractHashMsgInfo(hashMsgBytes []byte) ([]byte, *messages.Socket, error) {
	hashMsg := &messages.HashMessage{}
	if err := proto.Unmarshal(hashMsgBytes, hashMsg); err != nil {
		return nil, &messages.Socket{}, fmt.Errorf("could not unmarshal message: %v", err)
	}

	return hashMsg.GetHashBytes(), hashMsg.GetSenderSocket(), nil
}

// computeHash computes the hash of GossipMessage using sha256 algorithm
func computeHash(msg *messages.GossipMessage) ([]byte, error) {

	msgBytes, err := proto.Marshal(msg)

	if err != nil {
		return nil, fmt.Errorf("could not marshal message: %v", err)
	}

	//msgHash := sha256.Sum256(msgBytes)
	alg, err := crypto.NewHasher(crypto.SHA3_256)
	if err != nil {
		return nil, err
	}
	msgHash := alg.ComputeHash(msgBytes)

	return msgHash[:], nil
}
