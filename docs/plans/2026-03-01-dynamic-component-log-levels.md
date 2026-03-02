# Dynamic Per-Component Log Levels — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow operators to control zerolog verbosity per component, both statically via CLI flag
and dynamically via admin server commands, with hierarchical wildcard patterns.

**Architecture:** A `LogRegistry` in `utils/logging/` owns a `componentLevelWriter` per registered
component. Each writer holds a pointer to an atomic level that the registry updates on admin
commands. The registry also owns `zerolog.SetGlobalLevel` to preserve pre-creation optimisation.
All filtering happens at write time via a single atomic read — no locking on the hot path.

**Tech Stack:** `github.com/rs/zerolog`, `sync/atomic`, existing admin command framework
(`admin/commands`), `cmd/scaffold.go` node builder.

**Design doc:** `docs/plans/2026-03-01-dynamic-component-log-levels-design.md`

---

## Task 1: `componentLevelWriter`

The internal write-time filter. Every log event written by a component-registered logger passes
through this before reaching the real output.

**Files:**
- Create: `utils/logging/component_writer.go`
- Create: `utils/logging/component_writer_test.go`

**Step 1: Write the failing test**

```go
// utils/logging/component_writer_test.go
package logging_test

import (
	"bytes"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/logging"
)

// TestComponentLevelWriter_FiltersBelow verifies that events below the configured level
// are silently discarded without writing to the delegate.
func TestComponentLevelWriter_FiltersBelow(t *testing.T) {
	var buf bytes.Buffer
	delegate := logging.NoopLevelWriter(&buf)

	var lvl atomic.Int32
	lvl.Store(int32(zerolog.InfoLevel))

	w := logging.NewComponentLevelWriter(&lvl, delegate)
	n, err := w.WriteLevel(zerolog.DebugLevel, []byte(`{"level":"debug","msg":"hi"}`))
	require.NoError(t, err)
	assert.Equal(t, 0, buf.Len(), "expected nothing written for debug below info")
	assert.Greater(t, n, 0, "expected non-zero n even on discard")
}

// TestComponentLevelWriter_PassesAtOrAbove verifies that events at or above the configured
// level are forwarded to the delegate unchanged.
func TestComponentLevelWriter_PassesAtOrAbove(t *testing.T) {
	var buf bytes.Buffer
	delegate := logging.NoopLevelWriter(&buf)

	var lvl atomic.Int32
	lvl.Store(int32(zerolog.InfoLevel))

	w := logging.NewComponentLevelWriter(&lvl, delegate)
	msg := []byte(`{"level":"warn","msg":"hi"}`)
	_, err := w.WriteLevel(zerolog.WarnLevel, msg)
	require.NoError(t, err)
	assert.Equal(t, msg, buf.Bytes())
}

// TestComponentLevelWriter_AtomicUpdate verifies that updating the atomic level is
// immediately reflected in subsequent writes without re-creating the writer.
func TestComponentLevelWriter_AtomicUpdate(t *testing.T) {
	var buf bytes.Buffer
	delegate := logging.NoopLevelWriter(&buf)

	var lvl atomic.Int32
	lvl.Store(int32(zerolog.InfoLevel))

	w := logging.NewComponentLevelWriter(&lvl, delegate)

	// debug blocked at info
	_, _ = w.WriteLevel(zerolog.DebugLevel, []byte(`{"level":"debug"}`))
	assert.Equal(t, 0, buf.Len())

	// lower to debug
	lvl.Store(int32(zerolog.DebugLevel))
	msg := []byte(`{"level":"debug","msg":"now visible"}`)
	_, err := w.WriteLevel(zerolog.DebugLevel, msg)
	require.NoError(t, err)
	assert.Equal(t, msg, buf.Bytes())
}
```

**Step 2: Run to confirm it fails**

```
go test github.com/onflow/flow-go/utils/logging -run TestComponentLevelWriter -v
```

Expected: compilation error — `logging.NewComponentLevelWriter` and `logging.NoopLevelWriter`
do not exist yet.

**Step 3: Implement**

