package gnode

import (
	"testing"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/golang/protobuf/proto"
)

//TestComputeHash tests the computeHash helper function
func TestComputeHash(t *testing.T) {

	msg1, _ := generateGossipMessage([]byte("hi"), []string{}, 0)
	msg1Bytes, _ := proto.Marshal(msg1)
	alg, _ := crypto.NewHasher(crypto.SHA3_256)
	h1 := alg.ComputeHash(msg1Bytes)

	msg2, _ := generateGossipMessage([]byte("nohi"), []string{}, 0)
	msg2Bytes, _ := proto.Marshal(msg2)
	h2 := alg.ComputeHash(msg2Bytes)

	tt := []struct {
		msg          *shared.GossipMessage
		expectedHash []byte
		modification []byte
	}{
		{
			msg:          msg1,
			expectedHash: h1[:],
			modification: []byte("why"),
		},
		{
			msg:          msg2,
			expectedHash: h2[:],
			modification: []byte("starset"),
		},
	}

	for _, tc := range tt {
		res1, _ := computeHash(tc.msg)
		// testing if hash generated properly
		if string(res1) != string(tc.expectedHash) {
			t.Errorf("computHash: Expected: %v, Got: %v", tc.expectedHash, res1)
		}
		tc.msg.Payload = tc.modification
		// testing if hash changes after modifying payload
		res2, _ := computeHash(tc.msg)
		if string(res2) == string(res1) {
			t.Errorf("computeHash: Expected: not equal after modifying, Got: Equal after modifying")
		}
	}
}
