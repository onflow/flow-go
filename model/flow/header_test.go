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

func TestHeaderMalleability(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	// Require that LastViewTC (TimeoutCertificate) is not malleable, since its ID is incorporated in Header's ID
	unittest.RequireEntityNonMalleable(t, helper.MakeTC())
	unittest.RequireEntityNonMalleable(t, header)
}

// TestNewRootHeaderBody verifies that NewRootHeaderBody enforces root‐only constraints,
// using a fixture helper to supply only the field overrides needed per case.
//
// Test Cases:
//
// 1. Valid root input with non‐zero ParentID:
//   - Ensures a HeaderBody is returned when only ChainID and Timestamp are set,
//     ParentView is zero, QC fields empty, and ParentID != ZeroID.
//
// 2. Valid root input with zero ParentID:
//   - Same as above, but ParentID==ZeroID. Should also succeed.
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
	t.Run("valid root input with non-zero ParentID", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(WithRootDefaults())
		hb, err := flow.NewRootHeaderBody(u)
		assert.NoError(t, err)
		assert.NotNil(t, hb)
	})

	t.Run("valid root input with zero ParentID", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ParentID = flow.ZeroID
			})
		hb, err := flow.NewRootHeaderBody(u)
		assert.NoError(t, err)
		assert.NotNil(t, hb)
	})

	t.Run("missing ChainID", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ChainID = ""
			})
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ChainID of root header body must not be empty")
	})

	t.Run("non-empty ParentVoterIndices", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ParentVoterIndices = unittest.SignerIndicesFixture(4)
			})
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "must not set ParentVoterIndices")
	})

	t.Run("non-empty ParentVoterSigData", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ParentVoterSigData = unittest.QCSigDataFixture()
			})
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "must not set ParentVoterSigData")
	})

	t.Run("non-zero ProposerID", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ProposerID = unittest.IdentifierFixture()
			})
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "must not set ProposerID")
	})

	t.Run("non-zero ParentView", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ParentView = 7
			})
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentView of root header body must be zero")
	})

	t.Run("zero Timestamp", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.Timestamp = 0
			})
		hb, err := flow.NewRootHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "Timestamp of root header body must not be zero")
	})
}

// TestNewHeaderBody verifies the behavior of the NewHeaderBody constructor after
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
	t.Run("valid input", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture()
		hb, err := flow.NewHeaderBody(u)
		assert.NoError(t, err)
		assert.NotNil(t, hb)
		assert.Equal(t, flow.HeaderBody(u), *hb)
	})

	t.Run("missing ChainID", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.ChainID = ""
		})
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ChainID must not be empty")
	})

	t.Run("missing parent QC: ParentID", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.ParentID = flow.ZeroID
		})
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentID must not be empty")
	})

	t.Run("missing parent QC: ParentVoterIndices", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.ParentVoterIndices = nil
		})
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentVoterIndices must not be empty")
	})

	t.Run("missing parent QC: ParentVoterSigData", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.ParentVoterSigData = nil
		})
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentVoterSigData must not be empty")
	})

	t.Run("missing parent QC: ProposerID", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.ProposerID = flow.ZeroID
		})
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ProposerID must not be empty")
	})

	t.Run("zero Height", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.Height = 0
		})
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "Height must be > 0")
	})

	t.Run("zero View", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.View = 0
		})
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "View must be > 0")
	})

	t.Run("ParentView ≥ View", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			// set ParentView equal to the generated View
			u.ParentView = u.View
		})
		hb, err := flow.NewHeaderBody(u)
		assert.Error(t, err)
		assert.Nil(t, hb)
		assert.Contains(t, err.Error(), "ParentView")
	})

	t.Run("zero Timestamp", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.Timestamp = 0
		})
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
	ts := uint64(time.Unix(1_600_000_000, 0).UnixMilli())

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
// 5. Body with ParentVoterIndices set:
//   - Ensures an error wrapping "invalid root header body" is returned when HeaderBody.ParentVoterIndices is non-empty.
//
// 6. Body with ParentVoterSigData set:
//   - Ensures an error wrapping "invalid root header body" is returned when HeaderBody.ParentVoterSigData is non-empty.
//
// 7. Body with ProposerID set:
//   - Ensures an error wrapping "invalid root header body" is returned when HeaderBody.ProposerID is non-zero.
func TestNewRootHeader(t *testing.T) {
	nonZeroHash := unittest.IdentifierFixture()
	validID := unittest.IdentifierFixture()

	t.Run("valid root input", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
		)
		h, err := flow.NewRootHeader(flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(u),
			PayloadHash: nonZeroHash,
		})
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, nonZeroHash, h.PayloadHash)
	})

	t.Run("valid root input with zero ParentID", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ParentID = flow.ZeroID
			},
		)
		h, err := flow.NewRootHeader(flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(u),
			PayloadHash: nonZeroHash,
		})
		assert.NoError(t, err)
		assert.Zero(t, h.ParentID)
	})

	t.Run("invalid root body (missing ChainID)", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ChainID = ""
			},
		)
		h, err := flow.NewRootHeader(flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(u),
			PayloadHash: nonZeroHash,
		})
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid root header body")
	})

	t.Run("empty PayloadHash", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(WithRootDefaults())
		h, err := flow.NewRootHeader(flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(u),
			PayloadHash: flow.ZeroID,
		})
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "PayloadHash must not be empty")
	})

	t.Run("body with ParentVoterIndices set", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ParentVoterIndices = unittest.SignerIndicesFixture(3)
			},
		)
		h, err := flow.NewRootHeader(flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(u),
			PayloadHash: nonZeroHash,
		})
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid root header body")
	})

	t.Run("body with ParentVoterSigData set", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ParentVoterSigData = unittest.QCSigDataFixture()
			},
		)
		h, err := flow.NewRootHeader(flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(u),
			PayloadHash: nonZeroHash,
		})
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid root header body")
	})

	t.Run("body with ProposerID set", func(t *testing.T) {
		u := UntrustedHeaderBodyFixture(
			WithRootDefaults(),
			func(u *flow.UntrustedHeaderBody) {
				u.ProposerID = validID
			},
		)
		h, err := flow.NewRootHeader(flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(u),
			PayloadHash: nonZeroHash,
		})
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid root header body")
	})
}

