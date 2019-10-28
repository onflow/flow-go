package gnode

import (
	"testing"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/stretchr/testify/assert"
)

//TestComputeHash tests the computeHash helper function
func TestComputeHash(t *testing.T) {
	assert := assert.New(t)
	msg1 := generateGossipMessage([]byte("hi"), []string{}, 0)
	alg, _ := crypto.NewHasher(crypto.SHA3_256)
	h1 := alg.ComputeHash(msg1.GetPayload())

	msg2 := generateGossipMessage([]byte("nohi"), []string{}, 0)
	h2 := alg.ComputeHash(msg2.GetPayload())

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
		assert.Equal(string(tc.expectedHash), string(res1))
		tc.msg.Payload = tc.modification
		// testing if hash changes after modifying payload
		res2, _ := computeHash(tc.msg)
		assert.NotEqual(string(res2), string(res1))
	}
}
