# logging

Package `logging` provides per-component log level control for Flow nodes via `LogRegistry`.

## Overview

Each component registers once at startup by calling `Logger`, receiving a `zerolog.Logger`
backed by a per-component writer. The registry controls that writer's level at runtime, so
level changes take effect immediately on all loggers derived from a registered component —
including child loggers created with `.With()`.

The registry maintains `zerolog.GlobalLevel` as the minimum of all component levels to
preserve zerolog's pre-event-creation short-circuit optimisation.

## Component IDs

Component IDs are dot-separated lowercase strings, e.g. `"hotstuff"`,
`"hotstuff.voter"`, `"hotstuff.voter.timer"`. The hierarchy is a naming convention only —
the registry stores every ID as a flat key.

Wildcard patterns (`"prefix.*"`) match any ID that starts with `prefix.`, including deeply
nested ones. `"hotstuff.*"` matches `"hotstuff.voter"` and `"hotstuff.voter.timer"` but
**not** `"hotstuff"` itself.

## Level resolution priority

For each component, the effective level is resolved in this order:

1. Runtime override — exact pattern (via `SetLevel` or admin command)
2. Runtime override — most-specific wildcard
3. Static config — exact pattern (`--component-log-levels` flag)
4. Static config — most-specific wildcard
5. Global default

## Usage

```go
// Construct once at node startup.
registry := logging.NewLogRegistry(os.Stderr, zerolog.InfoLevel, staticConfig)

// Register a root component.
logger := registry.Logger(node.Logger, "hotstuff")

// Register a child, inheriting parent context fields.
child := registry.Logger(logger, "hotstuff.voter")

// Adjust levels at runtime (e.g. from an admin command).
registry.SetLevel("hotstuff.*", zerolog.DebugLevel)  // all hotstuff children → debug
registry.SetLevel("hotstuff", zerolog.WarnLevel)     // hotstuff root → warn

// Remove runtime overrides, falling back to static config or default.
registry.Reset("hotstuff.*")
registry.Reset("*")  // reset everything

// Inspect current state.
defaultLevel, components := registry.Levels()
cfg := registry.Config()
```
