package logging_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/logging"
)

func TestNormalizePattern(t *testing.T) {
	assert.Equal(t, "hotstuff", logging.NormalizePattern("HotStuff"))
	assert.Equal(t, "hotstuff.voter", logging.NormalizePattern("Hotstuff.Voter"))
	assert.Equal(t, "network.*", logging.NormalizePattern("NETWORK.*"))
	assert.Equal(t, "already-lower", logging.NormalizePattern("already-lower"))
}

func TestValidateComponentID(t *testing.T) {
	valid := []string{
		"hotstuff",
		"hotstuff.voter",
		"consensus.vote-aggregator",
		"execution.state_sync",
		"network.p2p.gossipsub",
		"a",
		"a1",
		"node123",
	}
	for _, id := range valid {
		t.Run(id, func(t *testing.T) {
			require.NoError(t, logging.ValidateComponentID(id))
		})
	}

	invalid := []string{
		"",
		"Hotstuff",        // uppercase rejected (must normalize first)
		"hotstuff.",       // trailing dot
		".hotstuff",       // leading dot
		"hotstuff..voter", // double dot
		"hotstuff.*",      // wildcard is not a component ID
		"*",               // reset-all token
		"-hotstuff",       // segment starting with hyphen
		"hotstuff voter",  // space
	}
	for _, id := range invalid {
		t.Run(id, func(t *testing.T) {
			require.Error(t, logging.ValidateComponentID(id))
		})
	}
}

func TestValidatePattern(t *testing.T) {
	valid := []string{
		"hotstuff",
		"hotstuff.voter",
		"hotstuff.*",
		"hotstuff.voter.*",
		"network.*",
		"consensus.vote-aggregator",
		"execution.state_sync.*",
	}
	for _, p := range valid {
		t.Run(p, func(t *testing.T) {
			require.NoError(t, logging.ValidatePattern(p))
		})
	}

	invalid := []string{
		"",
		"*",                // reset-all token, not a pattern
		"hotstuff.**",      // double wildcard
		"hotstuff.*.voter", // wildcard in the middle
		"*.hotstuff",       // wildcard at the beginning
		"hotstuff*",        // wildcard without dot
		"HotStuff",         // uppercase rejected (must normalize first)
		"-hotstuff",        // segment starting with hyphen
		"hotstuff.",        // trailing dot without wildcard
	}
	for _, p := range invalid {
		t.Run(p, func(t *testing.T) {
			require.Error(t, logging.ValidatePattern(p))
		})
	}
}

// TestLogRegistry_NormalizesComponentID verifies that Logger() normalizes the ID to
// lowercase, such that mixed-case and lowercase refer to the same registration.
func TestLogRegistry_NormalizesComponentID(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "HotStuff") // registered as "hotstuff"

	// Prove they are the same registration: changing via the lowercase key is
	// reflected in the mixed-case lookup, and attempting to re-register panics.
	r.SetLevel("hotstuff", zerolog.DebugLevel)
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("HotStuff"), "mixed-case lookup should see the level change")

	require.Panics(t, func() { r.Logger(log, "hotstuff") }, "re-registering the normalized form should panic")
}

// TestLogRegistry_NormalizesSetLevel verifies that SetLevel normalizes the pattern.
func TestLogRegistry_NormalizesSetLevel(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "hotstuff.voter")

	r.SetLevel("Hotstuff.Voter", zerolog.DebugLevel)
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff.voter"))
}

// TestLogRegistry_PanicsOnInvalidComponentID verifies that Logger() panics for invalid IDs.
func TestLogRegistry_PanicsOnInvalidComponentID(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	require.Panics(t, func() { r.Logger(log, "invalid id with spaces") })
	require.Panics(t, func() { r.Logger(log, ".leading-dot") })
	require.Panics(t, func() { r.Logger(log, "trailing-dot.") })
}

// TestParseComponentLogLevels_NormalizesAndValidatesPatterns verifies that parsing
// normalizes component names and rejects invalid patterns.
func TestParseComponentLogLevels_NormalizesAndValidatesPatterns(t *testing.T) {
	// Mixed case is normalized
	result, err := logging.ParseComponentLogLevels("HotStuff:debug")
	require.NoError(t, err)
	assert.Equal(t, zerolog.DebugLevel, result["hotstuff"])
	_, hasOriginal := result["HotStuff"]
	assert.False(t, hasOriginal, "original case key should not be present")

	// Invalid pattern is rejected
	_, err = logging.ParseComponentLogLevels("invalid id:debug")
	require.Error(t, err)
}
