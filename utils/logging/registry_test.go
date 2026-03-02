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

// TestLogRegistry_SetLevelExact verifies that SetLevel with an exact pattern updates only
// the matching registered component.
func TestLogRegistry_SetLevelExact(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger("hotstuff")
	r.Logger("hotstuff.voter")

	r.SetLevel("hotstuff", zerolog.DebugLevel)

	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff"))
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff.voter"))
}

// TestLogRegistry_SetLevelWildcard verifies that SetLevel with a wildcard pattern updates
// all matching children but not the parent.
func TestLogRegistry_SetLevelWildcard(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger("hotstuff")
	r.Logger("hotstuff.voter")
	r.Logger("hotstuff.pacemaker")
	r.Logger("network")

	r.SetLevel("hotstuff.*", zerolog.DebugLevel)

	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff"))
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff.voter"))
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff.pacemaker"))
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("network"))
}

// TestLogRegistry_SetLevelMoreSpecificOverrideNotClobbered verifies that a more specific
// override is not overwritten by a less specific wildcard set.
func TestLogRegistry_SetLevelMoreSpecificOverrideNotClobbered(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger("hotstuff.voter")

	r.SetLevel("hotstuff.voter", zerolog.WarnLevel) // exact override first
	r.SetLevel("hotstuff.*", zerolog.DebugLevel)    // wildcard applied second

	// exact override beats wildcard
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("hotstuff.voter"))
}

// TestLogRegistry_SetLevelUpdatesGlobalLevel verifies that lowering a component level
// also lowers zerolog.GlobalLevel.
func TestLogRegistry_SetLevelUpdatesGlobalLevel(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger("hotstuff")

	r.SetLevel("hotstuff", zerolog.DebugLevel)
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
}

// TestLogRegistry_ResetExact verifies that Reset removes a runtime override, restoring
// the component to its static config or global default.
func TestLogRegistry_ResetExact(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff": zerolog.WarnLevel,
	})
	r.Logger("hotstuff")
	r.SetLevel("hotstuff", zerolog.DebugLevel)
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff"))

	r.Reset([]string{"hotstuff"})
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("hotstuff")) // back to static
}

// TestLogRegistry_ResetWildcard verifies that Reset with a wildcard removes matching
// overrides and re-resolves affected components.
func TestLogRegistry_ResetWildcard(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger("hotstuff.voter")
	r.Logger("hotstuff.pacemaker")

	r.SetLevel("hotstuff.*", zerolog.DebugLevel)
	r.Reset([]string{"hotstuff.*"})

	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff.voter"))
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff.pacemaker"))
}

// TestLogRegistry_ResetAll verifies that Reset(["*"]) restores all components to static
// config or global default.
func TestLogRegistry_ResetAll(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger("hotstuff")
	r.Logger("network")

	r.SetLevel("hotstuff", zerolog.DebugLevel)
	r.SetLevel("network", zerolog.TraceLevel)
	r.Reset([]string{"*"})

	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff"))
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("network"))
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
}

// TestLogRegistry_SetDefaultLevel verifies that updating the default re-resolves components
// without explicit overrides and leaves overridden components unchanged.
func TestLogRegistry_SetDefaultLevel(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger("hotstuff")
	r.Logger("network")
	r.SetLevel("network", zerolog.WarnLevel)

	r.SetDefaultLevel(zerolog.DebugLevel)

	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff")) // re-resolved
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("network"))   // override preserved
}

// TestLogRegistry_Levels verifies that Levels returns the correct level and source for
// each registered component.
func TestLogRegistry_Levels(t *testing.T) {
	r := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff.voter": zerolog.WarnLevel,
		"network.*":      zerolog.ErrorLevel,
	})
	r.Logger("hotstuff")
	r.Logger("hotstuff.voter")
	r.Logger("network.p2p")
	r.SetLevel("hotstuff", zerolog.DebugLevel)

	defaultLevel, levels := r.Levels()
	assert.Equal(t, zerolog.InfoLevel, defaultLevel)

	assert.Equal(t, zerolog.DebugLevel, levels["hotstuff"].Level)
	assert.Equal(t, logging.LevelSourceOverride, levels["hotstuff"].Source)

	assert.Equal(t, zerolog.WarnLevel, levels["hotstuff.voter"].Level)
	assert.Equal(t, logging.LevelSourceStatic, levels["hotstuff.voter"].Source)

	assert.Equal(t, zerolog.ErrorLevel, levels["network.p2p"].Level)
	assert.Equal(t, logging.LevelSourceStaticWildcard, levels["network.p2p"].Source)
}
