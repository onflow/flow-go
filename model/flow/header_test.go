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
	unittest.RequireEntityNonMalleable(t, header, unittest.WithFieldGenerator("HeaderBody.Timestamp", timestampGenerator))
}

// TestNewHeaderBody verifies the behavior of the NewHeaderBody constructor.
// Test Cases:
//
// 1. Valid input:
//   - Ensures a HeaderBody is returned when all required fields are non-zero/nil.
//
// 2. Missing ChainID:
//   - Ensures an error is returned when ChainID is empty.
//
// 3. Missing ParentID:
//   - Ensures an error is returned when ParentID is ZeroID.
//
// 4. Zero Timestamp:
//   - Ensures an error is returned when Timestamp is zero-value.
//
// 5. Nil ParentVoterIndices:
//   - Ensures an error is returned when ParentVoterIndices is nil.
//
// 6. Empty ParentVoterIndices:
//   - Ensures an error is returned when ParentVoterIndices is empty.
//
// 7. Nil ParentVoterSigData:
//   - Ensures an error is returned when ParentVoterSigData is nil.
//
// 8. Empty ParentVoterSigData:
//   - Ensures an error is returned when ParentVoterSigData is empty.
//
// 9. Missing ProposerID:
//   - Ensures an error is returned when ProposerID is ZeroID.
func TestNewHeaderBody(t *testing.T) {
	validID := unittest.IdentifierFixture()
	ts := time.Unix(1_600_000_000, 0)
	view := uint64(5)
	parentView := uint64(4)

	base := flow.UntrustedHeaderBody{
		ChainID:            flow.Emulator,
		ParentID:           validID,
		Height:             42,
		Timestamp:          ts,
		View:               view,
		ParentView:         parentView,
		ParentVoterIndices: []byte{0x01},
		ParentVoterSigData: []byte{0x02},
		ProposerID:         validID,
		LastViewTC:         nil,
	}

	t.Run("valid input", func(t *testing.T) {
		hb, err := flow.NewHeaderBody(base)
		assert.NoError(t, err)
		assert.NotNil(t, hb)
		assert.Equal(t, *hb, flow.HeaderBody(base))
		assert.Nil(t, hb.LastViewTC)
	})

	t.Run("missing ChainID", func(t *testing.T) {
		u := base
		u.ChainID = ""
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ChainID")
	})

	t.Run("missing ParentID", func(t *testing.T) {
		u := base
		u.ParentID = flow.ZeroID
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentID")
	})

	t.Run("zero Timestamp", func(t *testing.T) {
		u := base
		u.Timestamp = time.Time{}
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "Timestamp")
	})

	t.Run("nil ParentVoterIndices", func(t *testing.T) {
		u := base
		u.ParentVoterIndices = nil
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentVoterIndices")
	})

	t.Run("empty ParentVoterIndices", func(t *testing.T) {
		u := base
		u.ParentVoterIndices = []byte{}
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentVoterIndices")
	})

	t.Run("nil ParentVoterSigData", func(t *testing.T) {
		u := base
		u.ParentVoterSigData = nil
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentVoterSigData")
	})

	t.Run("empty ParentVoterSigData", func(t *testing.T) {
		u := base
		u.ParentVoterSigData = []byte{}
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentVoterSigData")
	})

	t.Run("missing ProposerID", func(t *testing.T) {
		u := base
		u.ProposerID = flow.ZeroID
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ProposerID")
	})
}

// TestNewHeader verifies the behavior of the NewHeader constructor.
// Test Cases:
//
// 1. Valid input:
//   - Ensures a Header is returned when HeaderBody is valid and PayloadHash is non-zero.
//
// 2. Invalid HeaderBody:
//   - Ensures an error is returned when the embedded HeaderBody is invalid.
//
// 3. Missing PayloadHash:
//   - Ensures an error is returned when PayloadHash is ZeroID.
func TestNewHeader(t *testing.T) {
	validID := unittest.IdentifierFixture()
	ts := time.Unix(1_600_000_000, 0)

	hb, err := flow.NewHeaderBody(flow.UntrustedHeaderBody{
		ChainID:            "chain",
		ParentID:           validID,
		Height:             1,
		Timestamp:          ts,
		View:               2,
		ParentView:         1,
		ParentVoterIndices: []byte{0x01},
		ParentVoterSigData: []byte{0x02},
		ProposerID:         validID,
	})
	assert.NoError(t, err)

	t.Run("valid input", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  *hb,
			PayloadHash: validID,
		}
		h, err := flow.NewHeader(u)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, *hb, h.HeaderBody)
		assert.Equal(t, validID, h.PayloadHash)
	})

	t.Run("invalid HeaderBody", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody{}, // missing required fields
			PayloadHash: validID,
		}
		h, err := flow.NewHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("missing PayloadHash", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  *hb,
			PayloadHash: flow.ZeroID,
		}
		h, err := flow.NewHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "PayloadHash")
	})
}