```go
// utils/logging/component_writer.go
package logging

import (
	"io"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// componentLevelWriter is a zerolog.LevelWriter that filters log events below a dynamically
// controlled level. The level is read atomically on every write, enabling lock-free updates
// from admin commands without any coordination with writers.
//
// All exported methods are safe for concurrent access.
type componentLevelWriter struct {
	level    *atomic.Int32
	delegate zerolog.LevelWriter
}

// NewComponentLevelWriter returns a zerolog.LevelWriter that forwards events to delegate only
// when the event level is at or above the value stored in level.
func NewComponentLevelWriter(level *atomic.Int32, delegate zerolog.LevelWriter) zerolog.LevelWriter {
	return &componentLevelWriter{level: level, delegate: delegate}
}

// Write forwards p to the delegate unconditionally. This path is only taken by callers that
// bypass zerolog's level-aware dispatch; in normal zerolog usage WriteLevel is called instead.
//
// No error returns are expected during normal operation.
func (w *componentLevelWriter) Write(p []byte) (int, error) {
	return w.delegate.Write(p)
}

// WriteLevel forwards p to the delegate if l is at or above the configured level. If the
// event is filtered, WriteLevel returns (len(p), nil) without writing — matching zerolog's
// discard convention.
//
// No error returns are expected during normal operation.
func (w *componentLevelWriter) WriteLevel(l zerolog.Level, p []byte) (int, error) {
	if l < zerolog.Level(w.level.Load()) {
		return len(p), nil
	}
	return w.delegate.WriteLevel(l, p)
}

// NoopLevelWriter wraps an io.Writer as a zerolog.LevelWriter, ignoring the level argument.
// Intended for tests.
func NoopLevelWriter(w io.Writer) zerolog.LevelWriter {
	return &noopLevelWriter{w}
}

type noopLevelWriter struct{ io.Writer }

func (n *noopLevelWriter) WriteLevel(_ zerolog.Level, p []byte) (int, error) {
	return n.Write(p)
}
```

**Step 4: Run tests**

```
go test github.com/onflow/flow-go/utils/logging -run TestComponentLevelWriter -v
```

Expected: all three pass.

**Step 5: Commit**

```
git add utils/logging/component_writer.go utils/logging/component_writer_test.go
git commit -m "add componentLevelWriter for per-component log filtering"
```

---

## Task 2: `LogRegistry` — registration and level resolution

The core registry: `Register`, resolution priority, and `zerolog.GlobalLevel` maintenance.

**Files:**
- Create: `utils/logging/registry.go`
- Create: `utils/logging/registry_test.go`

**Step 1: Write the failing tests**

```go
// utils/logging/registry_test.go
package logging_test

import (
	"os"
	"sync/atomic"
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
	// should compile and be usable
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
```

**Step 2: Run to confirm failure**

```
go test github.com/onflow/flow-go/utils/logging -run TestLogRegistry -v
```

Expected: compilation error — `logging.NewLogRegistry` and `logging.LogRegistry` do not exist.

**Step 3: Implement**

