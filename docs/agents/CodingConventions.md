# Coding Conventions

## High-Assurance Software Engineering Principles

Flow is a high-assurance software project where the cost of bugs that slip through can be catastrophically high. We consider all inputs to be potentially byzantine. This fundamentally shapes our approach to error handling and code correctness:

### Inversion of Default Safety Assumptions
- Traditional software engineering often assumes code paths are safe unless proven dangerous
- In Flow, we invert this: **no code path is considered safe unless explicitly proven and documented to be safe**
- The mere absence of known failure cases is NOT sufficient evidence of safety
- We require conclusive arguments for why each code path will always behave correctly

### Context-Dependent Error Classification

A critical rule in Flow's error handling is that **the same error type can be benign in one context but an exception in another**. Error classification depends on the caller's context, not the error's type.

Key principles:
- An error type alone CANNOT determine whether it's benign or an exception
- The caller's context and expectations determine the error's severity
- The same error type may be handled differently in different contexts
- Documentation *must* specify which errors are benign in which contexts

Example of context-dependent error handling, where `storage.ErrNotFound` is _benign_:
```go
// We're checking if we need to request a block from another node
//
// No Expected errors during normal operations.
func (s *Synchronizer) checkBlockExists(blockID flow.Identifier) error {
    _, err := s.storage.ByBlockID(blockID)
    if errors.Is(err, storage.ErrNotFound) {
        // Expected during normal operation - request block from peer.
        return s.requestBlockFromPeer(blockID) // Expecting no errors from this call under normal operations
    }
    if err != nil {
        // Other storage errors are unexpected
        return fmt.Errorf("unexpected storage error: %w", err)
    }
    return nil
}
```

However, in this context, the same `storage.ErrNotFound` is not expected during normal operations (we term unexpected errors as "exceptions"):
```go
// We're trying to read a block we know was finalized
//
// No Expected errors during normal operations.
func (s *State) GetFinalizedBlock(height uint64) (*flow.Block, error) {
    blockID, err := s.storage.FinalizedBlockID(height)
    if err != nil {
        return nil, fmt.Errorf("could not get finalized block ID: %w", err)
    }
    
    // At this point, we KNOW the block should exist
    block, err := s.storage.ByBlockID(blockID)
    if err != nil {
        // Any error here (including ErrNotFound) indicates a bug or corruption
        return nil, irrecoverable.NewExceptionf(
            "storage corrupted - failed to get finalized block %v: %w", 
            blockID, err)
    }
    return block, nil
}
```

### Rules for Error Classification

1. **Documentation Requirements**
   - Functions MUST document which error types are benign in their context
   - Documentation MUST explain WHY an error is considered benign
   - Absence of documentation means an error is treated as an exception

2. **Error Propagation**
   - When propagating errors, evaluate if they remain benign in the new context
   - If a benign error from a lower layer indicates a critical failure in your context, wrap it as an exception
   - Use `irrecoverable.NewExceptionf` when elevating a benign error to an exception

3. **Testing Requirements**
   - Tests MUST verify error handling in different contexts
   - Test that benign errors in one context are properly elevated to exceptions in another
   - Mock dependencies to test both benign and exceptional paths

### Error Handling Philosophy
- All errors are considered potentially fatal by default
- Only explicitly documented benign errors are safe to recover from
- For any undocumented error case, we must assume the execution state is corrupted
- Recovery from undocumented errors requires node restart from last known safe state
- This conservative approach prioritizes safety over continuous operation

Example of proper high-assurance error handling:
```go
func (e *engine) process(event interface{}) error {
    // Step 1: type checking of input
    switch v := event.(type) {
    case *ValidEvent:
        // explicitly documented safe path
        return e.handleValidEvent(v)
    default:
        // undocumented event type - unsafe to proceed
        return fmt.Errorf("unexpected event type %T: %w", event, ErrInvalidEventType)
    }
}

func (e *engine) Submit(event interface{}) {
    err := e.process(event)
    if errors.Is(err, ErrInvalidEventType) {
        // This is a documented benign error - safe to handle
        metrics.InvalidEventsCounter.Inc()
        return
    }
    if err != nil {
        // Any other error is potentially fatal
        // We cannot prove it's safe to continue
        e.log.Fatal().Err(err).Msg("potentially corrupted state - must restart")
        return
    }
}
```

## 1. Code Documentation
- Every interface must have clear documentation
- Copy and extend interface documentation in implementations
- Include clear explanations for any deviations from conventions
- Document all public functions individually
- Document error handling strategies and expected error types

Example of proper error documentation:
```go
// foo does abc.
// Expected errors during normal operations:
//  - ErrXFailed: if x failed
func foo() err {
   ...
   return fmt.Errorf("details about failure: %w", ErrXFailed)
}
```