// TestNewHeader verifies the behavior of the NewHeader constructor.
//
// Test Cases:
//
// 1. Valid input:
//   - Ensures a Header is returned when HeaderBody is valid and PayloadHash is non-zero.
//
// 2. Empty ChainID:
//   - Ensures an error is returned when the embedded HeaderBody.ChainID is empty.
//
// 3. Zero ParentID:
//   - Ensures an error is returned when the embedded HeaderBody.ParentID is ZeroID.
//
// 4. Zero Timestamp:
//   - Ensures an error is returned when the embedded HeaderBody.Timestamp is zero.
//
// 5. Empty ParentVoterIndices:
//   - Ensures an error is returned when the embedded HeaderBody.ParentVoterIndices is empty.
//
// 6. Empty ParentVoterSigData:
//   - Ensures an error is returned when the embedded HeaderBody.ParentVoterSigData is empty.
//
// 7. Zero ProposerID:
//   - Ensures an error is returned when the embedded HeaderBody.ProposerID is ZeroID.
//
// 8. Missing PayloadHash:
//   - Ensures an error is returned when PayloadHash is ZeroID.
func TestNewHeader(t *testing.T) {
	validHash := unittest.IdentifierFixture()

	t.Run("valid input", func(t *testing.T) {
		// start from a fully-populated, valid untrusted header body
		uBody := UntrustedHeaderBodyFixture()
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(uBody),
			PayloadHash: validHash,
		}
		h, err := flow.NewHeader(u)
		assert.NoError(t, err)
		assert.NotNil(t, h)
		assert.Equal(t, flow.HeaderBody(uBody), h.HeaderBody)
		assert.Equal(t, validHash, h.PayloadHash)
	})

	t.Run("empty ChainID", func(t *testing.T) {
		uBody := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.ChainID = ""
		})
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(uBody),
			PayloadHash: validHash,
		}
		h, err := flow.NewHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("zero ParentID", func(t *testing.T) {
		uBody := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.ParentID = flow.ZeroID
		})
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(uBody),
			PayloadHash: validHash,
		}
		h, err := flow.NewHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("zero Timestamp", func(t *testing.T) {
		uBody := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.Timestamp = 0
		})
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(uBody),
			PayloadHash: validHash,
		}
		h, err := flow.NewHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("empty ParentVoterIndices", func(t *testing.T) {
		uBody := UntrustedHeaderBodyFixture(
			func(u *flow.UntrustedHeaderBody) {
				u.ParentVoterIndices = []byte{}
			},
			func(u *flow.UntrustedHeaderBody) {
				u.ParentVoterSigData = []byte{}
			},
		)
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(uBody),
			PayloadHash: validHash,
		}
		h, err := flow.NewHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("empty ParentVoterSigData", func(t *testing.T) {
		uBody := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.ParentVoterSigData = []byte{}
		})
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(uBody),
			PayloadHash: validHash,
		}
		h, err := flow.NewHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("zero ProposerID", func(t *testing.T) {
		uBody := UntrustedHeaderBodyFixture(func(u *flow.UntrustedHeaderBody) {
			u.ProposerID = flow.ZeroID
		})
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(uBody),
			PayloadHash: validHash,
		}
		h, err := flow.NewHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("missing PayloadHash", func(t *testing.T) {
		// use a valid body but an empty payload hash
		uBody := UntrustedHeaderBodyFixture()
		u := flow.UntrustedHeader{
			HeaderBody:  flow.HeaderBody(uBody),
			PayloadHash: flow.ZeroID,
		}
		h, err := flow.NewHeader(u)
		assert.Error(t, err)
		assert.Nil(t, h)
		assert.Contains(t, err.Error(), "PayloadHash must not be empty")
	})
}

// UntrustedHeaderBodyFixture returns an UntrustedHeaderBody
// pre‐populated with sane defaults. Any opts override those defaults.
func UntrustedHeaderBodyFixture(opts ...func(*flow.UntrustedHeaderBody)) flow.UntrustedHeaderBody {
	u := flow.UntrustedHeaderBody(unittest.HeaderBodyFixture())
	for _, opt := range opts {
		opt(&u)
	}
	return u
}

// WithRootDefaults zeroes out all parent‐QC fields and enforces root constraints.
func WithRootDefaults() func(*flow.UntrustedHeaderBody) {
	ts := uint64(time.Unix(1_600_000_000, 0).UnixMilli())
	return func(u *flow.UntrustedHeaderBody) {
		u.ChainID = flow.Emulator // still must be non‐empty
		u.ParentID = flow.ZeroID  // allowed to be zero
		u.Height = 0
		u.View = 0
		u.Timestamp = ts // non‐zero
		u.ParentView = 0
		u.ParentVoterIndices = []byte{}
		u.ParentVoterSigData = []byte{}
		u.ProposerID = flow.ZeroID
		u.LastViewTC = nil
	}
}