```go
// utils/logging/registry.go
package logging

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// LogRegistry manages per-component log levels. Components register by calling Logger with a
// unique ID; the returned zerolog.Logger is backed by a componentLevelWriter whose level the
// registry controls. All registered loggers derived from a component logger via With() share the
// same componentLevelWriter and automatically reflect level changes.
//
// The registry owns zerolog.SetGlobalLevel, maintaining it as the minimum of all configured
// component levels to preserve zerolog's pre-creation event optimisation.
//
// All exported methods are safe for concurrent access.
type LogRegistry struct {
	mu sync.RWMutex

	baseLogger    zerolog.Logger
	baseWriter    zerolog.LevelWriter
	globalDefault zerolog.Level

	staticConfig map[string]zerolog.Level // from CLI flag; never mutates after construction
	overrides    map[string]zerolog.Level // from admin commands; cleared on reset

	registered map[string]*atomic.Int32 // componentID → current effective level
}

// NewLogRegistry constructs a LogRegistry. baseLogger carries node-level context fields
// (node_role, node_id, etc.) that are inherited by all component loggers. baseWriter is the
// underlying output; it is wrapped per component. staticConfig is the parsed --component-log-levels
// flag; it must not be mutated after construction.
func NewLogRegistry(
	baseLogger zerolog.Logger,
	baseWriter io.Writer,
	globalDefault zerolog.Level,
	staticConfig map[string]zerolog.Level,
) *LogRegistry {
	static := make(map[string]zerolog.Level, len(staticConfig))
	for k, v := range staticConfig {
		static[k] = v
	}
	r := &LogRegistry{
		baseLogger:    baseLogger.Level(zerolog.TraceLevel),
		baseWriter:    toLevelWriter(baseWriter),
		globalDefault: globalDefault,
		staticConfig:  static,
		overrides:     make(map[string]zerolog.Level),
		registered:    make(map[string]*atomic.Int32),
	}
	zerolog.SetGlobalLevel(globalDefault)
	return r
}

// Logger registers componentID and returns a zerolog.Logger backed by a componentLevelWriter
// for that component. The logger inherits the base logger's context fields.
// Panics if componentID is already registered — duplicate registration is a programming error.
func (r *LogRegistry) Logger(componentID string) zerolog.Logger {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.registered[componentID]; exists {
		panic(fmt.Sprintf("log registry: component %q already registered", componentID))
	}

	level := r.resolve(componentID)
	atomicLevel := &atomic.Int32{}
	atomicLevel.Store(int32(level))
	r.registered[componentID] = atomicLevel

	w := NewComponentLevelWriter(atomicLevel, r.baseWriter)
	r.updateGlobalLevel()
	return r.baseLogger.Output(w)
}

// EffectiveLevel returns the current effective log level for a registered component.
// Intended for inspection and testing; returns zerolog.Disabled if not registered.
func (r *LogRegistry) EffectiveLevel(componentID string) zerolog.Level {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if al, ok := r.registered[componentID]; ok {
		return zerolog.Level(al.Load())
	}
	return zerolog.Disabled
}

// resolve returns the effective level for componentID using the priority order:
// override exact > override wildcard (most specific) > static exact > static wildcard > globalDefault.
// Must be called with r.mu held.
func (r *LogRegistry) resolve(componentID string) zerolog.Level {
	if level, ok := r.overrides[componentID]; ok {
		return level
	}
	if level, ok := bestWildcardMatch(r.overrides, componentID); ok {
		return level
	}
	if level, ok := r.staticConfig[componentID]; ok {
		return level
	}
	if level, ok := bestWildcardMatch(r.staticConfig, componentID); ok {
		return level
	}
	return r.globalDefault
}

// updateGlobalLevel sets zerolog.GlobalLevel to min(globalDefault, all component levels).
// Must be called with r.mu held.
func (r *LogRegistry) updateGlobalLevel() {
	min := r.globalDefault
	for _, al := range r.registered {
		if l := zerolog.Level(al.Load()); l < min {
			min = l
		}
	}
	zerolog.SetGlobalLevel(min)
}

// bestWildcardMatch finds the most specific "prefix.*" wildcard in config that matches id.
// Returns the level and true if found, zero and false otherwise.
func bestWildcardMatch(config map[string]zerolog.Level, id string) (zerolog.Level, bool) {
	var bestPrefix string
	var bestLevel zerolog.Level
	found := false
	for pattern, level := range config {
		if !strings.HasSuffix(pattern, ".*") {
			continue
		}
		prefix := strings.TrimSuffix(pattern, ".*")
		if strings.HasPrefix(id, prefix+".") && len(prefix) > len(bestPrefix) {
			bestPrefix = prefix
			bestLevel = level
			found = true
		}
	}
	return bestLevel, found
}

// toLevelWriter wraps w as a zerolog.LevelWriter if it does not already implement the interface.
func toLevelWriter(w io.Writer) zerolog.LevelWriter {
	if lw, ok := w.(zerolog.LevelWriter); ok {
		return lw
	}
	return NoopLevelWriter(w)
}
```

**Step 4: Run tests**

```
go test github.com/onflow/flow-go/utils/logging -run TestLogRegistry -v
```

Expected: all pass.

**Step 5: Commit**

```
git add utils/logging/registry.go utils/logging/registry_test.go
git commit -m "add LogRegistry with component-level writer and resolution priority"
```

---

## Task 3: `LogRegistry` — `SetLevel`, `Reset`, `SetDefaultLevel`, `Levels`

The mutation methods used by admin commands.

**Files:**
- Modify: `utils/logging/registry.go`
- Modify: `utils/logging/registry_test.go`

**Step 1: Write the failing tests**

Add to `utils/logging/registry_test.go`:

```go
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

	r.SetLevel("hotstuff.voter", zerolog.WarnLevel)  // exact override first
	r.SetLevel("hotstuff.*", zerolog.DebugLevel)     // wildcard applied second

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
```

**Step 2: Run to confirm failure**

```
go test github.com/onflow/flow-go/utils/logging -run "TestLogRegistry_Set|TestLogRegistry_Reset|TestLogRegistry_Default|TestLogRegistry_Levels" -v
```

Expected: compilation errors for `SetLevel`, `Reset`, `SetDefaultLevel`, `Levels`, `LevelSource*`.

**Step 3: Implement — add to `utils/logging/registry.go`**

Add these types and methods:

