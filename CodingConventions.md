# Coding Style Guidelines for Flow Core Protocol

Below, we discuss code-style conventions for Flow's core-protocol implementation. They are _guidelines_ with the goal to
increase readability, uniformity, and maintainability of the code base.

We believe that sticking to these guidelines will increase code quality and reduce bugs in many situations. In some
cases, you will find that applying our guidelines does not provide an immediate benefit for the specific code you are
working on. Nevertheless, uniform conventions and sticking to established design patterns often makes it easier for
others to work with your code.

#### They are guidelines, not a law

Coding guidelines are tools: they are designed to be widely useful but generally have limitations as well. If you find
yourself in a situation where you feel that sticking to a guideline would have significant negative effects on a piece
of code
(e.g. bloats it significantly, makes it highly error-prone) you have the freedom to break with the conventions. In this
case, please include a comment in the code with a brief motivation for your decision. This will help reviewers and
prevent others from going through the same learning process that led to your decision to break with the conventions.


## Motivation and Context
### A Flow Node

On a high level, a Flow node consists of various data-processing components that receive information from node-internal
and external sources. We model the data flow inside a Flow node as a graph, aka the **data flow graph**:
* a data-processing component forms a vertex;
* two components exchanging messages is represented as an edge.

Generally, an individual vertex (data processing component) has a dedicated pool of go-routines for processing its work.
Inbound data packages are appended to queue(s), and the vertex's internal worker threads process the inbound messages from the queue(s).
In this design, there is an upper limit to the CPU resources an individual vertex can consume. A vertex with `k` go-routines as workers can
at most keep `k` cores busy. The benefit of this design is that the node can remain responsive even if one (or a few) of
its data processing vertices are completely overwhelmed.

In its entirety, a node is composed of many data flow vertices, that together form the node's data flow graph. The vertices
within the node typically trust each other; while external messages are untrusted. Depending on the inputs it consumes, a vertex
must carefully differentiate between trusted and untrusted inputs.

