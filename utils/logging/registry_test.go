package logging_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/logging"
)

func testRegistry(t *testing.T, defaultLevel zerolog.Level, static map[string]zerolog.Level) (*logging.LogRegistry, zerolog.Logger) {
	t.Helper()

	originalGlobalLevel := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(originalGlobalLevel) })

	r := logging.NewLogRegistry(os.Stderr, defaultLevel, static)
	return r, zerolog.New(os.Stderr).Level(zerolog.TraceLevel)
}

// testRegistryWithBuffer creates a LogRegistry backed by a bytes.Buffer for output inspection.
func testRegistryWithBuffer(t *testing.T, defaultLevel zerolog.Level) (*logging.LogRegistry, zerolog.Logger, *bytes.Buffer) {
	t.Helper()

	originalGlobalLevel := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(originalGlobalLevel) })

	var buf bytes.Buffer
	baseLogger := zerolog.New(&buf).Level(zerolog.TraceLevel)
	r := logging.NewLogRegistry(&buf, defaultLevel, nil)
	return r, baseLogger, &buf
}

// TestLogRegistry_RegisterReturnsLogger verifies Logger() returns a usable zerolog.Logger.
func TestLogRegistry_RegisterReturnsLogger(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	logger := r.Logger(log, "hotstuff")
	logger.Info().Msg("test")
}

// TestLogRegistry_DuplicateRegisterPanics verifies that registering the same ID twice panics.
func TestLogRegistry_DuplicateRegisterPanics(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "hotstuff")
	require.Panics(t, func() { r.Logger(log, "hotstuff") })
}

// TestLogRegistry_ResolvesDefaultLevel verifies that a component with no static config
// or override resolves to globalDefault.
func TestLogRegistry_ResolvesDefaultLevel(t *testing.T) {
	r, log := testRegistry(t, zerolog.WarnLevel, nil)
	r.Logger(log, "hotstuff")
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("hotstuff"))
}

// TestLogRegistry_ResolvesStaticExact verifies that a static exact match beats globalDefault.
func TestLogRegistry_ResolvesStaticExact(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff": zerolog.DebugLevel,
	})
	r.Logger(log, "hotstuff")
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff"))
}

// TestLogRegistry_ResolvesStaticWildcard verifies that a static wildcard match is applied
// when no exact match exists.
func TestLogRegistry_ResolvesStaticWildcard(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff.*": zerolog.DebugLevel,
	})
	r.Logger(log, "hotstuff.voter")
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff.voter"))
}

// TestLogRegistry_StaticWildcardDoesNotMatchParent verifies that "hotstuff.*" does not
// match "hotstuff" itself.
func TestLogRegistry_StaticWildcardDoesNotMatchParent(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff.*": zerolog.DebugLevel,
	})
	r.Logger(log, "hotstuff")
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff"))
}

// TestLogRegistry_MostSpecificWildcardWins verifies that the longest matching prefix
// wildcard takes priority.
func TestLogRegistry_MostSpecificWildcardWins(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff.*":       zerolog.DebugLevel,
		"hotstuff.voter.*": zerolog.WarnLevel,
	})
	r.Logger(log, "hotstuff.voter.timer")
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("hotstuff.voter.timer"))
}

// TestLogRegistry_GlobalLevelSetToMinimum verifies that zerolog.GlobalLevel is set to
// the minimum of all component levels after registration.
func TestLogRegistry_GlobalLevelSetToMinimum(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff": zerolog.DebugLevel,
	})
	r.Logger(log, "hotstuff")
	r.Logger(log, "network")
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
}

// TestLogRegistry_SetLevelExact verifies that SetLevel with an exact pattern updates only
// the matching registered component.
func TestLogRegistry_SetLevelExact(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "hotstuff")
	r.Logger(log, "hotstuff.voter")

	r.SetLevel("hotstuff", zerolog.DebugLevel)

	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff"))
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff.voter"))
}

// TestLogRegistry_SetLevelWildcard verifies that SetLevel with a wildcard pattern updates
// all matching children but not the parent.
func TestLogRegistry_SetLevelWildcard(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "hotstuff")
	r.Logger(log, "hotstuff.voter")
	r.Logger(log, "hotstuff.pacemaker")
	r.Logger(log, "network")

	r.SetLevel("hotstuff.*", zerolog.DebugLevel)

	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff"))
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff.voter"))
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff.pacemaker"))
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("network"))
}

