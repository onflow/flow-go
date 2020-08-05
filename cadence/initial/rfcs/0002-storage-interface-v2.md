<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Storage Interface v2](#storage-interface-v2)
  - [Description](#description)
  - [API](#api)
    - [Types](#types)
    - [Loading and Saving Objects](#loading-and-saving-objects)
    - [Creating and Getting Capabilities](#creating-and-getting-capabilities)
      - [Checking and Borrowing Capabilities](#checking-and-borrowing-capabilities)
  - [Syntax](#syntax)
    - [Paths](#paths)
    - [Capabilities](#capabilities)
    - [Borrowing](#borrowing)
  - [Examples](#examples)
  - [Future Work](#future-work)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->


# Storage Interface v2

## Description

The new storage interface replaces storable references with Capabilities.

A Capability consist of an Account and a Path.

An Account is either a Signing Account or a Public Account.

A Path consists of a Domain and an Identifier.
A Path is either a Storage Path, a Public Path, or a Private Path.

Paths and Capabilities are both value types, i.e. they are copied, and they can be stored.

References are created from capabilities.

There can be multiple capabilities, but only one unique reference to an object.

Capabilities are unforgeable, i.e. they can't be created arbitrarily,
only through the API described below (using a Signing Account).

Only an entity which has access to a Signing Account can access its storage.

Other entities never have direct access to storage.
Instead, an indirection scheme enables access, but also gives the possibility for revocation:

A value in storage (stored in an account's storage using a storage path) can be exposed to a limited audience by creating a Private Path link to a Storage Path.
The Private Path link can then changed, e.g. retargeted to another Storage Path, or removed (i.e. access is revoked).

A value in storage can be exposed to everyone by creating a Public Path link.

Aquiring a reference to a stored values basically consists of three steps:

- Account `getCapability`: Check if a capability exists at the given path.
- `Capability.check`: check if capability could be borrowed with type.
- `Capability.borrow`: Check if the value is stored under the path.

## API

### Types

Signing Accounts have the type `AuthAccount`.

Public Accounts have the type `PublicAccount`.

Capabilities have the type `Capability`.

Paths have the type `Path`.

### Loading and Saving Objects

Objects can be moved into/out of storage through functions of a Signing Account (`AuthAccount`) and a Storage Path:

- `fun save<T>(_ value: T, to: Path)`, where `T` is the type parameter for the object type:

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
  - Returns `nil`:
    - If the storage slot is empty.
    - If there is an active borrow on that slot.
  - The storage slot is always empty after succeeding.

   The path must be a Storage Path.

- `fun copy<T: AnyStruct>(from: Path): T?`, where `T` is the type parameter for the value type:

   Returns a copy of a value type in storage without removing it from storage.

  - Returns `nil`:
    - If the storage slot is empty.
    - if there is an active borrow on that slot.

   The type `T` must be a value/non-resource type.

   The path must be a Storage Path.

- `fun borrow<T: &Any>(from: Path): T?`, where `T` is the type parameter for the object type:

   Returns a unique reference to an object in storage without removing it from storage.
   The stored object must satisfy the required type.

  - Works for Resource or Value types.
  - Returns `nil`:
    - If the storage slot is empty.
    - If there is already an active borrow on that slot.

   The path must be a Storage Path.

### Creating and Getting Capabilities

Capabilities can be created through the `link` function of a Signing Account (`AuthAccount`):

- `fun link<T: &Any>(_ newCapabilityPath: Path, target: Path): Capability?`, where `T` is the type parameter for the object type:

  - `newCapabilityPath`: A Public Path or a Private Path where the new capability is created.

  - `target`: A Public Path, a Private Path, or a Storage Path that leads to the object that will provide the functionality defined by this capability.

     It is not necessary for the target to lead to a valid object; the target slot could be empty, or could lead to an object which does not provide the necessary type interface.

  - `T`: A type parameter that defines how the capability can be borrowed,
     i.e. what type the stored value can be accessed as.

     For example, if the stored value at the target path has type `@Kitty` (which conforms to interface `NFT`):

     If the type argument for type parameter `T` is `&{NFT}`,
     an unauthorized reference to any resource that allows access to the `NFT` functionality,
     then the borrowing may not downcast to `&Kitty`.

     However, if the type argument for type parameter `T` is `auth &{NFT}`,
     an authorized reference to any resource that allows access to the `NFT` functionality,
     then the borrowing may downcast to `&Kitty`.

  - Returns `nil` if a link for the given path already exists.

  - Does **not** check if the target path is valid/exists,
    and does **not** check if the target value conforms to the given type.
    The link is latent. The target value might be stored after the link is created,
    the target value might be moved out after the link has been created.

Capabilities can be removed through the `unlink` function of a Signing Account (`AuthAccount`):

- `fun unlink(_ path: Path)`:

  - `path`: A Public Path or a Private Path.

Capability targets can be queried using the `getLinkTarget` function:

- `fun getLinkTarget(_ path: Path): Path?`

  - `path`: A Public Path or a Private Path.

  - Returns the target path if a capability exists at the given path, or `nil` if it does not.

Existing capabilities can be accessed by path using `getCapability`,
defined on Signing Accounts (`AuthAccount`) and Public Accounts (`PublicAccount):

- `fun getCapability(at: Path): Capability?`

  - Returns a capability:
    - For Public Accounts, if passed a Public Path.
    - For Signing Accounts, if passed a Public Path or Private path.

  - Returns `nil`:
    - For Public Accounts, if passed a Private Path or Storage Path.
    - For Signing Accounts, if passed a Storage Path.

  - Does **not** check if the target exists. The link is latent.
    Use `Capability.check` to check if the target exists currently.

#### Checking and Borrowing Capabilities

Capabilities can be checked if they satisfy a type:

- `fun check<T: &Any>(): Bool`, where `T` is the type parameter for the object type:

   Returns true if the capability currently references an object satisfying the given type
   (without exceeding the type interfaces of all interim capabilities).

- `fun borrow<T: &Any>(): T?`, where `T` is the type parameter for the object type:

  Returns a unique reference to the object targeted by the capability,
  provided it meets the required type interface.

  - Returns `nil`:
    - If the targeted storage slot is empty.
    - If the targeted storage slot is borrowed.
    - If the requested type exceeds what is allowed by the capability (or any interim capabilities)

## Syntax

### Paths

The syntax of a Path is `/<domain>/<identifier>`.

The `<domain>` part of the Path syntax is `storage` for a Storage Path,
`public` for a Public Path, or `private` for a Private Path.

The `<identifier>` part of the Path syntax is an arbitrary identifier,
i.e. it does not have to be an identifier of an existing type or value.

Both `<domain>` and `<identifier>` parts of the Path syntax are static, i.e. they are not evaluated.

### Capabilities

The syntax of a Capability is `/<account>/<path>`.

The `<account>` part of the Capability syntax is an identifier and dynamic,
it is considered a variable identifier,
which should either have the type `AuthAccount` (authorized) or `PublicAccount` (unauthorized).

If the `<path>` part of the Capability syntax is just an identifier,
then the part is dynamic and is considered a variable which should have the type `Path`.
The `<path>` part of the Capability syntax may also be a Path literal.

### Borrowing

```cadence
let providerA: &{Provider} = borrow! capability as &{Provider}
let providerB: &{Provider}? = borrow? capability as &{Provider}?
```

## Examples

```cadence
// Setup
transaction {
    prepare(account: AuthAccount) {

        // Create a new Vault and store it
        account.save(
            <-create Vault(),
            to: /storage/ExampleVault)

        // Create a private withdrawal capability, to be used for default payments.
        account.link<&{Provider}>(
            /private/ExampleProvider,
            target: /storage/ExampleVault
        )

        // Create a public deposit capability, to be used when someone wants to send me money.
        account.link<&{Receiver}(
           /public/ExampleReceiver,
           target: /storage/ExampleVault
        )
    }
}
```

```cadence
// Transfer
transaction(amount: UFix64) {

    let tokensToSend: &{Provider}

    prepare(signer: AuthAccout) {
        self.tokensToSend <-
            signer.getCapability(/private/ExampleProvider)!
            .borrow<&{Provider}>()!
            .withdraw(amount)
    }

    execute {
        getAccount(0x02)
            .getCapability(/public/ExampleReceiver)!
            .borrow<&{Receiver}()!
            .deposit(<-tokensToSend)
    }
}
```

## Future Work

- Introspection for links and capabilities.
