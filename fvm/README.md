# Flow Virtual Machine (FVM)

The Flow Virtual Machine (FVM) augments the Cadence runtime with the domain-specific 
functionality required by the Flow protocol.

## Usage

### Quickstart

```go
import (
    "github.com/onflow/cadence/runtime"
    "github.com/dapperlabs/flow-go/fvm"
)

vm := fvm.New(runtime.NewInterpreterRuntime())

ctx := vm.NewContext()

tx := flow.NewTransactionBody().
    SetScript(`transaction { execute { log("Hello, World!") } }`)

ledger := make(fvm.MapLedger)

result, err := ctx.Invoke(fvm.Transaction(tx), ledger)
if err != nil {
  panic("fatal error during invocation!")
}

fmt.Println(result.Logs[0]) // prints "Hello, World!"
```

### Invocations

The FVM interacts with the Flow execution state by performing atomic invocations against 
a shared ledger. An `Invokable` is an operation that can be applied to a `Ledger`.

#### Invocation Types

##### Transactions (Read/Write)

An `InvokableTransaction` is an operation that mutates the ledger state.

Invokable transactions can be created from a `flow.TransactionBody`:

```go
var tx flow.TransactionBody

i := fvm.Transaction(tx)
```

Transaction invocation has the following steps:

1. Verify all transaction signatures against the current ledger state
1. Validate and increment the proposal key sequence number
1. Deduct transaction fee from the payer account
1. Invoke transaction script with provided arguments and authorizers

##### Scripts (Read-only)

An `InvokableScript` is a simple operation that reads from the ledger state.

```go
foo := fvm.Script([]byte(`fun main(): Int { return 42 }`))
```

A script can have optionally bound arguments:

```go
script := fvm.Script([]byte(`fun main(x: Int): Int { x * 2 }`))

foo := script.WithArguments(argA)
bar := script.WithArguments(argB)
```

### Contexts

Invocations are applied inside of an execution context that defines the host
functionality provided to the Cadence runtime. A `Context` instance is 
an immutable object, and any changes to a context must be made by spawning
a new child context.

```go
vm := fvm.New(runtime.NewInterpreterRuntime())

globalCtx := vm.NewContext()

// create separate contexts for different blocks
block1Ctx := globalCtx.NewChild(fvm.WithBlockHeader(block1))
block2Ctx := globalCtx.NewChild(fvm.WithBlockHeader(block2))

// contexts can be safely used in parallel
go executeBlock(block1Ctx)
go executeBlock(block2Ctx)
```

#### Context Options

TODO: document context options

## Design

The structure of the FVM package is intended to promote the following:
- **Determinism**: the VM should be consistent and predictable, producing identical results given the 
same inputs.
- **Performance**: the VM should expose functionality that allows the caller to exploit parallelism and pre-computation.
- **Flexibility**: the VM should support a variety of use cases including not only execution and verification, but also
developer tooling and network emulation.

### `Context`

A `Context` is a reusable component that simply defines the parameters of an execution environment. 
For example, all transactions within the same block would share a common `Context` configured with the relevant
block data.

Contexts can be arbitrarily nested. A child context inherits the parameters of its parent, but is free to override
any parameters that it chooses.

### `HostEnvironment`

A `HostEnvironment` is a short-lived component that is consumed by an invocation. A `HostEnvironment` is the 
interface through which the VM communicates with the Cadence runtime. The `HostEnvironment` provides information about
the current `Context` and also collects logs, events and metrics emitted from the runtime.

### Invocation Lifecycle

The diagram below illustrates the relationship between contexts, host environments and invocations. The context below
is reused for multiple invocations, each of which is supplied with a unique `HostEnvironment` instance.

![fvm](./fvm.svg)


## TODO

- [ ] Improve error handling
- [ ] Create list of all missing test cases
- [ ] Implement missing test cases
- [ ] Documentation
  - [ ] Transaction validation (signatures, sequence numbers)
  - [ ] Fee payments
  - [ ] Address generation
  - [ ] Account management
  - [ ] Bootstrapping

## Open Questions

- Currently the caller is responsible for providing the correct `Ledger` instance when performing an invocation. When
performing atomic operations in batches (i.e blocks or chunks), multiple ledgers are spawned in a hierarchy similar to 
that of multiple `Context` instances. Does it make sense for the `fvm` package to manage ledgers as well?
