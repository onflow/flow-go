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

// TestNewRootHeaderBody verifies that NewRootHeaderBody enforces root‐only constraints.
//
// Test Cases:
//
// 1. Valid root input with non‐zero ParentID:
//   - Ensures a HeaderBody is returned when only ChainID, Timestamp are set,
//     ParentView is zero, and the three QC fields are empty.
//
// 2. Valid root input with zero ParentID:
//   - Same as above, but ParentID==ZeroID.  Should also succeed.
//
// 3. Missing ChainID:
//   - Ensures an error is returned when ChainID is empty.
//
// 4. Non‐empty ParentVoterIndices:
//   - Ensures an error is returned when ParentVoterIndices is non‐empty.
//
// 5. Non‐empty ParentVoterSigData:
//   - Ensures an error is returned when ParentVoterSigData is non‐empty.
//
// 6. Non‐zero ProposerID:
//   - Ensures an error is returned when ProposerID is non‐zero.
//
// 7. Non‐zero ParentView:
//   - Ensures an error is returned when ParentView is non‐zero.
//
// 8. Zero Timestamp:
//   - Ensures an error is returned when Timestamp is the zero value.
func TestNewRootHeaderBody(t *testing.T) {
	validID := unittest.IdentifierFixture()
	ts := time.Unix(1_600_000_000, 0)

	// Base untrusted root header: ChainID & Timestamp set, no QC fields, ParentView==0
	base := flow.UntrustedHeaderBody{
		ChainID:            flow.Emulator,
		ParentID:           validID, // allowed to be non-zero
		Height:             0,
		Timestamp:          ts,
		View:               0,
		ParentView:         0,
		ParentVoterIndices: []byte{},
		ParentVoterSigData: []byte{},
		ProposerID:         flow.ZeroID, // must be zero
		LastViewTC:         nil,
	}

	t.Run("valid root input with non-zero ParentID", func(t *testing.T) {
		hb, err := flow.NewRootHeaderBody(base)
		assert.NoError(t, err)
		assert.NotNil(t, hb)
	})

	t.Run("valid root input with zero ParentID", func(t *testing.T) {
		u := base
		u.ParentID = flow.ZeroID
		hb, err := flow.NewRootHeaderBody(u)
		assert.NoError(t, err)
		assert.NotNil(t, hb)
	})

	t.Run("missing ChainID", func(t *testing.T) {
		u := base
		u.ChainID = ""
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ChainID of root header body must not be empty")
	})

	t.Run("non-empty ParentVoterIndices", func(t *testing.T) {
		u := base
		u.ParentVoterIndices = unittest.SignerIndicesFixture(4)
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "must not set ParentVoterIndices")
	})

	t.Run("non-empty ParentVoterSigData", func(t *testing.T) {
		u := base
		u.ParentVoterSigData = unittest.QCSigDataFixture()
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "must not set ParentVoterSigData")
	})

	t.Run("non-zero ProposerID", func(t *testing.T) {
		u := base
		u.ProposerID = validID
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "must not set ProposerID")
	})

	t.Run("non-zero ParentView", func(t *testing.T) {
		u := base
		u.ParentView = 1
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentView of root header body must be zero")
	})

	t.Run("zero Timestamp", func(t *testing.T) {
		u := base
		u.Timestamp = time.Time{}
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "Timestamp of root header body must not be zero")
	})
}

// / TestNewHeaderBody verifies the behavior of the NewHeaderBody constructor after
// grouping of the parent‐QC checks and addition of height/view ordering checks.
//
// Test Cases:
//
// 1. Valid input:
//   - All required fields are set (ChainID, parent‐QC, Height>0, View>0, ParentView<View, non-zero Timestamp).
//   - Expect no error.
//
// 2. Missing ChainID:
//   - ChainID is empty.
//   - Ensures an error mentioning "ChainID".
//
// 3. Missing parent‐QC fields:
//   - ParentID, ParentVoterIndices, ParentVoterSigData or ProposerID is missing (nil/zero).
//   - Ensures an error mentioning "missing parent QC".
//
// 4. Zero Height:
//   - Height set to 0.
//   - Ensures an error mentioning "Height must be > 0".
//
// 5. Zero View:
//   - View set to 0.
//   - Ensures an error mentioning "View must be > 0".
//
// 6. ParentView ≥ View:
//   - ParentView is equal to or greater than View.
//   - Ensures an error mentioning "ParentView".
//
// 7. Zero Timestamp:
//   - Timestamp is the zero value.
//   - Ensures an error mentioning "Timestamp must not be zero-value".
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
	})

	t.Run("missing ChainID", func(t *testing.T) {
		u := base
		u.ChainID = ""
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ChainID must not be empty")
	})

	t.Run("missing parent QC", func(t *testing.T) {
		u := base
		u.ParentID = flow.ZeroID
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "missing parent QC")
	})

	t.Run("nil ParentVoterIndices", func(t *testing.T) {
		u := base
		u.ParentVoterIndices = nil
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "missing parent QC")
	})

	t.Run("nil ParentVoterSigData", func(t *testing.T) {
		u := base
		u.ParentVoterSigData = nil
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "missing parent QC")
	})

	t.Run("missing ProposerID", func(t *testing.T) {
		u := base
		u.ProposerID = flow.ZeroID
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "missing parent QC")
	})

	t.Run("zero Height", func(t *testing.T) {
		u := base
		u.Height = 0
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "Height must be > 0")
	})

	t.Run("zero View", func(t *testing.T) {
		u := base
		u.View = 0
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "View must be > 0")
	})

	t.Run("ParentView ≥ View", func(t *testing.T) {
		u := base
		u.ParentView = view
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentView")
	})

	t.Run("zero Timestamp", func(t *testing.T) {
		u := base
		u.Timestamp = time.Time{}
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "Timestamp must not be zero-value")
	})
}