```go
// LevelSource describes where a component's effective level originated.
type LevelSource string

const (
	// LevelSourceOverride indicates the level was set by an admin command (exact pattern).
	LevelSourceOverride LevelSource = "override"
	// LevelSourceOverrideWildcard indicates the level was set by an admin command (wildcard pattern).
	LevelSourceOverrideWildcard LevelSource = "override-wildcard"
	// LevelSourceStatic indicates the level was set by the --component-log-levels CLI flag (exact).
	LevelSourceStatic LevelSource = "static"
	// LevelSourceStaticWildcard indicates the level was set by the --component-log-levels CLI flag (wildcard).
	LevelSourceStaticWildcard LevelSource = "static-wildcard"
	// LevelSourceDefault indicates the level falls back to the global default.
	LevelSourceDefault LevelSource = "default"
)

// ComponentLevel holds the effective log level and its source for a registered component.
type ComponentLevel struct {
	Level  zerolog.Level
	Source LevelSource
}

// SetLevel applies level to all registered components matching pattern. pattern may be an
// exact component ID or a wildcard ("prefix.*"). The new override is stored and takes effect
// immediately on all matching registered components.
//
// No error returns are expected during normal operation.
func (r *LogRegistry) SetLevel(pattern string, level zerolog.Level) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.overrides[pattern] = level
	r.applyToMatching(pattern)
	r.updateGlobalLevel()
}

// Reset removes runtime overrides matching each pattern in patterns and re-resolves affected
// components from static config and globalDefault. Passing ["*"] removes all overrides and
// resets every registered component.
//
// No error returns are expected during normal operation.
func (r *LogRegistry) Reset(patterns []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	affected := make(map[string]struct{})

	for _, pattern := range patterns {
		if pattern == "*" {
			for id := range r.registered {
				affected[id] = struct{}{}
			}
			r.overrides = make(map[string]zerolog.Level)
		} else if strings.HasSuffix(pattern, ".*") {
			prefix := strings.TrimSuffix(pattern, ".*")
			for id := range r.registered {
				if strings.HasPrefix(id, prefix+".") {
					affected[id] = struct{}{}
				}
			}
			delete(r.overrides, pattern)
		} else {
			affected[pattern] = struct{}{}
			delete(r.overrides, pattern)
		}
	}

	for id := range affected {
		if al, ok := r.registered[id]; ok {
			al.Store(int32(r.resolve(id)))
		}
	}
	r.updateGlobalLevel()
}

// SetDefaultLevel updates the global default and re-resolves all registered components that
// have no per-component override (runtime or static). Per-component overrides are preserved.
//
// No error returns are expected during normal operation.
func (r *LogRegistry) SetDefaultLevel(level zerolog.Level) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.globalDefault = level
	for id, al := range r.registered {
		al.Store(int32(r.resolve(id)))
	}
	r.updateGlobalLevel()
}

// Levels returns the current globalDefault and a snapshot of every registered component's
// effective level and source.
func (r *LogRegistry) Levels() (zerolog.Level, map[string]ComponentLevel) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]ComponentLevel, len(r.registered))
	for id := range r.registered {
		level, source := r.resolveWithSource(id)
		result[id] = ComponentLevel{Level: level, Source: source}
	}
	return r.globalDefault, result
}

// resolveWithSource is like resolve but also returns the LevelSource for reporting.
// Must be called with r.mu held.
func (r *LogRegistry) resolveWithSource(id string) (zerolog.Level, LevelSource) {
	if level, ok := r.overrides[id]; ok {
		return level, LevelSourceOverride
	}
	if level, ok := bestWildcardMatch(r.overrides, id); ok {
		return level, LevelSourceOverrideWildcard
	}
	if level, ok := r.staticConfig[id]; ok {
		return level, LevelSourceStatic
	}
	if level, ok := bestWildcardMatch(r.staticConfig, id); ok {
		return level, LevelSourceStaticWildcard
	}
	return r.globalDefault, LevelSourceDefault
}

// applyToMatching re-resolves all registered components matched by pattern.
// Must be called with r.mu held.
func (r *LogRegistry) applyToMatching(pattern string) {
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		for id, al := range r.registered {
			if strings.HasPrefix(id, prefix+".") {
				al.Store(int32(r.resolve(id)))
			}
		}
	} else {
		if al, ok := r.registered[pattern]; ok {
			al.Store(int32(r.resolve(pattern)))
		}
	}
}
```

**Step 4: Run tests**

```
go test github.com/onflow/flow-go/utils/logging/... -v
```

Expected: all pass.

**Step 5: Commit**

```
git add utils/logging/registry.go utils/logging/registry_test.go
git commit -m "add SetLevel, Reset, SetDefaultLevel, Levels to LogRegistry"
```

---

## Task 4: Parse `--component-log-levels` flag

A utility function that parses the CLI flag string into a `map[string]zerolog.Level`. Well-tested
because bad input should fail fast at startup.

**Files:**
- Create: `utils/logging/parse.go`
- Create: `utils/logging/parse_test.go`

