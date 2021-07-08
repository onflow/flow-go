package codec_test

// pushd network ; BINSTAT_LEN_WHAT="~net=99;~lock=99;~Backend=99" BINSTAT_ENABLE=1 GO111MODULE=on go test -v -coverprofile=coverage.txt -covermode=atomic --tags relic ./codec/roundTripHeader_test.go ; popd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRoundTripHeader(t *testing.T) {
	codec := json.NewCodec()
	block := unittest.BlockFixture()  
	message := &messages.BlockProposal{ Header: block.Header, Payload: block.Payload }
	encoded, err := codec.Encode(message)
	assert.NoError(t, err)
	decodedInterface, err := codec.Decode(encoded)
	assert.NoError(t, err)
	decoded := decodedInterface.(*messages.BlockProposal)
	assert.Equal(t, message.Header.ProposerSig, decoded.Header.ProposerSig)
	messageHeader := fmt.Sprintf("- .Header=%+v\n", message.Header)
	decodedHeader := fmt.Sprintf("- .Header=%+v\n", decoded.Header)
	assert.Equal(t, messageHeader, decodedHeader)
}
