# Dynamic Per-Component Log Levels — Design

## Overview

Operators need the ability to control log verbosity at the granularity of individual components,
both statically at startup and dynamically via the admin server. This enables targeted debugging
without flooding the node with global debug output.

## Goals

- Set global log level via the existing `--loglevel` CLI flag and `set-log-level` admin command
- Set static per-component levels via a new `--component-log-levels` CLI flag
- Dynamically modify per-component levels via new admin commands
- Support hierarchical component IDs with wildcard patterns for bulk operations
- Allow components to opt in incrementally; non-opted components continue to use the global default

## Non-Goals

- Automatically inferring component identity from log context fields (rejected: requires JSON
  scanning on every write, fragile due to inconsistent field naming)
- Permanently running components at debug level in production (feature is designed for transient
  debugging only)

---

## Core Data Model

### `LogRegistry` (`module/logging/registry.go`)

```go
type LogRegistry struct {
    mu sync.RWMutex

    baseLogger       zerolog.Logger            // TraceLevel, carries node role/ID context
    globalDefault    zerolog.Level             // current default; replaces zerolog.SetGlobalLevel

    staticConfig     map[string]zerolog.Level  // from --component-log-levels; never mutates
    overrides        map[string]zerolog.Level  // from admin commands; cleared on reset

    registered       map[string]*atomic.Int32  // componentID → effective level
}
```

### `componentLevelWriter` (internal)

```go
type componentLevelWriter struct {
    level    *atomic.Int32       // pointer into registry.registered[componentID]
    delegate zerolog.LevelWriter // base node writer
}

func (w *componentLevelWriter) WriteLevel(l zerolog.Level, p []byte) (int, error) {
    if l < zerolog.Level(w.level.Load()) {
        return len(p), nil // discard
    }
    return w.delegate.WriteLevel(l, p)
}
```

The atomic is owned by the registry and shared with the writer. The registry updates it; the
writer reads it on every log event. No locking at write time.

### `zerolog.GlobalLevel` ownership

The registry owns `zerolog.SetGlobalLevel`. It maintains the invariant:

```
zerolog.GlobalLevel = min(globalDefault, min(all component levels))
```

This preserves zerolog's pre-creation optimisation: events below the minimum across all configured
levels are never allocated. The registry recalculates and calls `zerolog.SetGlobalLevel` whenever
any level changes. Non-registered components fall through to this global filter, same as today.

---

## Level Resolution

### Pattern syntax

- **Exact**: `"hotstuff.voter"` — matches one registered ID only
- **Prefix wildcard**: `"hotstuff.*"` — matches all IDs with prefix `"hotstuff."`;
  does NOT match `"hotstuff"` itself

### Resolution priority (most specific wins)

When resolving the effective level for a component ID:

```
1. overrides    — exact match
2. overrides    — most specific prefix wildcard match
3. staticConfig — exact match
4. staticConfig — most specific prefix wildcard match
5. globalDefault
```

"Most specific" means longest matching prefix. `"hotstuff.voter.*"` beats `"hotstuff.*"` for
the ID `"hotstuff.voter.timer"`.

### `Register(id string) zerolog.Logger`

- Panics if `id` is already registered (programming error)
- Resolves effective level using priority order above
- Creates `*atomic.Int32` initialised to resolved level
- Creates `componentLevelWriter` wrapping the base writer, holding the atomic
- Returns `baseLogger.Output(componentLevelWriter)`
- Updates `zerolog.GlobalLevel` if the new component's level is below the current minimum

### `SetLevel(pattern string, level zerolog.Level)`

```
1. Store in overrides[pattern]
2. Find affected registered IDs:
     exact pattern   → the matching registered ID (if present)
     wildcard pattern → all IDs where strings.HasPrefix(id, prefix+".")
3. For each affected ID: re-resolve full priority order, update atomic
4. Recalculate zerolog.GlobalLevel
```

Re-resolving (rather than directly writing `level`) ensures a more specific existing override is
not clobbered. Example: if `"hotstuff.voter"` is at `warn` (exact override) and
`"hotstuff.*": debug` is applied, voter stays at `warn`.

### `Reset(patterns []string)`

```
For each pattern (["*"] expands to all registered IDs + all override keys):
    1. Remove pattern from overrides
    2. Find affected registered IDs (same matching logic as SetLevel)
    3. For each: re-resolve priority order (without the removed override), update atomic
4. Recalculate zerolog.GlobalLevel
```

### `SetDefaultLevel(level zerolog.Level)`

```
1. Update globalDefault
2. Re-resolve every registered ID with no per-component override (runtime or static)
3. Recalculate zerolog.GlobalLevel
```

Called by the updated `set-log-level` admin command.

