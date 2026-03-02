package logging

import (
	"fmt"
	"io"
	"maps"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
)

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

// RegistryConfig is a snapshot of the registry's configuration at a point in time.
type RegistryConfig struct {
	// Default is the global default log level applied to components with no specific override.
	Default zerolog.Level
	// StaticOverrides are patterns and levels set at startup via the --component-log-levels flag.
	StaticOverrides map[string]zerolog.Level
	// DynamicOverrides are patterns and levels set at runtime via admin commands.
	DynamicOverrides map[string]zerolog.Level
}

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
	// these fields are immutable after construction
	baseWriter   zerolog.LevelWriter      // underlying output; wrapped per component
	staticConfig map[string]zerolog.Level // from CLI flag; never mutates after construction

	// mu protects all mutable fields listed below
	mu sync.RWMutex

	globalDefault zerolog.Level            // set during construction; updated by SetDefaultLevel
	overrides     map[string]zerolog.Level // from admin commands; cleared on reset
	registered    map[string]*atomic.Int32 // componentID → current effective level
}

// NewLogRegistry constructs a LogRegistry. baseWriter is the underlying output; it is wrapped
// per component. staticConfig is the parsed --component-log-levels flag; it must not be mutated
// after construction.
func NewLogRegistry(
	baseWriter io.Writer,
	globalDefault zerolog.Level,
	staticConfig map[string]zerolog.Level,
) *LogRegistry {
	static := make(map[string]zerolog.Level, len(staticConfig))
	maps.Copy(static, staticConfig)

	zerolog.SetGlobalLevel(globalDefault)

	return &LogRegistry{
		baseWriter:    toLevelWriter(baseWriter),
		globalDefault: globalDefault,
		staticConfig:  static,
		overrides:     make(map[string]zerolog.Level),
		registered:    make(map[string]*atomic.Int32),
	}
}

// Logger registers componentID and returns a zerolog.Logger derived from parent,
// preserving all of parent's context fields while backing the logger with a fresh
// componentLevelWriter for this component.
//
// Component IDs use dot-separated segments to express hierarchy, e.g. "hotstuff",
// "hotstuff.voter", "hotstuff.voter.timer". This hierarchy is purely a naming convention
// for level targeting — the registry treats every ID as a flat key. Hierarchy only matters
// when applying wildcard patterns via [LogRegistry.SetLevel] or the --component-log-levels
// flag: the pattern "hotstuff.*" matches any ID whose prefix is "hotstuff.", such as
// "hotstuff.voter" and "hotstuff.voter.timer", but not "hotstuff" itself.
//
// Use the node's top-level logger as parent at the root level:
//
//	logger := registry.Logger(node.Logger, "hotstuff")
//
// Pass an already-enriched logger to inherit accumulated context down a hierarchy:
//
//	enriched := logger.With().Uint64("view", view).Logger()
//	child := registry.Logger(enriched, "hotstuff.voter")
//
// Panics if componentID is already registered or invalid — both are programming errors.
// componentID is normalized to lowercase before registration.
func (r *LogRegistry) Logger(parent zerolog.Logger, componentID string) zerolog.Logger {
	r.mu.Lock()
	defer r.mu.Unlock()

	componentID = NormalizePattern(componentID)
	if err := ValidateComponentID(componentID); err != nil {
		panic(fmt.Sprintf("log registry: %s", err))
	}
	if _, exists := r.registered[componentID]; exists {
		panic(fmt.Sprintf("log registry: component %q already registered", componentID))
	}

	level := r.resolve(componentID)
	atomicLevel := &atomic.Int32{}
	atomicLevel.Store(int32(level))
	r.registered[componentID] = atomicLevel

	w := NewComponentLevelWriter(atomicLevel, r.baseWriter)
	r.updateGlobalLevel()
	return parent.Output(w)
}

// EffectiveLevel returns the current effective log level for a registered component.
// Returns [zerolog.Disabled] if not registered.
// componentID is normalized to lowercase before lookup.
func (r *LogRegistry) EffectiveLevel(componentID string) zerolog.Level {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if al, ok := r.registered[NormalizePattern(componentID)]; ok {
		return zerolog.Level(al.Load())
	}
	return zerolog.Disabled
}

// updateGlobalLevel sets zerolog.GlobalLevel to min(globalDefault, all component levels).
//
// Not concurrency safe. caller must hold r.mu
func (r *LogRegistry) updateGlobalLevel() {
	min := r.globalDefault
	for _, al := range r.registered {
		if l := zerolog.Level(al.Load()); l < min {
			min = l
		}
	}
	zerolog.SetGlobalLevel(min)
}

// SetLevel applies level to all registered components matching pattern. pattern may be an
// exact component ID or a wildcard ("prefix.*"). The new override is stored and takes effect
// immediately on all matching registered components.
// pattern is normalized to lowercase before storage.
//
// No error returns are expected during normal operation.
func (r *LogRegistry) SetLevel(pattern string, level zerolog.Level) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pattern = NormalizePattern(pattern)
	r.overrides[pattern] = level
	r.applyToMatching(pattern)
	r.updateGlobalLevel()
}

// Reset removes runtime overrides matching each pattern in patterns and re-resolves affected
// components from static config and globalDefault. Passing ["*"] removes all overrides and
// resets every registered component. Each pattern is normalized to lowercase.
//
// No error returns are expected during normal operation.
func (r *LogRegistry) Reset(patterns ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, pattern := range patterns {
		pattern = NormalizePattern(pattern)
		if pattern == "*" {
			r.overrides = make(map[string]zerolog.Level)
			for id, al := range r.registered {
				al.Store(int32(r.resolve(id)))
			}
			break
		}
		delete(r.overrides, pattern)
		r.applyToMatching(pattern)
	}
	r.updateGlobalLevel()
}

// SetDefaultLevel updates the global default and re-resolves all registered components.
// Per-component overrides (runtime or static) take priority and are preserved.
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

// Config returns a snapshot of the registry's current configuration.
func (r *LogRegistry) Config() RegistryConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	static := make(map[string]zerolog.Level, len(r.staticConfig))
	maps.Copy(static, r.staticConfig)

	dynamic := make(map[string]zerolog.Level, len(r.overrides))
	maps.Copy(dynamic, r.overrides)

	return RegistryConfig{
		Default:          r.globalDefault,
		StaticOverrides:  static,
		DynamicOverrides: dynamic,
	}
}

// resolve returns the effective level for componentID using the priority order:
// override exact > override wildcard (most specific) > static exact > static wildcard > globalDefault.
//
// Not concurrency safe. caller must hold r.mu
func (r *LogRegistry) resolve(componentID string) zerolog.Level {
	level, _ := r.resolveWithSource(componentID)
	return level
}

// resolveWithSource is like resolve but also returns the LevelSource for reporting.
//
// Not concurrency safe. caller must hold r.mu
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
//
// Not concurrency safe. caller must hold r.mu
func (r *LogRegistry) applyToMatching(pattern string) {
	if prefix, ok := strings.CutSuffix(pattern, ".*"); ok {
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

// bestWildcardMatch finds the most specific "prefix.*" wildcard in config that matches id.
// Returns the level and true if found, zero value and false otherwise.
func bestWildcardMatch(config map[string]zerolog.Level, id string) (zerolog.Level, bool) {
	var bestPrefix string
	var bestLevel zerolog.Level
	found := false
	for pattern, level := range config {
		prefix, ok := strings.CutSuffix(pattern, ".*")
		if !ok {
			continue
		}
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
