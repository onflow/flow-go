package gnode

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

//closeConnection closes a grpc client connection and prints the errors, if any,
func closeConnection(conn *grpc.ClientConn) {
	err := conn.Close()
	if err != nil {
		fmt.Printf("Error closing grpc connection %v", err)
	}
}

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