// TestHeaderBodyBuilder_PresenceChecks verifies that HeaderBodyBuilder.Build
// returns an error when any required setter was not called (tracked via bits in `present`).
func TestHeaderBodyBuilder_PresenceChecks(t *testing.T) {
	validID := unittest.IdentifierFixture()
	ts := time.Unix(1_600_000_000, 0)

	// Each entry names the field and provides the setter to call.
	setters := []struct {
		field string
		fn    func(*flow.HeaderBodyBuilder)
	}{
		{"ChainID", func(b *flow.HeaderBodyBuilder) {
			b.WithChainID("chain")
		}},
		{"ParentID", func(b *flow.HeaderBodyBuilder) {
			b.WithParentID(validID)
		}},
		{"Height", func(b *flow.HeaderBodyBuilder) {
			b.WithHeight(42)
		}},
		{"Timestamp", func(b *flow.HeaderBodyBuilder) {
			b.WithTimestamp(ts)
		}},
		{"View", func(b *flow.HeaderBodyBuilder) {
			b.WithView(7)
		}},
		{"ParentView", func(b *flow.HeaderBodyBuilder) {
			b.WithParentView(6)
		}},
		{"ParentVoterIndices", func(b *flow.HeaderBodyBuilder) {
			b.WithParentVoterIndices(unittest.SignerIndicesFixture(4))
		}},
		{"ParentVoterSigData", func(b *flow.HeaderBodyBuilder) {
			b.WithParentVoterSigData(unittest.QCSigDataFixture())
		}},
		{"ProposerID", func(b *flow.HeaderBodyBuilder) {
			b.WithProposerID(validID)
		}},
	}

	// When all setters are called, Build should succeed.
	t.Run("all setters present", func(t *testing.T) {
		b := flow.NewHeaderBodyBuilder()
		for _, s := range setters {
			s.fn(b)
		}
		hb, err := b.Build()
		assert.NoError(t, err)
		assert.NotNil(t, hb)
	})

	// Omitting each setter in turn should produce an error mentioning that field.
	for _, s := range setters {
		t.Run(fmt.Sprintf("missing %s", s.field), func(t *testing.T) {
			b := flow.NewHeaderBodyBuilder()
			// call every setter except the one we're omitting
			for _, other := range setters {
				if other.field == s.field {
					continue
				}
				other.fn(b)
			}
			hb, err := b.Build()
			assert.Error(t, err)
			assert.Nil(t, hb)
			assert.Contains(t, err.Error(), s.field)
		})
	}
}