### Interaction between `set-log-level` and per-component overrides

`set-log-level` updates `globalDefault` and re-resolves components with no per-component
override. Per-component overrides (runtime or static) are preserved — changing the global level
does not silently undo targeted debug overrides.

---

## Performance

### Write-time overhead

One atomic read per log event per registered component. Approximately 3–5 ns. No locking,
no JSON parsing.

### The contagion effect

When any component is lowered below `globalDefault`, the registry lowers `zerolog.GlobalLevel`
to accommodate. This causes all components (registered and unregistered) to begin creating log
events at the lower level, even if their writer immediately discards them. Event creation involves
a pool allocation and JSON serialisation.

This overhead is acceptable because:
- The feature is designed for transient operator debugging, not steady-state production use
- When no component is below `globalDefault`, `zerolog.GlobalLevel = globalDefault` and there is
  zero overhead — identical to today

### Child logger propagation

`zerolog.Logger` is a value type, but the writer field is an interface (pointer semantics).
All loggers derived from a component logger via `.With()...Logger()` share the same
`componentLevelWriter`. A level update propagates to all derived loggers automatically, with no
changes required at child callsites.

---

## CLI Flag

Added to `BaseConfig` in `cmd/node_builder.go`:

```
--component-log-levels  string
    Per-component log levels. Format: "component:level,prefix.*:level"
    Example: "hotstuff:debug,network.*:warn"
    Valid levels: trace, debug, info, warn, error, fatal, panic
```

Parsed during `initLogger()`. Invalid patterns or level strings are a fatal startup error.
Stored as `staticConfig` in the registry — never mutated after startup.

---

## Admin Commands

### `set-component-log-level`

Input: map of pattern → level string.
```json
{"hotstuff.voter": "debug"}
{"hotstuff.*": "debug", "network": "warn"}
```

Returns: affected components and their new levels.

Validator: each pattern is a valid ID or wildcard; each level string parses to a valid
`zerolog.Level`.

---

### `reset-component-log-level`

Input: array of patterns. `["*"]` resets all registered components.
```json
["hotstuff.voter", "hotstuff.*"]
["*"]
```

`"*"` is only valid as the sole element — mixing with specific patterns is a validation error.

Returns: affected components and the levels they were restored to.

---

### `get-component-log-levels`

No input. Returns the full registry state:
```json
{
  "default": "info",
  "components": {
    "hotstuff":           {"level": "debug", "source": "override"},
    "hotstuff.voter":     {"level": "warn",  "source": "static"},
    "hotstuff.pacemaker": {"level": "info",  "source": "default"},
    "network":            {"level": "warn",  "source": "override-wildcard"}
  }
}
```

`source` values: `"override"`, `"override-wildcard"`, `"static"`, `"static-wildcard"`,
`"default"`.

---

### Updated `set-log-level`

Currently calls `zerolog.SetGlobalLevel` directly. Updated to call
`registry.SetDefaultLevel(level)`.

---

## Integration

### `NodeConfig` (cmd/node_builder.go)

New field:
```go
LogRegistry *logging.LogRegistry
```

### `BaseConfig` (cmd/node_builder.go)

New field:
```go
ComponentLogLevels string  // raw --component-log-levels flag value
```

### `initLogger()` (cmd/scaffold.go)

1. Parse `ComponentLogLevels` string into `staticConfig` map
2. Construct `LogRegistry` with `staticConfig` and `globalDefault`
3. Remove direct `zerolog.SetGlobalLevel` call — registry owns this
4. Store registry in `NodeConfig.LogRegistry`

### `RegisterDefaultAdminCommands()` (cmd/scaffold.go)

- Register `set-component-log-level`, `reset-component-log-level`, `get-component-log-levels`
- Update `set-log-level` registration to pass `NodeConfig.LogRegistry`

---

## File Layout

```
module/logging/
    registry.go          // LogRegistry, componentLevelWriter
    registry_test.go

admin/commands/common/
    set_component_log_level.go    // new
    reset_component_log_level.go  // new
    get_component_log_levels.go   // new
    set_log_level.go              // updated

cmd/
    scaffold.go      // initLogger, RegisterDefaultAdminCommands updated
    node_builder.go  // NodeConfig.LogRegistry, BaseConfig.ComponentLogLevels added
```

---

## Component Opt-In

A component replaces:
```go
// before
node.Logger.With().Str("component", "hotstuff").Logger()

// after
node.LogRegistry.Logger("hotstuff")
```

Child loggers derived via `.With()` automatically inherit the `componentLevelWriter` — no changes
needed at child callsites. Opt-in is incremental: components not yet migrated continue using the
global default via the base `node.Logger`.
