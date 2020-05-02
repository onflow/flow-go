package flow_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestHeaderEncodingJSON(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	headerID := header.ID()
	data, err := json.Marshal(header)
	require.NoError(t, err)
	var decoded flow.Header
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, headerID, decodedID)
	assert.Equal(t, header, decoded)
}

func TestHeaderEncodingMsgpack(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	headerID := header.ID()
	data, err := msgpack.Marshal(header)
	require.NoError(t, err)
	var decoded flow.Header
	err = msgpack.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, headerID, decodedID)
	assert.Equal(t, header, decoded)
}

func TestNonUTCTimestampSameHashAsUTC(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	headerID := header.ID()
	loc := time.FixedZone("UTC-8", -8*60*60)
	header.Timestamp = header.Timestamp.In(loc)
	checkedID := header.ID()
	assert.Equal(t, headerID, checkedID)
}
