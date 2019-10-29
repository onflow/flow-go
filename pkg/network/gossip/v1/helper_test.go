package gnode

import (
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/stretchr/testify/assert"
)

//TestComputeHash tests the computeHash helper function
func TestComputeHash(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	msg1, err := generateGossipMessage([]byte("hi"), []string{}, 0)
	require.Nil(err, "non-nil error")
	alg, err := crypto.NewHasher(crypto.SHA3_256)
	require.Nil(err, "non-nil error")
	h1 := alg.ComputeHash(msg1.GetPayload())

	msg2, err := generateGossipMessage([]byte("nohi"), []string{}, 0)
	require.Nil(err, "non-nil error")
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
		res1, err := computeHash(tc.msg)
		require.Nil(err, "non-nil error")
		// testing if hash generated properly
		assert.Equal(string(tc.expectedHash), string(res1))
		tc.msg.Payload = tc.modification
		// testing if hash changes after modifying payload
		res2, err := computeHash(tc.msg)
		require.Nil(err, "non-nil error")
		assert.NotEqual(string(res2), string(res1))
	}
}