⇒ **A vertex (data processing component within a node) implements the [`Component` interface](https://github.com/onflow/flow-go/blob/57f89d4e96259f08fe84163c91ecd32484401b45/module/component/component.go#L22)**.

For example, [`VoteAggregator`](https://github.com/onflow/flow-go/blob/3899d335d5a6ce3cbbb9fde4f7af780d6bec8c9a/consensus/hotstuff/vote_aggregator.go#L29) is
a data flow vertex within a consensus nodes, whose job it is to collect incoming votes from other consensus nodes.
Hence, the corresponding function [`AddVote(vote *model.Vote)`](https://github.com/onflow/flow-go/blob/3899d335d5a6ce3cbbb9fde4f7af780d6bec8c9a/consensus/hotstuff/vote_aggregator.go#L37)
treats all inputs as untrusted. Furthermore, `VoteAggregator` consumes auxiliary inputs
([`AddBlock(block *model.Proposal)`](https://github.com/onflow/flow-go/blob/3899d335d5a6ce3cbbb9fde4f7af780d6bec8c9a/consensus/hotstuff/vote_aggregator.go#L45),
[`InvalidBlock(block *model.Proposal)`](https://github.com/onflow/flow-go/blob/3899d335d5a6ce3cbbb9fde4f7af780d6bec8c9a/consensus/hotstuff/vote_aggregator.go#L51),
[`PruneUpToView(view uint64)`](https://github.com/onflow/flow-go/blob/3899d335d5a6ce3cbbb9fde4f7af780d6bec8c9a/consensus/hotstuff/vote_aggregator.go#L57))
from the node's main consensus logic, which is trusted.


### External Byzantine messages result in benign sentinel errors

Any input that is coming from another node is untrusted and the vertex processing should gracefully handle any arbitrarily malicious input.
Under no circumstances should a byzantine input result in the vertex's internal state being corrupted.
Per convention, failure case due to a byzantine inputs are represented by specifically-typed
[sentinel errors](https://pkg.go.dev/errors#New) (for further details, see error handling section).

Byzantine inputs are one particular case, where a function returns a 'benign error'. The critical property for an error
to be benign is that the component returning it is still fully functional, despite encountering the error condition. All
benign errors should be handled within the vertex. As part of implementing error handling, developers must consider which
error conditions are benign in the context of the current component.

### Invalid internal messages result in exceptions

Any input that is coming from another internal component is trusted by default. 
If the internal message is malformed, this should be considered a critical exception.

**Exception:** some internal components forward external messages without verification. 
For example, the `sync` engine forwards `Block` messages to the `compliance` engine without verifying that the block is valid.
In this case, validity of the forwarded `Block` is not part of the contract between the `sync` and `compliance` engines,
so an invalid forwarded `Block` should not be considered an invalid internal message.

## Error handling

This is a highly-compressed summary
of [Error Handling in Flow (Notion)](https://www.notion.so/dapperlabs/Error-Handling-in-Flow-a7692d60681c4063b48851c0dbc6ed10)

A blockchain is a security-first system. Therefore, encountering a problem and continuing on a "best effort" basis is
*not* an option. There is a clear definition of the happy path (i.e. the block must be in storage). Any deviation of this
happy path is either

1. a protocol violation (from a different node), which itself is clearly defined
2. is an uncovered edge case (which could be exploited in the worst case to compromise the system)
3. a corrupted internal state of the node

### Best Practice Guidelines

1. **Errors are part of your API:** If an error is returned by a function, the error's meaning is clearly documented in the function's GoDoc. We conceptually differentiate between the following two classes of errors:

   1. _benign error: the component returning the error is still fully functional despite the error_.  
       Benign failure cases are represented as typed sentinel errors
       ([basic errors](https://pkg.go.dev/errors#New) and [higher-level errors](https://dev.to/tigorlazuardi/go-creating-custom-error-wrapper-and-do-proper-error-equality-check-11k7)),
       so we can do type checks.  
   2. _exception: the error is a potential symptom of internal state corruption_.  
      For example, a failed sanity check. In this case, the error is most likely fatal.
      <br /><br />

   * Documentation of error returns should be part of every interface. We also encourage to copy the higher-level interface documentation to every implementation
     and possibly extend it with implementation-specific comments. In particular, it simplifies maintenance work on the implementation, when the API-level contracts
     are directly documented above the implementation.
   * Adding a new sentinel often means that the higher-level logic has to gracefully handle an additional error case, potentially triggering slashing etc.
     Therefore, changing the set of specified sentinel errors is generally considered a breaking API change. 


2. **All errors beyond the specified, benign sentinel errors are considered unexpected failures, i.e. a symptom of potential state corruption.**       
   * We employ a fundamental principle of [High Assurance Software Engineering](https://www.researchgate.net/publication/228563190_High_Assurance_Software_Development),
where we treat everything beyond the known benign errors as critical failures. In unexpected failure cases, we assume that the vertex's in-memory state has been
broken and proper functioning is no longer guaranteed. The only safe route of recovery is to restart the vertex from a previously persisted, safe state.
Per convention, a vertex should throw any unexpected exceptions using the related [irrecoverable context](https://github.com/onflow/flow-go/blob/277b6515add6136946913747efebd508f0419a25/module/irrecoverable/irrecoverable.go).
   * Many components in our BFT system can return benign errors (type (i)) and irrecoverable exceptions (type (ii))

3. **Whether a particular error is benign or an exception depends on the caller's context. Errors _cannot_ be categorized as benign or exception based on their type alone.**

   ![Error Handling](/docs/ErrorHandling.png)

   * For example, consider `storage.ErrNotFound` that could be returned by the storage lager, when querying a block by ID
   (method [`Headers.ByBlockID(flow.Identifier)`](https://github.com/onflow/flow-go/blob/a918616c7b541b772c254e7eaaae3573561e6c0a/storage/headers.go#L15-L18)).
   In many cases, `storage.ErrNotFound` is expected, for instance if a node is receiving a new block proposal and checks whether the parent has already been ingested
   or needs to be requested from a different node. In contrast, if we are querying a block that we know is already finalized and the storage returns a `storage.ErrNotFound`
   something is badly broken. 
   * Use the special `irrecoverable.exception` [error type](https://github.com/onflow/flow-go/blob/master/module/irrecoverable/exception.go#L7-L26)
     to denote an unexpected error (and strip any sentinel information from the error stack).
   
     This is for any scenario when a higher-level function is interpreting a sentinel returned from a lower-level function as an exception.
     To construct an example, lets look at our `storage.Blocks` API, which has a [`ByHeight` method](https://github.com/onflow/flow-go/blob/a918616c7b541b772c254e7eaaae3573561e6c0a/storage/blocks.go#L24-L26)
     to retrieve _finalized_ blocks by height. The following could be a hypothetical implementation:
      ```golang
      // ByHeight retrieves the finalized block for the given height.
      // From the perspective of the storage layer, the following errors are benign:
      //   - storage.ErrNotFound if no finalized block at the given height is known
      func ByHeight(height uint64) (*flow.Block, error) {
          // Step 1: retrieve the ID of the finalized block for the given height. We expect
          // `storage.ErrNotFound` during normal operations, if no block at height has been
          // finalized. We just bubble this sentinel up, as it already has the expected type.
          blockID, err := retrieveBlockIdByHeight(height)
          if err != nil {
              return nil, fmt.Errorf("could not query block by height: %w", err)
          }

          // Step 2: retrieve full block by ID. Function `retrieveBlockByID` returns
          // `storage.ErrNotFound` in case no block with the given ID is known. In other parts
          // of the code that also use `retrieveBlockByID`, this would be expected during normal
          // operations. However, here we are querying a block, which the storage layer has
          // already indexed. Failure to retrieve the block implies our storage is corrupted.
          block, err := retrieveBlockByID(blockID)
          if err != nil {
              // We cannot bubble up `storage.ErrNotFound` as this would hide this irrecoverable
              // failure behind a benign sentinel error. We use the `Exception` wrapper, which
              // also implements the error `interface` but provides no `Unwrap` method. Thereby,
              // the `err`s sentinel type is hidden from upstream type checks, and consequently 
              // classified as unexpected, i.e. irrecoverable exceptions. 
              return nil, irrecoverable.NewExceptionf("storage inconsistency, failed to" +
                        "retrieve full block for indexed and finalized block %x: %w", blockID, err)
          }
          return block, nil
      }
      ```
     Functions **may** use `irrecoverable.NewExceptionf` when:
       - they are interpreting any error returning from a 3rd party module as unexpected
       - they are reacting to an unexpected condition internal to their stack frame and returning a generic error
  
     Functions **must** usd `irrecoverable.NewExceptionf` when:
       - they are interpreting any documented sentinel error returned from a flow-go module as unexpected

     For brief illustration, let us consider some function body, in which there are multiple subsequent calls to other lower-level functions.
     In most scenarios, a particular sentinel type is either always or never expected during normal operations.  If it is expected,
     then the sentinel type should be documented. If it is consistently not expected, the error should _not_ be mentioned in the 
     function's godoc. In the absence of positive affirmation that `error` is an expected and benign sentinel, the error is to be
     treated as an irrecoverable exception. So if a sentinel type `T` is consistently not expected throughout the function's body, make 
     sure the sentinel `T` is not mentioned in the function's godoc. The latter is fully sufficient to classify `T` as an irrecoverable
     exception.   


5. _Optional Simplification for components that solely return benign errors._
   * In this case, you _can_ use untyped errors to represent benign error cases (e.g. using `fmt.Errorf`).
   * By using untyped errors, the code would be _breaking with our best practice guideline_ that benign errors should be represented as typed sentinel errors.
Therefore, whenever all returned errors are benign, please clearly document this _for each public functions individually_.
For example, a statement like the following would be sufficient:
      ```golang
      // This function errors if XYZ was not successful. All returned errors are 
      // benign, as the function is side-effect free and failures are simply a no-op.  
      ```


### Hands-on suggestions

* Avoid generic errors, such as
  ```golang
  return fmt.Errorf("x failed")
  ```
* Use [sentinel errors](https://pkg.go.dev/errors#New):
  ```golang
  ErrXFailed := errors.New("x failed")

  // foo does abc.
  // Expected error returns during normal operations:
  //  * ErrXFailed: if x failed
  func foo() err {
     ...
     return fmt.Errorf("details about failure: %w", ErrXFailed)
  }
  ```
    - If some operation _can_ fail, create specific sentinel errors.
    - **Document the types of _all_ errors that are expected during normal operation in goDoc**. This enables the
      calling code to specifically check for these errors, and decide how to handle them.
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
* Handle errors at a level, where you still have enough context to decide whether the error is expected during normal
  operations.
* Errors of unexpected types are indicators that the node's internal state might be corrupted.


### Anti-Pattern

Continuing on a best-effort basis is not an option, i.e. the following is an anti-pattern in the context of Flow:

```golang
err := foo()
if err != nil {
   log.Error().Err(err).Msg("foo failed")
   return
}
```

There are _rare_ instances, where this might be acceptable. For example, when attempting to send a message via the
network to another node and the networking layer errored. In this case, please include a comment with an explanation,
why it is acceptable to keep going, even if the other component returned an error. For this example, it is acceptable to handle the error by logging because:
* we expect transient errors from this component (networking layer) during normal operation
* the networking layer uses a 3rd party library, which does not expose specific and exhaustive sentinel errors for expected failure conditions
* either it is not critical that the message is successfully sent at all, or it will be retried later on

### Prioritize Safety Over Liveness

**Ideally, a vertex should restart** (from a known good state), **when it encounters an unexpected error.**
Per convention, a vertex should throw any unexpected exceptions using the related [irrecoverable context](https://github.com/onflow/flow-go/blob/277b6515add6136946913747efebd508f0419a25/module/irrecoverable/irrecoverable.go).

<!-- TODO(alex) expand this section -->

If this is out of scope, we _prioritize safety over liveness_. This means that we rather crash the node than continue on a best-effort basis.
When in doubt, use the following as a fall-back:
```golang
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

The error-handling guidelines are set up such that they minimize technical debt, even if we don't support component
restarts right away.

* The code is already differentiating expected from unexpected errors in the core business logic and we documented them
  accordingly.
* The code handles the expected errors at the appropriate level, only propagating unexpected errors.

As a first step, you can crash the node when an unexpected error hits the `Engine` level (for more details, see `Engine`
section of this doc). If you have followed the error handling guidelines, leaving the restart logic for the component as
technical debt for later hopefully does not add much overhead. In contrast, if you log errors and continue on a
best-effort basis, you also leave the _entire_ logic for differentiating errors as tech debt. When adding this logic
later, you need to revisit the _entire_ business logic again.


# Appendix

## Engines (interface deprecated)
We model the data flow inside a node as a 'data flow graph'. An `Engine` is the deprecated interface for a vertex within the node's data flow graph. 
New vertices should implement the [`Component` interface](https://github.com/onflow/flow-go/blob/57f89d4e96259f08fe84163c91ecd32484401b45/module/component/component.go#L22)
and throw unexpected errors using the related [irrecoverable context](https://github.com/onflow/flow-go/blob/277b6515add6136946913747efebd508f0419a25/module/irrecoverable/irrecoverable.go).

See [related tech debt issue](https://github.com/dapperlabs/flow-go/issues/6361) for more information on the deprecation.

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

* Methods `engine.ProcessLocal` and `engine.Process` run their logic in the caller's routine. Therefore, the core
  business logic can return errors, which we should capture and propagate.
* Methods `engine.SubmitLocal` and `engine.Submit` run the business logic asynchronously. (no error return)
    * Note: computationally very cheap and predominantly non-blocking computation should _still_ be executed in the
      calling routine. This is beneficial to avoid a cascade of go-routine launches when stacking multiple engines.

### Guidelines

* In the error handling within an `Engine`, differentiate between trusted and untrusted inputs. For example, most
  engines expect specific input types. Inputs with incompatible type should result in a specific sentinel error (
  e.g. `InvalidMessageType`).    
  For messages that come from the network (methods `Submit` and `Process`), we expect invalid types. But if we get an
  invalid message type from a trusted local engine:
  (methods `SubmitLocal` and `ProcessLocal`), something is broken (critical failure).
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
