package gnode

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/golang/protobuf/proto"
)

// extractHashMsgInfo receives the bytes of a HashMessage instance, unmarshals it and returns
// its components
func extractHashMsgInfo(hashMsgBytes []byte) ([]byte, *shared.Socket, error) {
	hashMsg := &shared.HashMessage{}
	if err := proto.Unmarshal(hashMsgBytes, hashMsg); err != nil {
		return nil, &shared.Socket{}, fmt.Errorf("could not unmarshal message: %v", err)
	}

	return hashMsg.GetHashBytes(), hashMsg.GetSenderSocket(), nil
}

// computeHash computes the hash of GossipMessage using sha256 algorithm
func computeHash(msg *shared.GossipMessage) ([]byte, error) {

	alg, err := crypto.NewHasher(crypto.SHA3_256)
	if err != nil {
		return nil, err
	}
	msgHash := alg.ComputeHash(msg.GetPayload())

	return msgHash[:], nil
}
