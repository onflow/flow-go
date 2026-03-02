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
	maps.Copy(static, staticConfig)
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
// Returns [zerolog.Disabled] if not registered.
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
// Returns the level and true if found, zero value and false otherwise.
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
