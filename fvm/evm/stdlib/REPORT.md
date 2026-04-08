# EVM Contract Security Audit Report

**Contract:** `fvm/evm/stdlib/contract.cdc`
**Branch:** `josh/evm-contract-security-audit`
**Date:** 2026-04-07
**Auditor:** Josh Hannan (with AI assistance)
**Reference Docs:** https://developers.flow.com/build/evm/how-it-works

---

## Summary

The EVM contract is well-structured and demonstrates good use of Cadence patterns including entitlements, resource ownership, and pause-guard logic. No critical security vulnerabilities were identified. The findings below are organized by severity and cover bugs, access control concerns, documentation inconsistencies, and optimization opportunities.

---

## Open Findings

### [LOW] Event Emitted Inside `pre` Block — CEI Violation

**Location:** Lines 1157–1169 (`BridgeRouter.setBridgeAccessor` default implementation)

**Description:**
The `BridgeAccessorUpdated` event is emitted inside the `pre` condition block:

```cadence
access(Bridge) fun setBridgeAccessor(_ accessor: Capability<auth(Bridge) &{BridgeAccessor}>) {
    pre {
        accessor.check():
            "EVM.setBridgeAccessor(): Invalid BridgeAccessor Capability provided"
        emit BridgeAccessorUpdated(
            ...
        )
    }
}
```

This violates the Checks-Effects-Interactions pattern. Events represent state changes and should be emitted in the function body *after* the state has been updated, not as part of a precondition. While Cadence transactions are atomic (a panic reverts all events), placing `emit` in `pre` blocks is:

1. Semantically incorrect — a precondition should be a boolean assertion, not a side effect.
2. Fragile — if a future implementation of `setBridgeAccessor` adds logic that can fail in the function body, the event fires before those effects are applied, creating a misleading audit trail.
3. An anti-pattern per Cadence design guidelines.

Note: the `pre` block placement may be intentional to guarantee the event fires even when concrete implementations override the function body, since interface `pre` blocks are always executed. This should be confirmed before applying the fix.

**Potential fix** (if emission is not required to be guaranteed across overrides):
```cadence
access(Bridge) fun setBridgeAccessor(_ accessor: Capability<auth(Bridge) &{BridgeAccessor}>) {
    pre {
        accessor.check(): "EVM.setBridgeAccessor(): Invalid BridgeAccessor Capability provided"
    }
    emit BridgeAccessorUpdated(
        routerType: self.getType(),
        routerUUID: self.uuid,
        routerAddress: self.owner?.address ?? panic("Router must be stored in an account's storage"),
        accessorType: accessor.borrow()!.getType(),
        accessorUUID: accessor.borrow()!.uuid,
        accessorAddress: accessor.address
    )
}
```

---

### [LOW] `decodeABIWithSignature` Uses O(n) `removeFirst()` in a Loop

**Location:** Lines 969–973

**Description:**
```cadence
for byte in methodID {
    if byte != data.removeFirst() {
        panic("...")
    }
}
```

`removeFirst()` on a Cadence array is O(n) as it shifts all remaining elements. This is called 4 times (once per method ID byte), making the total prefix-strip O(4n). The array could instead be sliced:

```cadence
let methodID = HashAlgorithm.KECCAK_256.hash(signature.utf8).slice(from: 0, upTo: 4)
for i in [0, 1, 2, 3] {
    if data[i] != methodID[i] {
        panic("...")
    }
}
return InternalEVM.decodeABI(types: types, data: data.slice(from: 4, upTo: data.length))
```

---

## Summary Table

| ID | Severity | Title | Status |
|---|---|---|---|
| 3 | LOW | Event emitted in `pre` block (CEI violation) | Open |
| 4 | MEDIUM | Signed data not bound to EVM address in ownership proof | Fixed |
| 9 | LOW | O(n) `removeFirst()` in loop in `decodeABIWithSignature` | Open |
