package flow_test

import (
	"encoding/json"
	"fmt"
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

// TestNewRootHeaderBody verifies the behavior of the NewRootHeaderBody constructor.
// Test Cases:
//
// 1. Valid root input:
//   - Ensures a HeaderBody is returned when only ChainID is set and root constraints hold.
//
// 2. Missing ChainID:
//   - Ensures an error is returned when ChainID is empty.
//
// 3. Non-zero ParentView:
//   - Ensures an error is returned when ParentView is non-zero.
//
// 4. Non-empty ParentVoterIndices:
//   - Ensures an error is returned when ParentVoterIndices is non-empty.
//
// 5. Non-empty ParentVoterSigData:
//   - Ensures an error is returned when ParentVoterSigData is non-empty.
func TestNewRootHeaderBody(t *testing.T) {
	validID := unittest.IdentifierFixture()
	ts := time.Unix(1_600_000_000, 0)

	base := flow.UntrustedHeaderBody{
		ChainID:            flow.Emulator,
		ParentID:           validID,
		Height:             10,
		Timestamp:          ts,
		View:               0,
		ParentView:         0,
		ParentVoterIndices: []byte{},
		ParentVoterSigData: []byte{},
		ProposerID:         validID,
		LastViewTC:         nil,
	}

	t.Run("valid root input", func(t *testing.T) {
		hb, err := flow.NewRootHeaderBody(base)
		assert.NoError(t, err)
		assert.NotNil(t, hb)
		assert.Equal(t, *hb, flow.HeaderBody(base))
		assert.Nil(t, hb.LastViewTC)
	})

	t.Run("missing ChainID", func(t *testing.T) {
		u := base
		u.ChainID = ""
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ChainID of root header body must not be empty")
	})

	t.Run("non-zero ParentView", func(t *testing.T) {
		u := base
		u.ParentView = 1
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentView of root header body must be zero")
	})

	t.Run("non-empty ParentVoterIndices", func(t *testing.T) {
		u := base
		u.ParentVoterIndices = unittest.SignerIndicesFixture(4)
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentVoterIndices of root header body must be empty")
	})

	t.Run("non-empty ParentVoterSigData", func(t *testing.T) {
		u := base
		u.ParentVoterSigData = unittest.QCSigDataFixture()
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentVoterSigData of root header body must be empty")
	})
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
		ParentVoterIndices: unittest.SignerIndicesFixture(4),
		ParentVoterSigData: unittest.QCSigDataFixture(),
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

// TestHeaderBodyBuilder_PresenceChecks verifies that HeaderBodyBuilder.Build
// returns an error when any required setter was not called (tracked via bits in `present`).
func TestHeaderBodyBuilder_PresenceChecks(t *testing.T) {
	validID := unittest.IdentifierFixture()
	ts := time.Unix(1_600_000_000, 0)

	// Define each setter, its field name, and its bit index in HeaderBodyFields.
	setters := []struct {
		name string
		bit  int
		fn   func(*flow.HeaderBodyBuilder)
	}{
		{"ChainID", 0, func(b *flow.HeaderBodyBuilder) {
			b.WithChainID("chain")
		}},
		{"ParentID", 1, func(b *flow.HeaderBodyBuilder) {
			b.WithParentID(validID)
		}},
		{"Height", 2, func(b *flow.HeaderBodyBuilder) {
			b.WithHeight(42)
		}},
		{"Timestamp", 3, func(b *flow.HeaderBodyBuilder) {
			b.WithTimestamp(ts)
		}},
		{"View", 4, func(b *flow.HeaderBodyBuilder) {
			b.WithView(7)
		}},
		{"ParentView", 5, func(b *flow.HeaderBodyBuilder) {
			b.WithParentView(6)
		}},
		{"ParentVoterIndices", 6, func(b *flow.HeaderBodyBuilder) {
			b.WithParentVoterIndices(unittest.SignerIndicesFixture(4))
		}},
		{"ParentVoterSigData", 7, func(b *flow.HeaderBodyBuilder) {
			b.WithParentVoterSigData(unittest.QCSigDataFixture())
		}},
		{"ProposerID", 8, func(b *flow.HeaderBodyBuilder) {
			b.WithProposerID(validID)
		}},
	}

	// 1) Valid builder: all setters called -> no error.
	t.Run("all setters present", func(t *testing.T) {
		b := flow.NewHeaderBodyBuilder()
		for _, s := range setters {
			s.fn(b)
		}
		hb, err := b.Build()
		assert.NoError(t, err)
		assert.NotNil(t, hb)
	})

	// 2) Missing each individual setter -> error naming the correct bit index.
	for _, s := range setters {
		t.Run(fmt.Sprintf("missing %s", s.name), func(t *testing.T) {
			b := flow.NewHeaderBodyBuilder()
			// call every setter except the one we're omitting
			for _, other := range setters {
				if other.bit == s.bit {
					continue
				}
				other.fn(b)
			}
			hb, err := b.Build()
			assert.Error(t, err)
			assert.Nil(t, hb)
			assert.Contains(t, err.Error(), fmt.Sprintf("bit index %d", s.bit))
		})
	}
}

// TestNewGenesisHeader verifies the behavior of the NewGenesisHeader constructor.
//
// Test Cases:
//
// 1. Valid root input:
//   - Ensures a Header is returned when the embedded HeaderBody is a valid root body
//     and PayloadHash is ZeroID.
//
// 2. Invalid root body:
//   - Ensures an error is returned when the embedded HeaderBody is invalid.
//
// 3. Non‐empty PayloadHash:
//   - Ensures an error is returned when PayloadHash is non‐zero.
//
// 4. Non‐empty ParentVoterIndices:
//   - Ensures an error is returned when the root header’s ParentVoterIndices is non‐empty.
func TestNewGenesisHeader(t *testing.T) {
	ts := time.Unix(1_600_000_000, 0)
	validID := unittest.IdentifierFixture()

	rootBody, err := flow.NewRootHeaderBody(flow.UntrustedHeaderBody{
		ChainID:            flow.Emulator,
		ParentView:         0,
		ParentVoterIndices: []byte{},
		ParentVoterSigData: []byte{},
		ParentID:           validID,
		Height:             0,
		Timestamp:          ts,
		View:               0,
		ProposerID:         validID,
	})
	assert.NoError(t, err)

	t.Run("valid root input", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  *rootBody,
			PayloadHash: flow.ZeroID,
		}
		h, err := flow.NewGenesisHeader(u)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, *rootBody, h.HeaderBody)
		assert.Equal(t, flow.ZeroID, h.PayloadHash)
	})

	t.Run("invalid root body", func(t *testing.T) {
		badBody := flow.UntrustedHeaderBody{
			ChainID:            "",
			ParentView:         0,
			ParentVoterIndices: []byte{},
			ParentVoterSigData: []byte{},
		}
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(badBody),
			PayloadHash: flow.ZeroID,
		}
		h, err := flow.NewGenesisHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid root header body")
	})

	t.Run("empty PayloadHash", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  *rootBody,
			PayloadHash: unittest.IdentifierFixture(),
		}
		h, err := flow.NewGenesisHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "PayloadHash")
	})

	t.Run("non‐empty ParentVoterIndices", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  *rootBody,
			PayloadHash: flow.ZeroID,
		}
		// inject a non‐empty ParentVoterIndices, which is invalid for genesis
		u.HeaderBody.ParentVoterIndices = unittest.SignerIndicesFixture(4)
		h, err := flow.NewGenesisHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "ParentVoterIndices")
	})
}

