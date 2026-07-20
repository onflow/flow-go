# Rulesets

Source of truth for branch protection policy on this repository. Each JSON
file here is the desired shape of a named GitHub ruleset. The
[`verify-rulesets`](../workflows/verify-rulesets.yml) workflow compares the
live rulesets against these specs on every push, on every PR touching this
directory, and daily on a schedule.

The workflow is **read-only** — it never modifies GitHub state. When a spec
and live differ, a repo admin must reconcile via
`Settings -> Rules -> Rulesets` in the UI. This gives us:

- A PR-reviewable audit trail for every policy change.
- Zero write tokens sitting in the repo.
- An automated alarm when someone hand-edits the UI without a matching PR.

## Applying changes

Ruleset changes are applied by a repo admin running [`apply.sh`](apply.sh)
from their own machine, using their own `gh` credentials — no service
account, no long-lived tokens, no CI job with write access.

```
.github/rulesets/apply.sh .github/rulesets/default-branch-protection.json
```

The script creates the ruleset if it doesn't exist yet, or updates it in
place if it does. It requires the caller to have Administration: write on
the repo. Nothing in this repo grants that permission — the caller uses
their own admin standing.

## Change flow

For any change to a ruleset:

1. Open a PR editing the relevant JSON file. Reviewers evaluate the
   *policy change* here — this is the substantive review.
2. Before merging, a repo admin runs `apply.sh` locally against the
   proposed spec (checkout the branch, then run the command above). This
   applies the change to the live ruleset.
3. The `verify-rulesets` check turns green (may need a re-run via
   `workflow_dispatch`), unblocking the merge.
4. Merge the PR. The next daily run confirms the state.

If the check fails on `master` or on a schedule run, the message names
which ruleset drifted. Reconcile by re-running `apply.sh` against the
spec on `master`, or by making the intentional change via a PR.

## Bootstrapping a new ruleset

For a spec that does not yet exist live, the same command creates it:

```
.github/rulesets/apply.sh .github/rulesets/<name>.json
```

After creation, run the `verify-rulesets` workflow once via
`workflow_dispatch` to confirm live matches spec.

## Policies

### `default-branch-protection.json`

Mirrors the legacy branch protection currently on `master`, and adds a
path-scoped required-reviewer entry for the FVM policy. Intended to
replace the legacy branch protection entirely once bootstrapped.

The ruleset carries four rules:

- **`deletion`** — the branch cannot be deleted.
- **`non_fast_forward`** — force pushes are blocked.
- **`pull_request`** — 2 approving reviews, code-owner review required,
  stale reviews are NOT dismissed on push (matches legacy). Adds the FVM
  path-scoped required reviewer (see below).
- **`required_status_checks`** — the 48 checks currently required by legacy
  branch protection, with strict mode (branches must be up to date).

`bypass_actors` grants `RepositoryRole` id `5` (Admin) always-bypass,
matching the legacy setting `enforce_admins.enabled: false` — any repo
admin can push past the ruleset. This is intentionally the least
restrictive option and mirrors current behavior 1:1.

If a stricter policy is desired later, options include: switching to
`actor_type: "OrganizationAdmin"` (org owners only, no repo admins),
adding `bypass_mode: "pull_request"` (bypass allowed only on PRs, not
direct pushes), or removing `bypass_actors` entirely (no bypass).

Existing tag ruleset on this repo uses a team-based bypass
(`actor_type: "Team"`, `flow-engineering`). We deliberately do not match
that here — the tag ruleset and this branch ruleset serve different
purposes and can carry different bypass policies.

#### FVM path-scoped review policy

The `pull_request` rule includes a `required_reviewers` entry:

```json
{
  "reviewer_id": 11293013,
  "file_patterns": ["*", "!fvm/**"],
  "approvals_needed": 2
}
```

`reviewer_id: 11293013` is `@onflow/flow-core-protocol`, the team CODEOWNERS
already designates as the owner of the whole repo. Combined with the global
`required_approving_review_count: 2`, the effect is:

| PR touches           | Approvals that satisfy the policy                                                                     |
| -------------------- | ----------------------------------------------------------------------------------------------------- |
| non-FVM files        | 2 humans in `@onflow/flow-core-protocol`. Bot approvals do not satisfy the team requirement.          |
| FVM files only       | Any 2 approvers with write access (satisfies the global count; no team requirement on FVM paths).     |
| Mixed FVM + non-FVM  | Must satisfy both: 2 humans in `@onflow/flow-core-protocol` for the non-FVM portion.                  |

The point is to let an approving AI reviewer count as one of the two on
strictly-FVM PRs, without allowing bot approvals to substitute for humans
anywhere else.

**Assumption:** exactly one AI reviewer account exists on the repo. If a
second is introduced, add a second required-reviewer entry with
`file_patterns: ["fvm/**"]` and `approvals_needed: 1` pointing at a
humans-only team, so a "2 bots, 0 humans" merge on FVM stays impossible.

If FVM eventually gets a dedicated reviewer team (e.g.
`@onflow/flow-cadence-execution`, which exists on the org and is described
as covering the Cadence execution stack), a second required-reviewer entry
can carry that too.

### Team IDs

`reviewer_id` in the JSON is a numeric GitHub team ID, not a slug. To
resolve or verify one:

```
gh api /orgs/onflow/teams/<team-slug> --jq .id
```

Currently used:

| team slug              | numeric id | used in                              |
| ---------------------- | ---------- | ------------------------------------ |
| `flow-core-protocol`   | 11293013   | `default-branch-protection.json` (required reviewer) |