// TestNewRootHeader verifies the behavior of the NewRootHeader constructor.
//
// Test Cases:
//
// 1. Valid root input:
//   - Embedded HeaderBody is a valid root body and PayloadHash is non-zero.
//   - Ensures no error and returned Header has the correct fields.
//
// 2. Valid root input with ParentID == ZeroID:
//   - Same as above but ParentID is zero.  Should still succeed.
//
// 3. Invalid root body:
//   - Embedded HeaderBody fails root-only constraints.
//   - Ensures an error wrapping "invalid root header body".
//
// 4. Empty PayloadHash:
//   - PayloadHash is ZeroID.
//   - Ensures an error mentioning "PayloadHash must not be empty".
//
// 5. Non-empty ParentVoterIndices:
//   - Ensures an error is returned when ParentVoterIndices is non-empty.
//
// 6. Non-empty ParentVoterSigData:
//   - Ensures an error is returned when ParentVoterSigData is non-empty.
//
// 7. Non-zero ProposerID:
//   - Ensures an error is returned when ProposerID is non-zero.
//
// TestNewRootHeader verifies the behavior of the NewRootHeader constructor.
//
// Test Cases:
//
// 1. Valid root input:
//   - Embedded HeaderBody is a valid root body and PayloadHash is non-zero.
//   - Ensures no error and returned Header has the correct fields.
//
// 2. Valid root input with ParentID == ZeroID and PayloadHash non-zero:
//   - Same as above but ParentID is zero.  Should still succeed.
//
// 3. Invalid root body:
//   - Embedded HeaderBody fails root-only constraints (e.g. missing ChainID).
//   - Ensures an error wrapping "invalid root header body".
//
// 4. Empty PayloadHash:
//   - PayloadHash is ZeroID.
//   - Ensures an error mentioning "PayloadHash must not be empty".
//
// 5. Any QC‐field violation in the embedded body:
//   - Even if PayloadHash is non-zero, if ParentVoterIndices, ParentVoterSigData or ProposerID
//     are non‐empty in the root body, NewRootHeader should fail at the body check.
//   - Ensures an error wrapping "invalid root header body".
func TestNewRootHeader(t *testing.T) {
	ts := time.Unix(1_600_000_000, 0)
	nonZeroHash := unittest.IdentifierFixture()
	validID := unittest.IdentifierFixture()

	//  Prepare a valid root HeaderBody once
	rootBody, err := flow.NewRootHeaderBody(flow.UntrustedHeaderBody{
		ChainID:            flow.Emulator,
		ParentID:           validID, // allowed
		Height:             0,
		Timestamp:          ts,
		View:               0,
		ParentView:         0,
		ParentVoterIndices: []byte{},    // must be empty
		ParentVoterSigData: []byte{},    // must be empty
		ProposerID:         flow.ZeroID, // must be zero
		LastViewTC:         nil,
	})
	assert.NoError(t, err)

	t.Run("valid root input", func(t *testing.T) {
		u := flow.UntrustedHeader{
			HeaderBody:  *rootBody,
			PayloadHash: nonZeroHash,
		}
		h, err := flow.NewRootHeader(u)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, *rootBody, h.HeaderBody)
		assert.Equal(t, nonZeroHash, h.PayloadHash)
	})

	t.Run("valid root input with ParentID == ZeroID", func(t *testing.T) {
		body := *rootBody
		body.ParentID = flow.ZeroID
		u := flow.UntrustedHeader{
			HeaderBody:  body,
			PayloadHash: nonZeroHash,
		}
		h, err := flow.NewRootHeader(u)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, flow.ZeroID, h.HeaderBody.ParentID)
		assert.Equal(t, nonZeroHash, h.PayloadHash)
	})

	t.Run("invalid root body (missing ChainID)", func(t *testing.T) {
		// Build an entirely unvalidated flow.HeaderBody by casting an UntrustedHeaderBody)
		badBody := flow.HeaderBody(flow.UntrustedHeaderBody{
			ChainID:            "", // invalid
			ParentID:           flow.ZeroID,
			Height:             0,
			Timestamp:          ts,
			View:               0,
			ParentView:         0,
			ParentVoterIndices: []byte{},
			ParentVoterSigData: []byte{},
			ProposerID:         flow.ZeroID,
		})
		u := flow.UntrustedHeader{
			HeaderBody:  badBody,
			PayloadHash: nonZeroHash,
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
		assert.Contains(t, err.Error(), "PayloadHash must not be empty")
	})

	t.Run("body with ParentVoterIndices set", func(t *testing.T) {
		badBody := flow.HeaderBody(flow.UntrustedHeaderBody{
			ChainID:            flow.Emulator,
			ParentID:           flow.ZeroID,
			Height:             0,
			Timestamp:          ts,
			View:               0,
			ParentView:         0,
			ParentVoterIndices: unittest.SignerIndicesFixture(3), // invalid
			ParentVoterSigData: []byte{},
			ProposerID:         flow.ZeroID,
		})
		u := flow.UntrustedHeader{
			HeaderBody:  badBody,
			PayloadHash: nonZeroHash,
		}
		h, err := flow.NewRootHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid root header body: root header body must not set ParentVoterIndices")
	})

	t.Run("body with ParentVoterSigData set", func(t *testing.T) {
		badBody := flow.HeaderBody(flow.UntrustedHeaderBody{
			ChainID:            flow.Emulator,
			ParentID:           flow.ZeroID,
			Height:             0,
			Timestamp:          ts,
			View:               0,
			ParentView:         0,
			ParentVoterIndices: []byte{},
			ParentVoterSigData: unittest.QCSigDataFixture(), // invalid
			ProposerID:         flow.ZeroID,
		})
		u := flow.UntrustedHeader{
			HeaderBody:  badBody,
			PayloadHash: nonZeroHash,
		}
		h, err := flow.NewRootHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid root header body: root header body must not set ParentVoterSigData")
	})

	t.Run("body with ProposerID set", func(t *testing.T) {
		badBody := flow.HeaderBody(flow.UntrustedHeaderBody{
			ChainID:            flow.Emulator,
			ParentID:           flow.ZeroID,
			Height:             0,
			Timestamp:          ts,
			View:               0,
			ParentView:         0,
			ParentVoterIndices: []byte{},
			ParentVoterSigData: []byte{},
			ProposerID:         validID, // invalid
		})
		u := flow.UntrustedHeader{
			HeaderBody:  badBody,
			PayloadHash: nonZeroHash,
		}
		h, err := flow.NewRootHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid root header body: root header body must not set ProposerID")
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