**Step 1: Write the failing tests**

```go
// utils/logging/parse_test.go
package logging_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/logging"
)

func TestParseComponentLogLevels_Empty(t *testing.T) {
	result, err := logging.ParseComponentLogLevels("")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseComponentLogLevels_Single(t *testing.T) {
	result, err := logging.ParseComponentLogLevels("hotstuff:debug")
	require.NoError(t, err)
	assert.Equal(t, map[string]zerolog.Level{"hotstuff": zerolog.DebugLevel}, result)
}

func TestParseComponentLogLevels_Multiple(t *testing.T) {
	result, err := logging.ParseComponentLogLevels("hotstuff:debug,network.*:warn")
	require.NoError(t, err)
	assert.Equal(t, map[string]zerolog.Level{
		"hotstuff":   zerolog.DebugLevel,
		"network.*":  zerolog.WarnLevel,
	}, result)
}

func TestParseComponentLogLevels_InvalidLevel(t *testing.T) {
	_, err := logging.ParseComponentLogLevels("hotstuff:badlevel")
	require.Error(t, err)
}

func TestParseComponentLogLevels_MissingColon(t *testing.T) {
	_, err := logging.ParseComponentLogLevels("hotstuffdebug")
	require.Error(t, err)
}

func TestParseComponentLogLevels_EmptyComponent(t *testing.T) {
	_, err := logging.ParseComponentLogLevels(":debug")
	require.Error(t, err)
}
```

**Step 2: Run to confirm failure**

```
go test github.com/onflow/flow-go/utils/logging -run TestParseComponentLogLevels -v
```

**Step 3: Implement**

```go
// utils/logging/parse.go
package logging

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// ParseComponentLogLevels parses the --component-log-levels flag value into a map of
// component pattern to zerolog.Level.
//
// Format: "component:level,prefix.*:level"
// Example: "hotstuff:debug,network.*:warn"
//
// Expected error returns during normal operation:
//   - error: if any entry is malformed or contains an unrecognized level string.
func ParseComponentLogLevels(s string) (map[string]zerolog.Level, error) {
	result := make(map[string]zerolog.Level)
	if s == "" {
		return result, nil
	}
	for _, entry := range strings.Split(s, ",") {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid component log level entry %q: expected format component:level", entry)
		}
		component := strings.TrimSpace(parts[0])
		levelStr := strings.TrimSpace(parts[1])
		if component == "" {
			return nil, fmt.Errorf("invalid component log level entry %q: component name must not be empty", entry)
		}
		level, err := zerolog.ParseLevel(strings.ToLower(levelStr))
		if err != nil {
			return nil, fmt.Errorf("invalid log level %q for component %q: %w", levelStr, component, err)
		}
		result[component] = level
	}
	return result, nil
}
```

**Step 4: Run tests**

```
go test github.com/onflow/flow-go/utils/logging -run TestParseComponentLogLevels -v
```

**Step 5: Commit**

```
git add utils/logging/parse.go utils/logging/parse_test.go
git commit -m "add ParseComponentLogLevels for CLI flag parsing"
```

---

## Task 5: `get-component-log-levels` admin command

Read-only command: returns a snapshot of all registered components and their levels.

**Files:**
- Create: `admin/commands/common/get_component_log_levels.go`

**Step 1: Implement** (no test file needed — integration is tested via the registry)

```go
// admin/commands/common/get_component_log_levels.go
package common

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/utils/logging"
)

var _ commands.AdminCommand = (*GetComponentLogLevelsCommand)(nil)

// GetComponentLogLevelsCommand returns the current log level for every registered component
// and the global default.
type GetComponentLogLevelsCommand struct {
	registry *logging.LogRegistry
}

// NewGetComponentLogLevelsCommand constructs a GetComponentLogLevelsCommand.
func NewGetComponentLogLevelsCommand(registry *logging.LogRegistry) *GetComponentLogLevelsCommand {
	return &GetComponentLogLevelsCommand{registry: registry}
}

// Validator performs no validation — this command takes no input.
//
// No error returns are expected during normal operation.
func (g *GetComponentLogLevelsCommand) Validator(_ *admin.CommandRequest) error {
	return nil
}

// Handler returns a snapshot of all registered component log levels and the global default.
//
// No error returns are expected during normal operation.
func (g *GetComponentLogLevelsCommand) Handler(_ context.Context, _ *admin.CommandRequest) (interface{}, error) {
	defaultLevel, levels := g.registry.Levels()

	components := make(map[string]interface{}, len(levels))
	for id, cl := range levels {
		components[id] = map[string]string{
			"level":  zerolog.Level(cl.Level).String(),
			"source": string(cl.Source),
		}
	}

	return map[string]interface{}{
		"default":    defaultLevel.String(),
		"components": components,
	}, nil
}
```

