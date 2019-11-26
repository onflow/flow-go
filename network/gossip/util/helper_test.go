package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
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
		msg          *messages.GossipMessage
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
		res1, err := ComputeHash(tc.msg)
		require.Nil(err, "non-nil error")
		// testing if hash generated properly
		assert.Equal(string(tc.expectedHash), string(res1))
		tc.msg.Payload = tc.modification
		// testing if hash changes after modifying payload
		res2, err := ComputeHash(tc.msg)
		require.Nil(err, "non-nil error")
		assert.NotEqual(string(res2), string(res1))
	}
}

func TestRandomSubSet(t *testing.T) {
	peers := []string{
		"Wyatt",
		"Jayden",
		"John",
		"Owen",
		"Dylan",
		"Luke",
		"Gabriel",
		"Anthony",
		"Isaac",
		"Grayson",
		"Jack",
		"Julian",
		"Levi",
		"Christopher",
		"Joshua",
		"Andrew",
		"Lincoln",
	}

	for i := 1; i < len(peers)/2; i++ {
		subset, err := RandomSubSet(peers, i)
		require.Nil(t, err)
		require.Len(t, subset, i)
	}
}

func generateGossipMessage(payloadBytes []byte, recipients []string, msgType uint64) (*messages.GossipMessage, error) {
	return &messages.GossipMessage{
		Payload:     payloadBytes,
		MessageType: msgType,
		Recipients:  recipients,
	}, nil
}