// TestLogRegistry_SetLevelMoreSpecificOverrideNotClobbered verifies that a more specific
// override is not overwritten by a less specific wildcard set.
func TestLogRegistry_SetLevelMoreSpecificOverrideNotClobbered(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "hotstuff.voter")

	r.SetLevel("hotstuff.voter", zerolog.WarnLevel) // exact override first
	r.SetLevel("hotstuff.*", zerolog.DebugLevel)    // wildcard applied second

	// exact override beats wildcard
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("hotstuff.voter"))
}

// TestLogRegistry_SetLevelUpdatesGlobalLevel verifies that lowering a component level
// also lowers zerolog.GlobalLevel.
func TestLogRegistry_SetLevelUpdatesGlobalLevel(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "hotstuff")

	r.SetLevel("hotstuff", zerolog.DebugLevel)
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
}

// TestLogRegistry_ResetExact verifies that Reset removes a runtime override, restoring
// the component to its static config or global default.
func TestLogRegistry_ResetExact(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff": zerolog.WarnLevel,
	})
	r.Logger(log, "hotstuff")
	r.SetLevel("hotstuff", zerolog.DebugLevel)
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff"))

	r.Reset("hotstuff")
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("hotstuff")) // back to static
}

// TestLogRegistry_ResetWildcard verifies that Reset with a wildcard removes matching
// overrides and re-resolves affected components.
func TestLogRegistry_ResetWildcard(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "hotstuff.voter")
	r.Logger(log, "hotstuff.pacemaker")

	r.SetLevel("hotstuff.*", zerolog.DebugLevel)
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff.voter"))
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff.pacemaker"))

	r.Reset("hotstuff.*")
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff.voter"))
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff.pacemaker"))
}

// TestLogRegistry_ResetAll verifies that Reset("*") restores all components to static
// config or global default.
func TestLogRegistry_ResetAll(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "hotstuff")
	r.Logger(log, "network")

	r.SetLevel("hotstuff", zerolog.DebugLevel)
	r.SetLevel("network", zerolog.TraceLevel)
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff"))
	assert.Equal(t, zerolog.TraceLevel, r.EffectiveLevel("network"))

	r.Reset("*")
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("hotstuff"))
	assert.Equal(t, zerolog.InfoLevel, r.EffectiveLevel("network"))
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
}

// TestLogRegistry_SetDefaultLevel verifies that updating the default re-resolves components
// without explicit overrides and leaves overridden components unchanged.
func TestLogRegistry_SetDefaultLevel(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, nil)
	r.Logger(log, "hotstuff")
	r.Logger(log, "network")

	r.SetLevel("network", zerolog.WarnLevel)
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("network"))

	r.SetDefaultLevel(zerolog.DebugLevel)
	assert.Equal(t, zerolog.DebugLevel, r.EffectiveLevel("hotstuff")) // re-resolved
	assert.Equal(t, zerolog.WarnLevel, r.EffectiveLevel("network"))   // override preserved
}

