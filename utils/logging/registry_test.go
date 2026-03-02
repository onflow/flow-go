package logging_test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/logging"
)

func testRegistry(t *testing.T, defaultLevel zerolog.Level, static map[string]zerolog.Level) *logging.LogRegistry {
	t.Helper()
	baseLogger := zerolog.New(os.Stderr)
	r := logging.NewLogRegistry(baseLogger, os.Stderr, defaultLevel, static)
	t.Cleanup(func() { zerolog.SetGlobalLevel(zerolog.TraceLevel) })
	return r
}

// TestLogRegistry_RegisterReturnsLogger verifies Logger() returns a usable zerolog.Logger.
func TestLogRegistry_RegisterReturnsLogger(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, nil)
	logger := r.Logger("hotstuff")
	logger.Info().Msg("test")
}

// TestLogRegistry_DuplicateRegisterPanics verifies that registering the same ID twice panics.
func TestLogRegistry_DuplicateRegisterPanics(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger("hotstuff")
	require.Panics(t, func() { r.Logger("hotstuff") })
}

// TestLogRegistry_ResolvesDefaultLevel verifies that a component with no static config
// or override resolves to globalDefault.
func TestLogRegistry_ResolvesDefaultLevel(t *testing.T) {
	r := testRegistry(t, zerolog.WarnLevel, nil)
	r.Logger("hotstuff")
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("hotstuff"))
}

// TestLogRegistry_ResolvesStaticExact verifies that a static exact match beats globalDefault.
func TestLogRegistry_ResolvesStaticExact(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff": zerolog.DebugLevel,
	})
	r.Logger("hotstuff")
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff"))
}

// TestLogRegistry_ResolvesStaticWildcard verifies that a static wildcard match is applied
// when no exact match exists.
func TestLogRegistry_ResolvesStaticWildcard(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff.*": zerolog.DebugLevel,
	})
	r.Logger("hotstuff.voter")
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff.voter"))
}

// TestLogRegistry_StaticWildcardDoesNotMatchParent verifies that "hotstuff.*" does not
// match "hotstuff" itself.
func TestLogRegistry_StaticWildcardDoesNotMatchParent(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff.*": zerolog.DebugLevel,
	})
	r.Logger("hotstuff")
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff"))
}

// TestLogRegistry_MostSpecificWildcardWins verifies that the longest matching prefix
// wildcard takes priority.
func TestLogRegistry_MostSpecificWildcardWins(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff.*":       zerolog.DebugLevel,
		"hotstuff.voter.*": zerolog.WarnLevel,
	})
	r.Logger("hotstuff.voter.timer")
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("hotstuff.voter.timer"))
}

// TestLogRegistry_GlobalLevelSetToMinimum verifies that zerolog.GlobalLevel is set to
// the minimum of all component levels after registration.
func TestLogRegistry_GlobalLevelSetToMinimum(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff": zerolog.DebugLevel,
	})
	r.Logger("hotstuff")
	r.Logger("network")
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
}
