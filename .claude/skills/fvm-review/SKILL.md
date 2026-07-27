---
name: fvm-review
description: Reviews a pull request that modifies the Flow Virtual Machine (fvm/**) on onflow/flow-go. Use when asked to review an FVM PR, when running `claude-review-fvm` CI, or when checking FVM changes locally before pushing. Specialized for consensus-critical execution-layer code; distinct from generic code review.
---

# FVM PR Review

You are reviewing a pull request on **onflow/flow-go** that modifies the Flow
Virtual Machine (`fvm/**`), the execution layer of a byzantine-fault-tolerant
blockchain. Correctness here is consensus-critical: execution nodes must
produce byte-identical results for the same block, and bugs can corrupt
on-chain state or fork the network.

## Scope

Review ONLY the PR's own changes. The base branch is the PR's base ref — FVM
PRs are often stacked on other feature branches, so diff against that base
(e.g. `git diff origin/<base-ref>...HEAD`), never against master. Pre-existing
issues you notice may be mentioned, clearly labeled "Pre-existing", but keep
the focus on the diff.

## Read first

- `AGENTS.md` at the repo root — high-assurance conventions. The essentials:
  all inputs are potentially byzantine; error classification is
  context-dependent (the same error can be benign in one caller and fatal in
  another); undocumented errors are treated as fatal and must propagate —
  log-and-continue is forbidden.
- `fvm/README.md` — architecture overview (Context -> HostEnvironment ->
  Procedure lifecycle).
- `fvm/errors/codes.go` and `fvm/errors/errors.go` — FVM's own error taxonomy.
  NOTE: `fvm/` does NOT use the sentinel-error model from the rest of the
  repo. It separates non-fatal `CodedError` (ErrorCode, user-visible,
  recoverable) from fatal `CodedFailure` (FailureCode), split at the boundary
  via `errors.SplitErrorTypes`.

## FVM-specific bug classes to check

These are the failure modes that have actually bitten this codebase. Check
the diff against each:

1. **Execution-result determinism / HCU discipline.** Anything that changes
   observable execution behavior — error messages included in transaction
   results, metering amounts, emitted events, enforced limits — changes
   execution results and requires a coordinated height upgrade (HCU) to
   deploy. If the diff changes such behavior, the PR description must
   acknowledge it. Flag silent behavior changes loudly.

2. **Cache-state independence.** Program loads and other derived-data
   computations (`fvm/storage/derived`) must charge identical computation
   warm or cold. A transaction must never be charged differently because
   another transaction warmed the cache first. Look for meter state leaking
   into or out of cached snapshots.

3. **Metering scope semantics** (`fvm/meter`, `fvm/storage/state`).
   `RunWithMeteringDisabled` suppresses accumulation AND limit enforcement.
   Watch the boundaries: the interaction meter counts only the first read of
   a register (later reads are free cache hits), so reads/writes inside a
   disabled scope change what later metered reads cost. Nested transaction
   merges (`ExecutionState.Merge`) decide whose meter absorbs the charges —
   check every Begin/Commit/Restart pairing, especially error paths where
   nested transactions unwind. `RunWithMeteringDisabled` takes a closure, so
   errors escape via captured variables
   (`RunWithMeteringDisabled(func() { err = foo() })`) — check every such site
   for shadowed or stale `err` and for a missing error check immediately after
   the closure returns.

4. **Error taxonomy discipline** (`fvm/errors`). A `CodedFailure` must always
   propagate as a failure — flag any callsite that downgrades one to a benign
   error, and any re-wrap of a `CodedError` into a `CodedFailure` (or vice
   versa) without justification. Error CODES are an external contract —
   downstream systems index on them; never renumber or reuse. Error MESSAGES
   are part of the execution result — changing one is a behavior change (see
   #1). In `fvm/evm`, the EVM error path has its own semantics:
   `errors.NewEVMError` wraps non-fatal EVM errors as user errors, and
   `IsEVMError` / `IsFailure` classification at that boundary must be
   preserved.

5. **Transaction pipeline invariants** (`transactionInvoker.go` and friends).
   Order matters: signature verification → sequence number → payer balance
   check (unmetered) → body (metered, inside a nested transaction) → storage
   limit checks → fee deduction (unmetered). Fee deduction and other
   system-critical unmetered scopes must never be able to fail on a metering
   limit. The error-execution path must leave the nested transaction stack
   consistent.

6. **Service-account special cases.** Several limits are not enforced when
   the payer is the service account. Changes to limit enforcement must
   preserve these exemptions — and must not accidentally widen them.

7. **SPoCK sensitivity** (`fvm/storage/state` `spockState`). Execution
   snapshots feed SPoCK proofs. Any nondeterminism in the read/write set —
   map iteration order reaching the ledger, time, randomness — breaks
   verification.

## What NOT to spend turns on

- Style, formatting, import ordering — CI and CodeRabbit already cover these.
- Refactors out of the PR's scope.
- Test-only changes to assertions that keep the same coverage.

## Submitting your findings

Post exactly ONE comment on the PR via `gh pr comment`, with:

- First line: a one-sentence verdict ("No concerns from this review" or
  "N findings, M of them important").
- Findings grouped by severity: **Important** (would be a bug in production),
  **Nit** (worth fixing, not blocking), **Pre-existing** (not introduced by
  this PR).
- Every finding cites `file:line` and states the concrete failure scenario —
  not "this could be a problem" but "if X happens, Y breaks because Z".
- At most 5 nits; summarize the rest in one line if there are more.
- If you verified something subtle and found it correct, say so in one line —
  knowing what was checked has value.

When running in CI trial mode, do not approve or request changes. Findings
only. (The workflow's tool allowlist omits `gh pr review`, so approval is
impossible at the tool level regardless of this instruction.)
