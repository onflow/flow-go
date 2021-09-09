# Coding Style Guidelines for the Flow core-protocol

Below, we discuss code-style conventions for the Flow core-protocol implementation. 
They are _guidelines_ with the goal to increase readability and maintainability of the code base.

We believe that sticking to these guidelines will increase code quality and reduce bugs in many situations.
In some cases, you will find that applying our guidelines does not provide an immediate benefit for the specific code you are working on.
Nevertheless, uniform conventions and sticking to established design patterns often makes it easier for others to work with your code. 

#### They are guidelines, not a law 
Coding guidelines are tools: they are designed to be widely useful but generally have limitations as well.
If you find yourself in a situation where you feel that sticking to a guideline would have significant negative effects on a piece of code
(e.g. bloats it significantly, makes it highly error-prone) you have the freedom to break with the conventions.  
  In this case, please include a comment in the code with a brief motivation for your decision.
This will help reviewers to understand and prevent others from going through the same learning process that led to your decission to break with the conventions.     

## Error handling

This is a highly-compressed summary of [Error Handling in Flow (Notion)](https://www.notion.so/dapperlabs/Error-Handling-in-Flow-a7692d60681c4063b48851c0dbc6ed10)

A blockchain is a security-first system. Therefore, encountering a problem and continuing on a "best effort" basis is *not* an option.
There is a clear definition of the happy path (i.e. the block must be in storage). 
Any deviation of this happy path is either
1. a protocol violation (from a different node), which itself is clearly defined
2. is an uncovered edge case (which could be exploited in the worst case to compromise the system)
3. a corrupted internal state of the node

### guidelines
* avoid generic errors, such as
  ```golang
  return fmt.Errorf("x failed")
  ``` 
* Use [sentinel errors](https://pkg.go.dev/errors#New): 
  ```golang
  XFailedErr := errors.New("x failed")
  
  // foo does abc. 
  // Expected error returns during normal operations:
  //  * XFailedErr: if x failed
  func foo() err {
     ...
     return fmt.Errorf("details about failure: %w", XFailedErr)
  }
  ```
   - If some operation _can_ fail, create specific sentinel errors. 
   - **Document the types of _all_ errors that are expected during normal operation in goDoc**.
     This enables the calling code to specifically check for these errors, and decide how to handle them.
* Errors are bubbled up the call stack and wrapped to create a meaningful trace:
  ```golang
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
* Handle errors at a level, where you still have enough context to decide whether the error is expected during normal operations.
* Errors of unexpected types are indicators that the node's internal state might be corrupted. 

### anti-pattern
Continuing on a best-effort basis is not an option, i.e. the following is an anti-pattern in the context of Flow:
```golang
 err := foo()
 if err != nil {
    log.Error().Err(err).Msg("foo failed")
    return 
 }
```
There are _rare_ instances, where this might be acceptable. For example, when attempting to send a message via the network to another node and the networking layer errored.
In this case, please include a comment with an explanation, why it is acceptable to keep going, even if the other component has an internally corrupted state.   

### prioritize safety over liveness

**Ideally, a component should restart** (from a known good state), **when it encounters an unexpected error.**
**If this is out of scope, we _prioritize safety over liveness_. This means that we rather crash the node than continue on a best-effort basis.**

When in doubt, use the following as a fall-back:
```golang
 err := foo()
 if err != nil {
    if errors.Is(err, XFailedErr) {
        // expected error
    	return
    }
    log.Fatal().Err(err).Msg("unexpected internal error")
    return 
 }
```

The error-handling guidelines are set up such that they minimize technical debt, even if we don't support component restarts right away.
* The code is already differentiating expected from unexpected errors in the core business logic and have documented them accordingly
* The code handles the expected errors at the appropriate level, only propagating unexpected errors. 

As a first step, you can crash the node when an unexpected error hits the `Engine` level (for more details, see `Engine` section of this doc).
If you have followed the error handling guidelines, leaving the restart logic as technical debt for later hopefully does not add much overhead.  
  In contrast, if you log errors and continue on a best-effort basis, 
you also leave the _entire_ logic for differentiating errors as tech debt. When adding this logic later, you need to revisit the _entire_ business logic again.    


## Engines

On a high level, a Flow node consists of various data-processing components that receive information
from node-internal and external sources. We model the data flow inside a Flow node as a graph:
a data-processing component forms a vertex; two components exchanging messages is represented as an edge.    
An `Engine` is the interface for a data-processing component. 


Generally, we consider node-internal components as _trusted_ and external nodes as untrusted data sources. 
The `Engine` API differentiates between trusted and untrusted inputs:
* **Trusted inputs** from other components within the same node are handed to the `Engine` through  
  ```golang
  // SubmitLocal submits an event originating on the local node.
  SubmitLocal(event interface{})

  // ProcessLocal processes an event originating on the local node.
  ProcessLocal(event interface{}) error
  ```
* **Untrusted inputs** from other nodes are handed to the `Engine` via
  ```golang
  // Submit submits the given event from the node with the given origin ID
  // for processing in a non-blocking manner. It returns instantly and logs
  // a potential processing error internally when done.
  Submit(channel Channel, originID flow.Identifier, event interface{})

  // Process processes the given event from the node with the given origin ID
  // in a blocking manner. It returns the potential processing error when
  // done.
  Process(channel Channel, originID flow.Identifier, event interface{}) error
  ```

Furthermore, engines offer synchronous and asynchronous processing of their inputs: 
* Methods `engine.ProcessLocal` and `engine.Process` run their logic in the caller's routine. Therefore, the core business logic can return errors, which we should capture and propagate.
* Methods `engine.SubmitLocal` and `engine.Submit` run the business logic asynchronously. (no error return)
  * Note: computationally very cheap and predominantly non-blocking computation should _still_ be executed in the calling routine. This is beneficial to avoid a cascade of go-routine launches when stacking multiple engines.

### guidelines
* In the error handling within an `Engine`, differentiate between trusted and untrusted inputs.
  For example, most engines expect specific input types. Inputs with incompatible type should result in a specific sentinel error (e.g. `InvalidMessageType`).    
  For messages that come from the network (methods `Submit` and `Process`), we expect invalid types. But if I get an invalid message from a trusted local engine
  (methods `SubmitLocal` and `ProcessLocal`), something is badly wrong (critical failure).
  ```golang
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
    }
  }
  
  func (e *engine) ProcessLocal(event interface{}) {
    err := e.process(event)
    if err != nil {
        if errors.Is(err, InvalidMessageType) {
            // this is a CRITICAL BUG
        }
    }
  }
  ```
* avoid swallowing errors. The following is generally _not_ acceptable
  ```golang
  func (e *engine) Submit(event interface{}) error {
    e.unit.Launch(func() {
        err := e.process(event)
        if err != nil {
            engine.LogError(e.log, err)
        }
    })
  }
  ```
  Instead, implement:
  ```golang
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