// TestLogRegistry_Levels verifies that Levels returns the correct level and source for
// each registered component.
func TestLogRegistry_Levels(t *testing.T) {
	r, log := testRegistry(t, zerolog.InfoLevel, map[string]zerolog.Level{
		"hotstuff.voter": zerolog.WarnLevel,
		"network.*":      zerolog.ErrorLevel,
	})
	r.Logger(log, "hotstuff")
	r.Logger(log, "hotstuff.voter")
	r.Logger(log, "network.p2p")
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

// TestLogRegistry_OutputFiltering_DefaultLevel verifies that at the default level (info),
// debug events produce no output and info events are written.
func TestLogRegistry_OutputFiltering_DefaultLevel(t *testing.T) {
	r, log, buf := testRegistryWithBuffer(t, zerolog.InfoLevel)
	logger := r.Logger(log, "hotstuff")

	logger.Debug().Msg("debug suppressed")
	assert.Empty(t, buf.String(), "debug should produce no output at info level")

	logger.Info().Msg("info visible")
	assert.Contains(t, buf.String(), "info visible")
}

// TestLogRegistry_OutputFiltering_LowerLevel verifies that after SetLevel lowers a
// component to debug, debug events are written to the output.
func TestLogRegistry_OutputFiltering_LowerLevel(t *testing.T) {
	r, log, buf := testRegistryWithBuffer(t, zerolog.InfoLevel)
	logger := r.Logger(log, "hotstuff")

	logger.Debug().Msg("debug suppressed")
	assert.Empty(t, buf.String(), "debug should produce no output at info level")

	r.SetLevel("hotstuff", zerolog.DebugLevel)

	logger.Debug().Msg("debug now visible")
	assert.Contains(t, buf.String(), "debug now visible")
}

// TestLogRegistry_OutputFiltering_ResetRestoresSuppression verifies that after Reset,
// debug events are suppressed again.
func TestLogRegistry_OutputFiltering_ResetRestoresSuppression(t *testing.T) {
	r, log, buf := testRegistryWithBuffer(t, zerolog.InfoLevel)
	logger := r.Logger(log, "hotstuff")

	r.SetLevel("hotstuff", zerolog.DebugLevel)
	logger.Debug().Msg("debug visible")
	require.Contains(t, buf.String(), "debug visible")

	r.Reset("hotstuff")

	logger.Debug().Msg("debug suppressed again")
	assert.Empty(t, buf.String(), "debug should be suppressed after reset")

	logger.Info().Msg("info still visible")
	assert.Contains(t, buf.String(), "info still visible")
}

// TestLogRegistry_OutputFiltering_IndependentComponents verifies that lowering one
// component's level does not cause another component to emit events below its level.
// This tests the key design property: each component has its own componentLevelWriter.
func TestLogRegistry_OutputFiltering_IndependentComponents(t *testing.T) {
	r, log, buf := testRegistryWithBuffer(t, zerolog.InfoLevel)
	loggerA := r.Logger(log, "component-a") // stays at info
	loggerB := r.Logger(log, "component-b") // lowered to debug

	r.SetLevel("component-b", zerolog.DebugLevel)
	// GlobalLevel is now debug to allow B's debug events through — but A's writer
	// should still discard them.

	loggerA.Debug().Msg("a-debug")
	assert.Empty(t, buf.String(), "component-a should not emit debug despite global level drop")

	loggerB.Debug().Msg("b-debug")
	assert.Contains(t, buf.String(), "b-debug", "component-b should emit debug")

	loggerA.Info().Msg("a-info")
	assert.Contains(t, buf.String(), "a-info", "component-a should still emit info")
}

// TestLogRegistry_OutputFiltering_ChildLoggerInheritsLevel verifies that child loggers
// derived from a component logger via With() share the same componentLevelWriter and
// automatically reflect level changes without re-registration.
func TestLogRegistry_OutputFiltering_ChildLoggerInheritsLevel(t *testing.T) {
	r, log, buf := testRegistryWithBuffer(t, zerolog.InfoLevel)
	parent := r.Logger(log, "hotstuff")
	child := parent.With().Str("sub", "voter").Logger()

	// Before: both suppressed at info
	parent.Debug().Msg("parent debug suppressed")
	child.Debug().Msg("child debug suppressed")
	assert.Empty(t, buf.String())

	// Lower the component level — child should pick it up automatically
	r.SetLevel("hotstuff", zerolog.DebugLevel)

	child.Debug().Msg("child debug visible")
	assert.Contains(t, buf.String(), "child debug visible",
		"child should inherit level change via shared componentLevelWriter")
}

// TestLogRegistry_OutputFiltering_WildcardAffectsOutput verifies that a wildcard SetLevel
// affects the actual output of all matching components.
func TestLogRegistry_OutputFiltering_WildcardAffectsOutput(t *testing.T) {
	r, log, buf := testRegistryWithBuffer(t, zerolog.InfoLevel)
	voter := r.Logger(log, "hotstuff.voter")
	pacemaker := r.Logger(log, "hotstuff.pacemaker")

	r.SetLevel("hotstuff.*", zerolog.DebugLevel)

	voter.Debug().Msg("voter-debug")
	pacemaker.Debug().Msg("pacemaker-debug")

	assert.Contains(t, buf.String(), "voter-debug")
	assert.Contains(t, buf.String(), "pacemaker-debug")
}

// TestLogRegistry_Logger_InheritsParentContext verifies that Logger preserves
// all context fields added to the parent logger before the child is registered.
func TestLogRegistry_Logger_InheritsParentContext(t *testing.T) {
	r, log, buf := testRegistryWithBuffer(t, zerolog.InfoLevel)

	parent := r.Logger(log, "hotstuff")
	parent = parent.With().Str("view", "42").Logger() // add context after registration

	child := r.Logger(parent, "hotstuff.voter")

	child.Info().Msg("child message")
	out := buf.String()
	assert.Contains(t, out, "child message")
	assert.Contains(t, out, `"view":"42"`, "child should inherit parent's context field")
}

// TestLogRegistry_Logger_IndependentLevelControl verifies that the child registered
// via Logger has its own independently controllable level.
func TestLogRegistry_Logger_IndependentLevelControl(t *testing.T) {
	r, log, buf := testRegistryWithBuffer(t, zerolog.InfoLevel)

	parent := r.Logger(log, "hotstuff")
	child := r.Logger(parent, "hotstuff.voter")

	// Lower child to debug — parent stays at info
	r.SetLevel("hotstuff.voter", zerolog.DebugLevel)

	parent.Debug().Msg("parent-debug")
	assert.Empty(t, buf.String(), "parent should still be at info")

	child.Debug().Msg("child-debug")
	assert.Contains(t, buf.String(), "child-debug")
}

// TestLogRegistry_Logger_ParentLevelDoesNotAffectChild verifies that lowering the
// parent's level does not cause the child to emit events below its own level.
func TestLogRegistry_Logger_ParentLevelDoesNotAffectChild(t *testing.T) {
	r, log, buf := testRegistryWithBuffer(t, zerolog.InfoLevel)

	parent := r.Logger(log, "hotstuff")
	child := r.Logger(parent, "hotstuff.voter")

	// Lower parent to debug — child stays at info
	r.SetLevel("hotstuff", zerolog.DebugLevel)

	child.Debug().Msg("child-debug")
	assert.Empty(t, buf.String(), "child should still be at info")

	parent.Debug().Msg("parent-debug")
	assert.Contains(t, buf.String(), "parent-debug")
}

func TestBestWildcardMatch(t *testing.T) {
	tests := []struct {
		name          string
		config        map[string]zerolog.Level
		id            string
		expectedLevel zerolog.Level
		expectedMatch bool
	}{
		{
			name:          "empty config",
			config:        map[string]zerolog.Level{},
			id:            "hotstuff.voter",
			expectedMatch: false,
		},
		{
			name:          "exact pattern ignored",
			config:        map[string]zerolog.Level{"hotstuff.voter": zerolog.DebugLevel},
			id:            "hotstuff.voter",
			expectedMatch: false,
		},
		{
			name:          "single match",
			config:        map[string]zerolog.Level{"hotstuff.*": zerolog.DebugLevel},
			id:            "hotstuff.voter",
			expectedLevel: zerolog.DebugLevel,
			expectedMatch: true,
		},
		{
			name:          "wildcard does not match parent",
			config:        map[string]zerolog.Level{"hotstuff.*": zerolog.DebugLevel},
			id:            "hotstuff",
			expectedMatch: false,
		},
		{
			name:          "unrelated wildcard",
			config:        map[string]zerolog.Level{"network.*": zerolog.DebugLevel},
			id:            "hotstuff.voter",
			expectedMatch: false,
		},
		{
			name: "most specific wins",
			config: map[string]zerolog.Level{
				"hotstuff.*":       zerolog.DebugLevel,
				"hotstuff.voter.*": zerolog.WarnLevel,
			},
			id:            "hotstuff.voter.timer",
			expectedLevel: zerolog.WarnLevel,
			expectedMatch: true,
		},
		{
			name:          "shallow wildcard matches deep child",
			config:        map[string]zerolog.Level{"hotstuff.*": zerolog.DebugLevel},
			id:            "hotstuff.voter.timer",
			expectedLevel: zerolog.DebugLevel,
			expectedMatch: true,
		},
		{
			name: "one of multiple wildcards matches",
			config: map[string]zerolog.Level{
				"network.*":  zerolog.ErrorLevel,
				"hotstuff.*": zerolog.DebugLevel,
			},
			id:            "hotstuff.voter",
			expectedLevel: zerolog.DebugLevel,
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, ok := logging.BestWildcardMatch(tt.config, tt.id)
			assert.Equal(t, tt.expectedMatch, ok)
			if tt.expectedMatch {
				assert.Equal(t, tt.expectedLevel, level)
			}
		})
	}
}
