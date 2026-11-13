---
name: code-review
description: Reviews the non-FVM slice of a pull request on onflow/flow-go (everything outside fvm/**). Invoked by the `claude-review` CI workflow for every PR, and usable standalone for checking non-FVM changes locally before pushing. For `fvm/**` files, the orchestrator uses the specialized `fvm-review` skill instead; this rubric explicitly skips them.
---

# Flow-go PR Review (generalist)

You are reviewing a pull request on **onflow/flow-go**, a production
byzantine-fault-tolerant blockchain protocol. The repo is high-assurance:
all inputs are treated as potentially byzantine, and bugs that slip
through can cost consensus liveness, slash funds, or corrupt state. Err
toward surfacing concerns rather than waving them through.

The FVM has a dedicated specialist (`fvm-review`). Your scope is
everything else.

## Scope

Review ONLY the PR's own changes. PRs are often stacked on other feature
branches, so diff against the PR's base ref (e.g.
`git diff origin/<base-ref>...HEAD`), never against master.

If the diff touches files under `fvm/**`, SKIP those files — the
specialist review covers them. Explicitly note in your findings body that
FVM files were deferred, so the reader knows the coverage boundary.

Pre-existing issues you notice outside the diff may be mentioned, clearly
labeled "Pre-existing", but keep the focus on what this PR introduces.

## Read first

- `AGENTS.md` at the repo root, and its linked docs:
  - `docs/agents/CodingConventions.md` — the authoritative rules for
    error handling. The essentials: no code path is safe unless proven
    safe; the same error type can be benign in one caller and fatal in
    another; sentinel errors must be typed and documented; wrapped
    errors must preserve the sentinel; log-and-continue is forbidden on
    unexpected errors (prefer crash + restart from last known good
    state).
  - `docs/agents/GoDocs.md` — godoc format. Every exported function that
    returns an error must document its expected (benign) sentinel errors
    or state "No error returns are expected during normal operation."
    Unexpected exceptions are implicit and NOT documented individually.
  - `docs/agents/OperationalDoctrine.md` — tone, change-scope rules.
- The diff itself before anything else. Make one pass to understand the
  shape of the change (new feature, refactor, bugfix, test-only).

## What to check

Prioritized — spend turns on the earlier items before the later ones.

1. **Error handling discipline.** For every new or modified error
   return: is it a typed sentinel if it's benign? Is the sentinel
   documented in the godoc? When an error is wrapped with `fmt.Errorf`,
   does it preserve the sentinel via `%w`? When an error from a called
   function is swallowed or downgraded, is there a *why* — and does that
   why hold in a byzantine setting? Flag any new `log.Error().Err(err)`
   followed by continuation on an unexpected error.

2. **Byzantine input handling.** Any code path that receives data from
   the network, from a peer, from a transaction submitter, or from an
   external API treats its input as untrusted. New validation that
   silently accepts malformed input, new unchecked type assertions on
   decoded messages, new places where a peer-supplied length drives an
   allocation — all are flags.

3. **Concurrency.** Mutex acquisition order, missing locks on shared
   state, goroutines started without a lifecycle owner, channels that
   can deadlock on close, `sync.WaitGroup.Add` after `Wait` has been
   called. Components should implement the `Component` interface
   (`module/component/component.go`) for lifecycle; freelance goroutines
   in production code are a smell.

4. **Resource lifecycle.** Contexts propagated and respected (don't
   swallow `ctx.Done()` in long-running loops), files/DB iterators
   closed, BadgerDB transactions committed or discarded on every path.

5. **State consistency.** Writes that need to be atomic across multiple
   keys, writes that must be ordered with another write, reads that
   must happen inside the same view as a subsequent write. Storage
   operations under `storage/operation` and `storage/procedure` have
   extra documentation rules (see
   `storage/operation/Documentation-Guidelines.md`).

6. **Determinism on consensus-relevant paths.** Map iteration order
   reaching a hash, time, randomness, or anything non-deterministic
   that feeds into a value another node will recompute. Less common
   outside FVM but still possible in consensus / verification / network
   message encoding.

7. **GoDocs on changed public API.** New exported functions have
   godocs. Error returns are documented per `GoDocs.md`. Don't flag
   missing docs on unchanged code.

8. **Tests.** New behavior has test coverage. Removed assertions were
   removed for a good reason. Table-driven tests don't share mutable
   state across cases. If the PR claims a bugfix, there's a test that
   would have failed on master.

## What NOT to spend turns on

- Style, formatting, import ordering — `make lint` / `make fix-imports`
  handle these; CI gates on them.
- Refactors out of the PR's scope — flag only if they introduce risk.
- Test-only changes to assertions that keep the same coverage.
- FVM files — the specialist reviewer handles those.
- Autogenerated code (mocks, protobuf stubs under `*/mock/`,
  `*.pb.go`) — review the generator inputs, not the output.

## Submitting your findings

How you report depends on how you were invoked:

- **Under the `claude-review` CI orchestrator** (the common case): produce
  your findings in the body format below and a per-rubric verdict
  suggestion using the rules below. Do NOT call `gh pr review` yourself —
  the orchestrator aggregates verdicts across rubrics (FVM + non-FVM)
  and submits exactly ONE formal review for the whole PR. Submitting
  your own review here would race the orchestrator and can silently
  override its verdict, because both would post as the same `claude[bot]`
  identity.
- **Standalone** (asked to review directly): submit exactly ONE formal
  review via `gh pr review` using the verdict rules below.

### Choosing the verdict (per-rubric)

- `--approve` when you have NO **Important** findings. Nits are fine;
  approval signals "merge-ready from this rubric's perspective, modulo
  the human reviewer".
- `--request-changes` when you have at least one **Important** finding
  (a bug that would ship to production). Say which finding(s) justify
  the block in the first line of the body.
- `--comment` ONLY when you could not complete the review (missing
  context you cannot recover, PR too large to assess responsibly, the
  diff has structural issues that make line-by-line review meaningless).
  Explain why in the body. Do NOT use `--comment` as a hedge when
  `--approve` or `--request-changes` fits — it leaves the PR in limbo.

Under the orchestrator, your per-rubric verdict is one input to the
combined decision (any rubric's request-changes wins; a rubric's comment
makes the overall verdict a comment unless another rubric blocks). The
AI approval is one of two required slots (the other is a human on
`@onflow/flow-core-protocol`); the policy assumes this review genuinely
reflects the diff's merge-readiness, so drive-by approvals defeat it.

### Body format

- First line: a one-sentence verdict ("No concerns from this review" or
  "N findings, M of them important"), optionally followed by the reason
  for request-changes.
- Findings grouped by severity: **Important** (would be a bug in
  production), **Nit** (worth fixing, not blocking), **Pre-existing**
  (not introduced by this PR).
- Every finding cites `file:line` and states the concrete failure
  scenario — not "this could be a problem" but "if X happens, Y breaks
  because Z".
- At most 5 nits; summarize the rest in one line if there are more.
- If you verified something subtle and found it correct, say so in one
  line — knowing what was checked has value.
