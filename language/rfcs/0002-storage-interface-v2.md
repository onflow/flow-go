
# Storage Interface v2

## Description

The new storage interface replaces storable references with Capabilities.

A Capability consist of an Account and a Path.

An Account is either a Signing Account or a Public Account.

A Path consists of a Domain and an Identifier. A Path is either a Storage Path, a Public Path, or a Private Path.

Paths and Capabilities are both value types, i.e. they are copied, and they can be stored.

## API

### Types

Signing Accounts have the type `AuthAccount`.

Public Accounts have the type `PublicAccount`.

Capabilities have the type `Capability`.

Paths have the type `Path`.

### Loading and Saving Objects

Objects can be moved into/out of storage through functions of a Signing Account (`AuthAccount`) and a Storage Path:

- `fun save(_ value: T, to: Path)`, where `T` is the type parameter for the object type:

   Moves an object into storage from memory.

   - Works with Resource or Value types.
   - Aborts if the storage slot is not empty.
   - Value types that have been saved are still accessible in memory (the value is copied).

   The path must be a Storage Path.

- `fun load<T>(from: Path): T?`, where `T` is the type parameter for the object type:

   Moves a val out of storage into memory.

   - Works with Resource or Value types.
   - Returns an optional value if the stored value has a type that is a subtype of `T`.
     The types do not have to be exactly the same.
   - Aborts if:
     - The storage slot is empty.
     - There is an active borrow on that slot.
   - The storage slot is always empty after succeeding.

   The path must be a Storage Path.

- `fun copy<T>(from: Path): T?`, where `T` is the type parameter for the value type:

   Returns a copy of a value type in storage without removing it from storage.

   - Aborts if:
     - The storage slot is empty.
     - There is an active borrow on that slot.

   The type `T` must be a value/non-resource type.

   The path must be a Storage Path.

- `fun borrow<T>(from: Path): auth &T?`, where `T` is the type parameter for the object type:

   Returns a unique reference to an object in storage without removing it from storage.
   The stored object must satisfy the required type.

   - Works for Resource or Value types.
   - Aborts if:
     - The storage slot is empty.
     - There is already an active borrow on that slot.

   The path must be a Storage Path.


### Creating and Getting Capabilities

Capabilities can be created through the `link` function of a Signing Account (`AuthAccount`):

- `fun link<T>(from source: Path, to target: Path): Capability`, where `T` is the type parameter for the object type:

   - `source`: A Public Path or a Private Path where the new capability is created.

   - `target`: A Public Path, a Private Path, or a Storage Path that leads to the object that will provide the functionality defined by this capability.

     It is not necessary for the target to lead to a valid object; the target slot could be empty, or could lead to an object which does not provide the necessary type interface.

   - `T`: A type expression that indicates. The value is only exposed trough this type, not through a subtype.

   - Aborts if the storage slot is not empty.

Existing capabilities can be accessed by path using getCapability(), defined on Signing Accounts (`AuthAccount`) and Public Accounts (`PublicAccount):

- `fun getCapability(at: Path): Capability?`

  - Aborts if:
    - For Public Accounts, if passed a Private Path or Storage Path (only Public Paths are accepted).
    - For Signing Accounts, if passed a Storage Path (only Public Paths and Private paths are accepted).

#### Checking and Borrowing Capabilities

Capabilities can be checked if they satisfy a type:

- `fun check<T>(): Bool`, where `T` is the type parameter for the object type:

   Returns true if the capability currently references an object satisfying the given type (without exceeding the type interfaces of all interim capabilities).

- `fun borrow<T>(): &T?`, where `T` is the type parameter for the object type:

   Returns a unique reference to the object targeted by the capability, provided it meets the required type interface.

   - Aborts if:
     - The targeted storage slot is empty.
     - The targeted storage slot is borrowed.
     - The requested type exceeds what is allowed by the capability (or any interim capabilities)

## Syntax

### Paths

The syntax of a Path is `/<domain>/<identifier>`.

The `<domain>` part of the Path syntax is `storage` for a Storage Path, `public` for a Public Path, or `private` for a Private Path.

The `<identifier>` part of the Path syntax is an arbitrary identifier, i.e. it does not have to be an identifier of an existing type or value.

Both `<domain>` and `<identifier>` parts of the Path syntax are static, i.e. they are not evaluated.

### Capabilities

The syntax of a Capability is `/<account>/<path>`.

The `<account>` part of the Capability syntax is an identifier and dynamic, it is considered a variable identifier, which should either have the type `AuthAccount` (authorized) or `PublicAccount` (unauthorized).

If the `<path>` part of the Capability syntax is just an identifier, then the part is dynamic and is considered a variable which should have the type `Path`. The `<path>` part of the Capability syntax may also be a Path literal.