**Step 2: Build check**

```
go build github.com/onflow/flow-go/admin/commands/common/...
```

**Step 3: Commit**

```
git add admin/commands/common/get_component_log_levels.go
git commit -m "add get-component-log-levels admin command"
```

---

## Task 6: `set-component-log-level` admin command

**Files:**
- Create: `admin/commands/common/set_component_log_level.go`

**Step 1: Implement**

```go
// admin/commands/common/set_component_log_level.go
package common

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/utils/logging"
)

var _ commands.AdminCommand = (*SetComponentLogLevelCommand)(nil)

// SetComponentLogLevelCommand sets the log level for one or more components identified by
// exact or wildcard patterns. Input is a JSON object mapping pattern to level string.
//
// Example input:
//
//	{"hotstuff.voter": "debug", "network.*": "warn"}
type SetComponentLogLevelCommand struct {
	registry *logging.LogRegistry
}

// NewSetComponentLogLevelCommand constructs a SetComponentLogLevelCommand.
func NewSetComponentLogLevelCommand(registry *logging.LogRegistry) *SetComponentLogLevelCommand {
	return &SetComponentLogLevelCommand{registry: registry}
}

type parsedComponentLevel struct {
	pattern string
	level   zerolog.Level
}

// Validator validates that the input is a non-empty map of pattern → level string with
// recognisable level values.
//
// Returns [admin.InvalidAdminReqError] for invalid or malformed requests.
func (s *SetComponentLogLevelCommand) Validator(req *admin.CommandRequest) error {
	raw, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("input must be a JSON object mapping component pattern to level string")
	}
	if len(raw) == 0 {
		return admin.NewInvalidAdminReqFormatError("input must not be empty")
	}

	parsed := make([]parsedComponentLevel, 0, len(raw))
	for pattern, val := range raw {
		levelStr, ok := val.(string)
		if !ok {
			return admin.NewInvalidAdminReqErrorf("level for %q must be a string", pattern)
		}
		level, err := zerolog.ParseLevel(levelStr)
		if err != nil {
			return admin.NewInvalidAdminReqErrorf("invalid level %q for component %q: %w", levelStr, pattern, err)
		}
		parsed = append(parsed, parsedComponentLevel{pattern: pattern, level: level})
	}

	req.ValidatorData = parsed
	return nil
}

// Handler applies the validated component level overrides and returns the updated levels.
//
// No error returns are expected during normal operation.
func (s *SetComponentLogLevelCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	entries := req.ValidatorData.([]parsedComponentLevel)

	result := make(map[string]string, len(entries))
	for _, e := range entries {
		s.registry.SetLevel(e.pattern, e.level)
		result[e.pattern] = fmt.Sprintf("set to %s", e.level)
	}
	return result, nil
}
```

**Step 2: Build check**

```
go build github.com/onflow/flow-go/admin/commands/common/...
```

**Step 3: Commit**

```
git add admin/commands/common/set_component_log_level.go
git commit -m "add set-component-log-level admin command"
```

---

## Task 7: `reset-component-log-level` admin command

**Files:**
- Create: `admin/commands/common/reset_component_log_level.go`

**Step 1: Implement**

```go
// admin/commands/common/reset_component_log_level.go
package common

import (
	"context"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/utils/logging"
)

var _ commands.AdminCommand = (*ResetComponentLogLevelCommand)(nil)

// ResetComponentLogLevelCommand removes runtime log level overrides for components matching
// the specified patterns, restoring them to static config or global default.
//
// Input is a JSON array of patterns. ["*"] resets all registered components.
// "*" may not be mixed with other patterns.
//
// Example input:
//
//	["hotstuff.voter", "hotstuff.*"]
//	["*"]
type ResetComponentLogLevelCommand struct {
	registry *logging.LogRegistry
}

// NewResetComponentLogLevelCommand constructs a ResetComponentLogLevelCommand.
func NewResetComponentLogLevelCommand(registry *logging.LogRegistry) *ResetComponentLogLevelCommand {
	return &ResetComponentLogLevelCommand{registry: registry}
}

// Validator validates that the input is a non-empty array of pattern strings. "*" must be
// the sole element if present.
//
// Returns [admin.InvalidAdminReqError] for invalid or malformed requests.
func (r *ResetComponentLogLevelCommand) Validator(req *admin.CommandRequest) error {
	raw, ok := req.Data.([]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("input must be a JSON array of pattern strings")
	}
	if len(raw) == 0 {
		return admin.NewInvalidAdminReqFormatError("input must not be empty")
	}

	patterns := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			return admin.NewInvalidAdminReqFormatError("each element must be a string")
		}
		patterns = append(patterns, s)
	}

	for _, p := range patterns {
		if p == "*" && len(patterns) > 1 {
			return admin.NewInvalidAdminReqErrorf("\"*\" must be the only element when resetting all components")
		}
	}

	req.ValidatorData = patterns
	return nil
}

// Handler removes the specified runtime overrides and returns the restored levels.
//
// No error returns are expected during normal operation.
func (r *ResetComponentLogLevelCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	patterns := req.ValidatorData.([]string)
	r.registry.Reset(patterns)
	return "ok", nil
}
```