// TestNewRootHeader verifies the behavior of the NewRootHeader constructor.
//
// Test Cases:
//
// 1. Valid root input:
//   - Ensures a Header is returned when the embedded HeaderBody is a valid root body
//     and PayloadHash is ZeroID.
//
// 2. Invalid root body:
//   - Ensures an error is returned when the embedded HeaderBody is invalid.
//
// 3. Empty PayloadHash:
//   - Ensures an error is returned when PayloadHash is zero.
//
// 4. Non‐empty ParentVoterIndices:
//   - Ensures an error is returned when the root header’s ParentVoterIndices is non‐empty.
func TestNewRootHeader(t *testing.T) {
	ts := time.Unix(1_600_000_000, 0)
	validID := unittest.IdentifierFixture()
	validHash := unittest.IdentifierFixture()

	rootBody, err := flow.NewRootHeaderBody(flow.UntrustedHeaderBody{
		ChainID:            flow.Emulator,
		ParentView:         0,
		ParentVoterIndices: []byte{},
		ParentVoterSigData: []byte{},
		ParentID:           validID,
		Height:             0,
		Timestamp:          ts,
		View:               0,
		ProposerID:         validID,
	})
	assert.NoError(t, err)

	t.Run("valid root input", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  *rootBody,
			PayloadHash: validHash,
		}
		h, err := flow.NewRootHeader(u)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, *rootBody, h.HeaderBody)
		assert.Equal(t, validHash, h.PayloadHash)
	})

	t.Run("invalid root body", func(t *testing.T) {
		badBody := flow.UntrustedHeaderBody{
			ChainID:            "",
			ParentView:         0,
			ParentVoterIndices: []byte{},
			ParentVoterSigData: []byte{},
		}
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(badBody),
			PayloadHash: flow.ZeroID,
		}
		h, err := flow.NewRootHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid root header body")
	})

	t.Run("empty PayloadHash", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  *rootBody,
			PayloadHash: flow.ZeroID,
		}
		h, err := flow.NewRootHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "PayloadHash")
	})

	t.Run("non‐empty ParentVoterIndices", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  *rootBody,
			PayloadHash: flow.ZeroID,
		}
		// inject a non‐empty ParentVoterIndices, which is invalid for root
		u.HeaderBody.ParentVoterIndices = unittest.SignerIndicesFixture(4)
		h, err := flow.NewRootHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "ParentVoterIndices")
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
