package flow_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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
	assert.Equal(t, *header, decoded)
}

func TestHeaderFingerprint(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	header.LastViewTC = helper.MakeTC()
	headerID := header.ID()
	data := header.Fingerprint()
	var decoded flow.EncodableHeader
	rlp.NewMarshaler().MustUnmarshal(data, &decoded)
	decHeader := &flow.Header{
		ChainID:            decoded.ChainID,
		ParentID:           decoded.ParentID,
		Height:             decoded.Height,
		PayloadHash:        decoded.PayloadHash,
		Timestamp:          time.Unix(0, int64(decoded.Timestamp)).UTC(),
		View:               decoded.View,
		ParentView:         decoded.ParentView,
		ParentVoterIndices: decoded.ParentVoterIndices,
		ParentVoterSigData: decoded.ParentVoterSigData,
		ProposerID:         decoded.ProposerID,
		LastViewTC:         header.LastViewTC,
	}
	decodedID := decHeader.ID()
	assert.Equal(t, headerID, decodedID)
	assert.Equal(t, *header, *decHeader)
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
	assert.Equal(t, *header, decoded)
}

func TestHeaderEncodingCBOR(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	headerID := header.ID()
	data, err := cbor.Marshal(header)
	require.NoError(t, err)
	var decoded flow.Header
	err = cbor.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, headerID, decodedID)
	assert.Equal(t, *header, decoded)
}

func TestNonUTCTimestampSameHashAsUTC(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	headerID := header.ID()
	loc := time.FixedZone("UTC-8", -8*60*60)
	header.Timestamp = header.Timestamp.In(loc)
	checkedID := header.ID()
	assert.Equal(t, headerID, checkedID)
}

func TestHeaderMalleability(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	// Require that LastViewTC (TimeoutCertificate) is not malleable, since its ID is incorporated in Header's ID
	unittest.RequireEntityNonMalleable(t, helper.MakeTC())
	// time.Time contains private fields, so we provide a field generator
	timestampGenerator := func() time.Time { return time.Now().UTC() }
	unittest.RequireEntityNonMalleable(t, header, unittest.WithFieldGenerator("Timestamp", timestampGenerator))
}
