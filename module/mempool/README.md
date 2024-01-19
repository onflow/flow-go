# The `mempool` module

The `mempool` module provides mempool implementations for the Flow blockchain, which
are in-memory data structures that are tasked with storing the `flow.Entity` objects.
`flow.Entity` objects are the fundamental data model of the Flow blockchain, and
every Flow primitives such as transactions, blocks, and collections are represented
as `flow.Entity` objects.

Each mempool implementation is tasked for storing a specific type of `flow.Entity`.
As a convention, all mempools are built on top of the `stdmap.Backend` struct, which
provides a thread-safe cache implementation for storing and retrieving `flow.Entity` objects.
The primary responsibility of the `stdmap.Backend` struct is to provide thread-safety for its underlying
data model (i.e., `mempool.Backdata`) that is tasked with maintaining the actual `flow.Entity` objects.

At the moment, the `mempool` module provides two implementations for the `mempool.Backdata`:
- `backdata.Backdata`: a map implementation for storing `flow.Entity` objects using native Go `map`s.
- `herocache.Cache`: a cache implementation for storing `flow.Entity` objects, which is a heap-optimized
    cache implementation that is aims on minimizing the memory footprint of the mempool on the heap and 
    reducing the GC pressure.

Note-1: by design the `mempool.Backdata` interface is **not thread-safe**. Therefore, it is the responsibility
of the `stdmap.Backend` struct to provide thread-safety for its underlying `mempool.Backdata` implementation.

Note-2: The `herocache.Cache` implementation is several orders of magnitude faster than the `backdata.Backdata` on
high-throughput workloads. For the read or write-heavy workloads, the `herocache.Cache` implementation is recommended as
the underlying `mempool.Backdata` implementation.