## 2. Code Structure
- Follow the component-based architecture
- Each component must implement the `Component` interface
- Clearly differentiate between trusted (internal) and untrusted (external) inputs
- Components should have dedicated worker pools
- Proper resource management with worker limits
- Proper state management and recovery

## 3. Error Categories and Handling Philosophy

### a. Benign Errors
- Component remains fully functional despite the error
- Expected during normal operations
- Must be handled within the component
- Must be documented in the component's context
- Must be represented as typed sentinel errors
- Cannot be represented by generic/untyped errors unless explicitly documented as an optional simplification for components that solely return benign errors

Example of proper benign error handling:
```go
// Expected errors during normal operations:
//  * ErrXFailed: if x failed
func benignErrorExample() error {
    err := foo()
    if err != nil {
        return fmt.Errorf("failed to do foo: %w", err)
    }
    return nil
}
```

### b. Exceptions
- Potential symptoms of internal state corruption
- Unexpected failures that may compromise component state
- Should lead to component restart or node termination
- Strongly encouraged to wrap with context when bubbling up

Example of proper exception handling:
```go
err := foo()
if errors.Is(err, XFailedErr) {
   // expected error
   return
}
if err != nil {
   log.Fatal().Err(err).Msg("unexpected internal error")
   return
}
```

### c. Sentinel Error Requirements
- Must be properly typed
- Must be documented in GoDoc
- Must avoid generic error formats
- Must always wrap with context when bubbling up the call stack
- Must document all expected error types
- Must handle at the appropriate level where context is available
- Must use proper error wrapping for stack traces

Example of proper sentinel error definition and usage:
```go
ErrXFailed := errors.New("x failed")

// bar does ...
// Expected error returns during normal operations:
//  * XFailedErr: if x failed
func bar() err {
   ...
   err := foo()
   if err != nil {
      return fmt.Errorf("failed to do foo: %w", err)
   }
   ...
}
```

## 4. Additional Best Practices
- Prioritize safety over liveness
- Don't continue on "best-effort" basis when encountering unexpected errors
- Testing Error Handling:
  - Test both benign error cases and exceptions
  - Must verify that documented sentinel errors are returned in their specified situations
  - Must verify that unexpected errors (exceptions) from lower layers or their mocks are not misinterpreted as benign errors
  - Verify proper error propagation
  - Test component recovery from errors
  - Validate error handling in both trusted and untrusted contexts

Example of proper error handling in components:
```go
func (e *engine) process(event interface{}) error {
    switch v := event.(type) {
       ...
       default:
          return fmt.Errorf("invalid input type %T: %w", event, InvalidMessageType)
    }
}

func (e *engine) Process(chan network.Channel, originID flow.Identifier, event interface{}) error {
    err := e.process(event)
    if err != nil {
        if errors.Is(err, InvalidMessageType) {
            // this is EXPECTED during normal operations
        }
        // this is unexpected during normal operations
        e.log.Fatal().Err(err).Msg("unexpected internal error")
    }
}

func (e *engine) ProcessLocal(event interface{}) {
    err := e.process(event)
    if err != nil {
        if errors.Is(err, InvalidMessageType) {
            // this is a CRITICAL BUG
        }
        // this is unexpected during normal operations
        e.log.Fatal().Err(err).Msg("unexpected internal error")
    }
}
```

## 5. Anti-patterns to Avoid
- Don't use generic error logging without proper handling
- Don't swallow errors silently
- Don't continue execution after unexpected errors
- Don't use untyped errors unless explicitly documented as benign

Example of an anti-pattern to avoid:
```go
// DON'T DO THIS:
err := foo()
if err != nil {
   log.Error().Err(err).Msg("foo failed")
   return
}
```

Instead, implement proper error handling:
```go
func (e *engine) Submit(chan network.Channel, originID flow.Identifier, event interface{}) {
    e.unit.Launch(func() {
        err := e.process(event)
        if errors.Is(err, InvalidMessageType) {
            // invalid input: ignore or slash
            return
        }
        if err != nil {
            // unexpected input: for now we prioritize safety over liveness and just crash
            // TODO: restart engine from known good state
            e.log.Fatal().Err(err).Msg("unexpected internal error")
        }
    })
}
```

## 6. Security Considerations
- Treat all external inputs as potentially byzantine
- Handle byzantine inputs gracefully
- Prevent state corruption from malicious inputs
- Use proper error types for security-related issues 

Example of handling untrusted inputs:
```go
func (e *engine) Submit(event interface{}) {
    e.unit.Launch(func() {
        err := e.process(event)
        if errors.Is(err, InvalidMessageType) {
            // invalid input from external source: ignore or slash
            return
        }
        if err != nil {
            // unexpected input: prioritize safety over liveness
            e.log.Fatal().Err(err).Msg("unexpected internal error")
        }
    })
}
```