**Step 2: Build check**

```
go build github.com/onflow/flow-go/admin/commands/common/...
```

**Step 3: Commit**

```
git add admin/commands/common/reset_component_log_level.go
git commit -m "add reset-component-log-level admin command"
```

---

## Task 8: Update `set-log-level` command to use registry

Currently calls `zerolog.SetGlobalLevel` directly. Update it to delegate to `registry.SetDefaultLevel`.

**Files:**
- Modify: `admin/commands/common/set_log_level.go`

**Step 1: Implement** — replace the file contents:

```go
// admin/commands/common/set_log_level.go
package common

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/utils/logging"
)

var _ commands.AdminCommand = (*SetLogLevelCommand)(nil)

// SetLogLevelCommand sets the global default log level. Components with per-component overrides
// (from the CLI flag or a prior set-component-log-level command) are unaffected.
type SetLogLevelCommand struct {
	registry *logging.LogRegistry
}

// NewSetLogLevelCommand constructs a SetLogLevelCommand.
func NewSetLogLevelCommand(registry *logging.LogRegistry) *SetLogLevelCommand {
	return &SetLogLevelCommand{registry: registry}
}

// Handler sets the global default level via the registry.
//
// No error returns are expected during normal operation.
func (s *SetLogLevelCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	level := req.ValidatorData.(zerolog.Level)
	s.registry.SetDefaultLevel(level)
	log.Info().Msgf("changed default log level to %v", level)
	return "ok", nil
}

// Validator validates the request.
//
// Returns [admin.InvalidAdminReqError] for invalid or malformed requests.
func (s *SetLogLevelCommand) Validator(req *admin.CommandRequest) error {
	level, ok := req.Data.(string)
	if !ok {
		return admin.NewInvalidAdminReqFormatError("the input must be a string")
	}
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return admin.NewInvalidAdminReqErrorf("failed to parse level: %w", err)
	}
	req.ValidatorData = logLevel
	return nil
}
```

**Step 2: Build check**

```
go build github.com/onflow/flow-go/admin/commands/common/...
```

**Step 3: Commit**

```
git add admin/commands/common/set_log_level.go
git commit -m "update set-log-level to delegate to LogRegistry.SetDefaultLevel"
```

---

## Task 9: Wire `LogRegistry` into `NodeConfig` and `initLogger`

**Files:**
- Modify: `cmd/node_builder.go` (lines ~160-190, ~195-241, ~260-290)
- Modify: `cmd/scaffold.go` (lines ~887-921, ~2016-2038)

**Step 1: Add fields to `BaseConfig` and `NodeConfig` in `cmd/node_builder.go`**

In `BaseConfig` (after the existing `level` field at line ~161), add:
```go
componentLogLevels string // raw --component-log-levels flag value
```

In `NodeConfig` (after `Logger zerolog.Logger` at line ~198), add:
```go
LogRegistry *utilslogging.LogRegistry
```

Add the import at the top of node_builder.go:
```go
utilslogging "github.com/onflow/flow-go/utils/logging"
```

**Step 2: Register the CLI flag**

Find where `--loglevel` flag is registered in `cmd/scaffold.go` (search for `"loglevel"`). Add
the new flag immediately after it:

```go
flags.StringVarP(&fnb.BaseConfig.componentLogLevels, "component-log-levels", "", "",
    `Per-component log levels. Format: "component:level,prefix.*:level". `+
    `Example: "hotstuff:debug,network.*:warn"`)
```

**Step 3: Update `initLogger` in `cmd/scaffold.go`**

Replace the current `initLogger` body with:

