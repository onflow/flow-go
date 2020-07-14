# Flow Virtual Machine (FVM)

The Flow Virtual Machine (FVM) augments the Cadence runtime with the domain-specific 
functionality required by the Flow protocol.

## Usage

### Quickstart

```go
import (
    "github.com/onflow/cadence/runtime"
    "github.com/dapperlabs/flow-go/fvm"
    "github.com/dapperlabs/flow-go/model/flow"
)

vm := fvm.New(runtime.NewInterpreterRuntime())

tx := flow.NewTransactionBody().
    SetScript(`transaction { execute { log("Hello, World!") } }`)

ctx := fvm.NewContext()
ledger := make(fvm.MapLedger)

txProc := fvm.Transaction(tx)

err := vm.Run(ctx, txProc, ledger)
if err != nil {
  panic("fatal error during transaction procedure!")
}

fmt.Println(txProc.Logs[0]) // prints "Hello, World!"
```

### Procedure

The FVM interacts with the Flow execution state by running atomic procedures against 
a shared ledger. A `Procedure` is an operation that can be applied to a `Ledger`.

#### Procedure Types

##### Transactions (Read/write)

A `TransactionProcedure` is an operation that mutates the ledger state.

A transaction procedure can be created from a `flow.TransactionBody`:

```go
var tx flow.TransactionBody

i := fvm.Transaction(tx)
```

A transaction procedure has the following steps:

1. Verify all transaction signatures against the current ledger state
1. Validate and increment the proposal key sequence number
1. Deduct transaction fee from the payer account
1. Invoke transaction script with provided arguments and authorizers

##### Scripts (Read-only)

A `ScriptProcedure` is an operation that reads from the ledger state.

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

The VM runs procedures inside of an execution context that defines the host
functionality provided to the Cadence runtime. A `Context` instance is 
an immutable object, and any changes to a context must be made by spawning
a new child context.

```go
vm := fvm.New(runtime.NewInterpreterRuntime())

globalCtx := fvm.NewContext()

// create separate contexts for different blocks
block1Ctx := fvm.NewContextFromParent(globalCtx, fvm.WithBlockHeader(block1))
block2Ctx := fvm.NewContextFromParent(globalCtx, fvm.WithBlockHeader(block2))

// contexts can be safely used in parallel
go executeBlock(vm, block1Ctx)
go executeBlock(vm, block2Ctx)
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

A `HostEnvironment` is a short-lived component that is consumed by a procedure. A `HostEnvironment` is the 
interface through which the VM communicates with the Cadence runtime. The `HostEnvironment` provides information about
the current `Context` and also collects logs, events and metrics emitted from the runtime.

### Procedure Lifecycle

The diagram below illustrates the relationship between contexts, host environments and procedures. Multiple procedures
can reuse the same context, each of which is supplied with a unique `HostEnvironment` instance.

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
  - [ ] Ledger usage

## Open Questions

- Currently the caller is responsible for providing the correct `Ledger` instance when running a procedure. When
performing atomic operations in batches (i.e blocks or chunks), multiple ledgers are spawned in a hierarchy similar to 
that of multiple `Context` instances. Does it make sense for the `fvm` package to manage ledgers as well?
