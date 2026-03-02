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

	baseWriter    zerolog.LevelWriter
	globalDefault zerolog.Level

	staticConfig map[string]zerolog.Level // from CLI flag; never mutates after construction
	overrides    map[string]zerolog.Level // from admin commands; cleared on reset

	registered map[string]*atomic.Int32 // componentID → current effective level
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
	r := &LogRegistry{
		baseWriter:    toLevelWriter(baseWriter),
		globalDefault: globalDefault,
		staticConfig:  static,
		overrides:     make(map[string]zerolog.Level),
		registered:    make(map[string]*atomic.Int32),
	}
	zerolog.SetGlobalLevel(globalDefault)
	return r
}

// Logger registers componentID and returns a zerolog.Logger derived from parent,
// preserving all of parent's context fields while backing the logger with a fresh
// componentLevelWriter for this component.
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
func (r *LogRegistry) Reset(patterns []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	affected := make(map[string]struct{})

	for _, pattern := range patterns {
		pattern = NormalizePattern(pattern)
		if pattern == "*" {
			for id := range r.registered {
				affected[id] = struct{}{}
			}
			r.overrides = make(map[string]zerolog.Level)
		} else if prefix, ok := strings.CutSuffix(pattern, ".*"); ok {
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