```go
func (fnb *FlowNodeBuilder) initLogger() error {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }

	throttledSampler := logging.BurstSampler(fnb.BaseConfig.debugLogLimit, time.Second)

	log := fnb.Logger.With().
		Timestamp().
		Str("node_role", fnb.BaseConfig.NodeRole).
		Str("node_id", fnb.NodeID.String()).
		Logger().
		Sample(zerolog.LevelSampler{
			TraceSampler: throttledSampler,
			DebugSampler: throttledSampler,
		})

	log.Info().Msgf("flow %s node starting up", fnb.BaseConfig.NodeRole)

	lvl, err := zerolog.ParseLevel(strings.ToLower(fnb.BaseConfig.level))
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	staticConfig, err := utilslogging.ParseComponentLogLevels(fnb.BaseConfig.componentLogLevels)
	if err != nil {
		return fmt.Errorf("invalid --component-log-levels: %w", err)
	}

	// Logger level is set to TraceLevel; all filtering is handled by LogRegistry.
	log = log.Level(zerolog.TraceLevel)

	fnb.LogRegistry = utilslogging.NewLogRegistry(log, os.Stderr, lvl, staticConfig)
	fnb.Logger = log

	return nil
}
```

Note: `fnb.LogRegistry` requires `FlowNodeBuilder` to embed or reference `NodeConfig`. Check
if `fnb.NodeConfig.LogRegistry` is the correct accessor and use accordingly.

**Step 4: Update `RegisterDefaultAdminCommands` in `cmd/scaffold.go`**

```go
func (fnb *FlowNodeBuilder) RegisterDefaultAdminCommands() {
	fnb.AdminCommand("set-log-level", func(config *NodeConfig) commands.AdminCommand {
		return common.NewSetLogLevelCommand(config.LogRegistry)
	}).AdminCommand("set-golog-level", func(config *NodeConfig) commands.AdminCommand {
		return &common.SetGologLevelCommand{}
	}).AdminCommand("get-config", func(config *NodeConfig) commands.AdminCommand {
		return common.NewGetConfigCommand(config.ConfigManager)
	}).AdminCommand("set-config", func(config *NodeConfig) commands.AdminCommand {
		return common.NewSetConfigCommand(config.ConfigManager)
	}).AdminCommand("list-configs", func(config *NodeConfig) commands.AdminCommand {
		return common.NewListConfigCommand(config.ConfigManager)
	}).AdminCommand("get-component-log-levels", func(config *NodeConfig) commands.AdminCommand {
		return common.NewGetComponentLogLevelsCommand(config.LogRegistry)
	}).AdminCommand("set-component-log-level", func(config *NodeConfig) commands.AdminCommand {
		return common.NewSetComponentLogLevelCommand(config.LogRegistry)
	}).AdminCommand("reset-component-log-level", func(config *NodeConfig) commands.AdminCommand {
		return common.NewResetComponentLogLevelCommand(config.LogRegistry)
	}).AdminCommand("read-blocks", func(config *NodeConfig) commands.AdminCommand {
		return storageCommands.NewReadBlocksCommand(config.State, config.Storage.Blocks)
	}).AdminCommand("read-range-blocks", func(conf *NodeConfig) commands.AdminCommand {
		return storageCommands.NewReadRangeBlocksCommand(conf.Storage.Blocks)
	}).AdminCommand("read-results", func(config *NodeConfig) commands.AdminCommand {
		return storageCommands.NewReadResultsCommand(config.State, config.Storage.Results)
	}).AdminCommand("read-seals", func(config *NodeConfig) commands.AdminCommand {
		return storageCommands.NewReadSealsCommand(config.State, config.Storage.Seals, config.Storage.Index)
	}).AdminCommand("get-latest-identity", func(config *NodeConfig) commands.AdminCommand {
		return common.NewGetIdentityCommand(config.IdentityProvider)
	})
}
```

**Step 5: Build check**

```
go build github.com/onflow/flow-go/cmd/...
```

Fix any compilation errors (import paths, field access). The build must be clean before committing.

**Step 6: Commit**

```
git add cmd/node_builder.go cmd/scaffold.go
git commit -m "wire LogRegistry into NodeConfig, initLogger, and admin commands"
```

---

## Task 10: Full test run

**Step 1: Run all logging utility tests**

```
go test github.com/onflow/flow-go/utils/logging/... -v
```

Expected: all pass.

**Step 2: Run all admin command tests**

```
go test github.com/onflow/flow-go/admin/... -v
```

Expected: all pass (or pre-existing failures only — do not introduce new failures).

**Step 3: Run changed-file linter**

```
make fix-lint-new
```

Fix any issues reported.

**Step 4: Final build check across affected packages**

```
go build github.com/onflow/flow-go/utils/logging/... github.com/onflow/flow-go/admin/... github.com/onflow/flow-go/cmd/...
```

**Step 5: Commit lint fixes if any**

```
git add -u
git commit -m "fix lint issues in dynamic log level implementation"
```
