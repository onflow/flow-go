# Testnet EFM Recovery — Nov 2026

> **⚠️ DO NOT MERGE.** This document is the operational plan for a one-shot
> EFM recovery on testnet. It is pushed here for review visibility only
> and is not intended to land on `master`. Close the PR after the
> operation completes (or supersede with a trimmed, repo-worthy runbook).

**Author:** vishal (in collaboration with claude)
**Last regenerated:** 2026-11-25 ~20:03 UTC (= 12:03 PT)
**Status:** **PREP COMPLETE — awaiting go-ahead to submit `recover_epoch.cdc`.**
**Scope:** testnet (`devnet54`) only. Mainnet is explicitly out of scope.

This document adapts the [existing EFM Recovery runbook][runbook] and [Leo's TN53
recovery notes][tn53] to the current AWS-hosted testnet (post GCP→AWS fleet
migration, Nov 2026). Links to the Notion playbooks are in the References
section.

> **How this plan was actually executed (operator notes).**
>
> 1. `util` is built natively on the operator laptop (darwin/arm64) with
>    a plain `CGO_ENABLED=1 go build ./cmd/util` from the
>    `v0.51.0-rc.1` worktree — no Docker, no cross-compile.
> 2. Args generation runs locally on the laptop, not via SSM into
>    `devnet54-access-001`. The laptop already holds the
>    `service_account` signer key, so pulling the 14 CN tars onto the
>    same box does not widen the trust boundary.
> 3. The `util --config` flag wants a JSON array of
>    `[{Role, Address, Weight}]` filtered from `node-infos.pub.json` by
>    the set of node IDs present in `--internal-priv-dir` — **not** the
>    aggregated `node-internal-infos.pub.json` an earlier draft guessed.
>    See §4 Step 2.
> 4. `util` computes `targetEndTime` itself from the AN's reported
>    `currEpochTargetEndTime`, not from a flag. Scenario-B `refTimestamp`
>    (§4 Step 7) must use the util-sealed value from §2, not a value we
>    picked ahead of time.

[runbook]: # "Runbook: Epoch Fallback Mode (EFM) Recovery (Notion)"
[tn53]: # "TN53 EFM recovery, Jan 2026, Leo Zhang (Notion)"

---

## 1. Current state (observed 2026-11-25 ~20:00 UTC)

Pulled from Grafana Cloud Prometheus (`grafanacloud-flowfoundation-prom`,
`network="devnet54"`).

| Signal | Value |
|---|---|
| `consensus_compliance_epoch_fallback_triggered` | 1 |
| `consensus_compliance_current_epoch_counter` | 3277 |
| `consensus_compliance_current_epoch_phase` | 4 (`EpochPhaseFallback`) |
| `consensus_compliance_current_epoch_final_view` | **137,220,565** |
| `consensus_hotstuff_cur_view` | 137,142,984 |
| Views remaining in current extension | 77,581 |
| View rate | ~2.00 v/s |
| Estimated time to current extension end | **~10.8 h** (→ ~2026-11-26 06:47 UTC) |
| Views until the next extension gets added | ~76,581 (at `FinalizationSafetyThreshold = 1,000` before final view) |
| Est. time until next extension add | ~10.6 h (→ ~2026-11-26 06:38 UTC) |

Constants (independent of the current extension):

| Signal | Value |
|---|---|
| `consensus_compliance_current_dkg_phase1_final_view` | 133,414,165 |
| `consensus_compliance_current_dkg_phase2_final_view` | 133,416,165 |
| `consensus_compliance_current_dkg_phase3_final_view` | 133,418,165 |
| Inferred `DKGPhaseLength` | 2,000 views |
| EFM onset | between 2026-11-04 05:26 UTC and 09:21 UTC |
| Elapsed time in EFM | ~21.6 days (~43 normal 12 h epochs) |
| `FinalizationSafetyThreshold` (KV-store) | 1,000 views |
| `EpochExtensionViewCount` (KV-store) | 100,000 views |

### What this reading tells us

- **Latest extension in play.** `LastExtensionFinalView` is
  `137,220,565`; the recovery-epoch shape in §2 is derived against this
  value.
- **Submit window is ~10.6 h.** Next extension auto-adds at view
  `137,219,565` (= final view − `FinalizationSafetyThreshold`), i.e. at
  ~2026-11-26 06:38 UTC. We have comfortable headroom to submit.
- Pre-condition P2 (≥ `2 × FinalizationSafetyThreshold = 2,000` views
  remaining) holds trivially (77,581 views left).

### If a new extension lands before we submit

If execution slips past ~2026-11-26 06:38 UTC, `LastExtensionFinalView`
jumps to **137,320,565** and the extension end shifts to
~2026-11-26 20:27 UTC. The current noon-PT-Nov-26 target
(`2026-11-26 20:00 UTC`) becomes infeasible by ~27 min; we'd fall back
to **midnight PT Nov 26/27** (`2026-11-27 08:00 UTC` = Unix
`1,795,766,400`):

| New target end | Recovery duration (~) | `--epoch-length` (~) | `--epoch-staking-phase-length` (~) |
|---|---|---|---|
| **2026-11-27 08:00 UTC** (midnight PT Nov 26/27) | ~40,800 s (~11.3 h) | ~81,500 views | ~69,500 views (= 81,500 − 12,000) |

Procedure if this happens: re-run §4 Steps 2–3 with the fallback numbers;
`--recovery-epoch-counter` stays `3278`.

### Scenario classification — still Scenario B

The runbook splits recovery into two shapes:

- **Scenario A — EFM duration < one normal epoch.** We can configure a
  short recovery epoch that lands on the original N+1 boundary; no on-chain
  `FlowEpoch` timing-config update needed.
- **Scenario B — EFM duration > one normal epoch.** Extensions have pushed
  us past the scheduled end of Epoch N+1 (potentially many). We align the
  recovery epoch's end with some *later* epoch boundary (N+K) and update
  the `FlowEpoch` timing config so the on-chain epoch scheduler uses the
  recovery epoch as its new reference. Rewards for skipped epochs are not
  paid automatically.

**Where devnet54 sits.** EFM onset was ~2026-11-04 07:00 UTC, so we're
~21.6 days / ~43 normal 12 h epochs past the originally-scheduled end of
Epoch 3277 (~noon PT 2026-11-04). Our noon-PT-Nov-26 target is
~43 × 12 h after that boundary — i.e. the wall-clock point where
original Epoch 3277 + 43 ≈ **Epoch 3320** would have ended.

**→ Scenario B.** We need Step 7 (`update_epoch_timing_config.cdc`) in
addition to the base recovery.

**Manual reward payouts are skipped** on testnet (confirmed by vishal
2026-11-25) — testnet rewards have no economic value.

---

## 2. Target recovery parameters

Target: realign to **noon PT on Nov 26** (= `2026-11-26 20:00:00 UTC` =
Unix `1,795,723,200`). Gives a ~13.2 h recovery epoch starting from the
current extension's wall-clock end (~2026-11-26 06:47 UTC), landing
cleanly on the canonical noon-PT boundary.

Alternate targets considered (reject rationale in parens):

- Nov 26 08:00 UTC = midnight PT Nov 25/26 → infeasible after the latest
  extension add (would leave only ~1 h of recovery epoch).
- Nov 27 08:00 UTC = midnight PT Nov 26/27 → ~25 h recovery epoch (fine
  but longer than needed; prefer the shorter realignment).

### How each recovery-epoch flag is derived

The three "shape" flags for `util epochs efm-recover-tx-args` are
`--epoch-length` (views), `--epoch-staking-phase-length` (views), and
`--epoch-timing-recovery-duration` (seconds). The runbook's §1(i) gives the
derivation; this section walks it with devnet54's current-state numbers.

#### Common current-state inputs (from §1)

- `LastExtensionFinalView = 137,220,565`
- `ViewRate ≈ 2 v/s`
- Estimated wall-clock end of current extension ≈ `2026-11-26 06:47 UTC`
  (also matches the AN-reported `currEpochTargetEndTime = 1,795,675,640`).
- Target recovery-epoch end: **noon PT Nov 26 = 2026-11-26 20:00 UTC =
  Unix `1,795,723,200`**.
- `FinalizationSafetyThreshold = 1,000 views`.
- `DKGPhaseLength = 2,000 views`.

#### Step-by-step derivation

> **Note on `targetEndTime`.** The Cadence `recover_epoch.cdc` transaction
> argument `targetEndTime` is **not** a wall-clock quantity we pick; it's
> computed by the util as `epoch.TargetEndTime() + recoveryEpochTargetDuration`
> (`cmd/bootstrap/run/epochs.go:259-276`). `epoch.TargetEndTime()` is a
> Protocol-State value — it is the `EpochSetup` event's `TargetEndTime`
> plus `extensionViews × setupViewDuration`, maintained as extensions
> are added (`state/protocol/inmem/epoch.go:190-197`). It is not the
> on-chain observed extension-end timestamp. So the right way to hit a
> wall-clock target T is:
>
> 1. Read `TargetEndTime()` from the latest snapshot (we do this by
>    letting the util query the AN — same value appears in arg 5 of the
>    output JSON, minus our chosen duration).
> 2. Set `--epoch-timing-recovery-duration = T − TargetEndTime`.
> 3. After generation, assert `output[5].value == T` (used as Step 7
>    `refTimestamp`).
>
> For devnet54 we expected `TargetEndTime ≈ 1,795,675,640`; we set
> `--epoch-timing-recovery-duration = 47,550 s` so the sealed
> `targetEndTime` would land near our noon-PT target. The verification
> step is below in §4 Step 3.

**(a) `FinalView` of the recovery epoch.** Runbook §1(i) formula:

> The desired recovery epoch length is the difference between `FinalView`
> for the recovery epoch and the final view of the last extension.
> `EpochLength = FinalView − LastExtensionFinalView`.

We flip this: pick a wall-clock duration we want the recovery epoch to
span, multiply by the view rate, derive `EpochLength`. With view rate
2 v/s and target duration ~47,550 s (= target end − AN-reported
`TargetEndTime`):

```
EpochLength (views) ≈ 47,550 × 2 ≈ 95,100 views
```

**(b) `--epoch-length = 95,100`.** From (a).

**(c) `--epoch-timing-recovery-duration = 47,550` seconds.** The gap
between the AN-reported `TargetEndTime` (~`1,795,675,640` =
2026-11-26 06:47 UTC) and our target end (noon PT Nov 26 =
`1,795,723,200`). Paired with `--epoch-length = 95,100` views so
`length / duration = 2 v/s` exactly.

**(d) `--epoch-staking-phase-length = 83,100` views.** Runbook §1(i)
constraint:

> The staking phase length should provide enough time for the `EpochSetup`
> and `EpochCommitted` phases.
> `SetupAndCommittedPhasesLength > 3·DKGPhaseLength + 2·FinalizationSafetyThreshold`
> `StakingPhaseLength = EpochLength − SetupAndCommittedPhasesLength`.

Plug in testnet values:

```
lower bound on Setup+Commit  = 3 × 2,000 + 2 × 1,000 = 8,000 views
chosen Setup+Commit margin   = 12,000 views    (matches Leo's TN52 convention)
StakingPhaseLength           = 95,100 − 12,000 = 83,100 views
```

The 4,000-view headroom above the minimum (~33 min at 2 v/s) covers
cruise-control settling, finalization lag at phase boundaries, and a
small per-phase safety margin. TN52 used 12,000; TN53 trimmed to 8,400
on a short Scenario-A run. For our 13 h epoch there's no reason to go
tight.

**(e) `--recovery-epoch-counter = 3278`.** Runbook: "one above the current
epoch counter". Current = 3277 → recovery = 3278.

**(f) `--collection-clusters = 2`.** The tool re-builds the root cluster QCs
for the recovery epoch from scratch using the `--internal-priv-dir` staking
keys. The cluster count must match the current testnet topology, which has
been 2 since TN52. Confirmed in the AWS testnet Terraform module and in
the history of TN52/TN53 recoveries.

**(g) `--access-address` / `--access-network-key`.** The tool talks to an
AN via gRPC to pull the latest protocol state snapshot (to derive cluster
assignments etc.). Pick a healthy Foundation AN:

- `access-001.devnet54.nodes.onflow.org:9001`
- Network pubkey (shared, deterministic across testnet sporks):
  `ba69f7d2e82b9edf…2988d` *(published in the Notion runbook; same hex
  Leo used on TN52 and TN53)*

**(h) `--root-chain-id = flow-testnet`.** Chain ID for the Flow testnet,
unchanged.

### Recovery epoch params — final values

| Flag | Value | Role |
|---|---|---|
| `--root-chain-id` | `flow-testnet` | chain ID |
| `--access-address` | `access-001.devnet54.nodes.onflow.org:9001` | AN endpoint for snapshot RPC |
| `--access-network-key` | the testnet AN network pubkey `ba69f7d2…2988d` *(published in the Notion runbook)* | AN secure-RPC network pubkey |
| `--collection-clusters` | `2` | cluster count — must match current topology |
| `--recovery-epoch-counter` | `3278` | current counter + 1 |
| `--epoch-length` | `95100` views | recovery epoch view span, from (a)/(b) |
| `--epoch-staking-phase-length` | `83100` views | staking phase, from (d) |
| `--epoch-timing-recovery-duration` | `47550` seconds (~13.2 h) | wall-clock epoch duration, from (c) |
| `--config` | `./node-config.json` (built in §4 Step 2) | `[]{Role, Address, Weight}` for internal nodes only |
| `--internal-priv-dir` | `./private-root-information` (built in §4 Step 2) | aggregated per-node private tree |
| `--out` | `./devnet54_epoch_recovery.json` | tx-args output path |

### Comparison with prior EFM recoveries

Side-by-side with Leo's two prior runs, to make the shape of our choices
obvious:

| Flag | TN52 (Mar 2025) | TN53 (Jan 2026) | **This plan (TN54, Nov 2026)** |
|---|---|---|---|
| `--root-chain-id` | `flow-testnet` | `flow-testnet` | `flow-testnet` |
| `--access-address` | `access-001.devnet52.…:9001` | `access-001.devnet53.…:9001` | `access-001.devnet54.…:9001` |
| `--access-network-key` | `ba69f7d2…2988d` | `ba69f7d2…2988d` | `ba69f7d2…2988d` |
| `--collection-clusters` | 2 | 2 | 2 |
| `--recovery-epoch-counter` | 2066 | 2719 | **3278** |
| `--epoch-length` | 138,360 views | 86,400 views | **95,100 views** |
| `--epoch-staking-phase-length` | 126,360 views | 78,000 views | **83,100 views** |
| ↳ implied Setup+Commit | 12,000 views | 8,400 views | **12,000 views** |
| `--epoch-timing-recovery-duration` | 69,180 s (~19 h 13 m) | 43,200 s (12 h) | **47,550 s (~13.2 h)** |
| Implied view rate | 2.00 v/s | 2.00 v/s | 2.00 v/s |
| Scenario | B | A-ish (single-day EFM) | **B** |

**What's the same across all three:**
- Chain ID, AN network key, cluster count — network-wide testnet
  constants.
- Derivation formula — `length ÷ duration = 2 v/s` and
  `staking = length − (setup+commit margin)` — identical.
- Operator pattern: generate args with the aggregated bootstrap tree,
  then send the tx with the service-account key.

**What differs this time:**

- `--recovery-epoch-counter = 3278`. Current counter is 3277 vs.
  2065/2718 for the prior runs (testnet has been resporked between each).
- `--epoch-length = 95,100` and `--epoch-timing-recovery-duration = 47,550`
  pick a **~13.2 h** recovery epoch. TN52 chose 19 h to span overnight;
  TN53 chose a vanilla 12 h. Our length is set by what fits between the
  end of the current extension (~2026-11-26 06:47 UTC) and the next
  canonical testnet epoch boundary (noon PT Nov 26 = 20:00 UTC).
- `--epoch-staking-phase-length = 83,100` → Setup+Commit margin = 12,000.
  Same margin TN52 used; we deliberately did **not** copy TN53's tighter
  8,400 margin — our epoch is longer and there's no reason to be tight.

**Why the Scenario-B treatment on devnet54 looks similar to TN52 but not
TN53:**

- TN52 was triggered intentionally, left in EFM for ~29 h (~2.5× the
  normal epoch length) — Scenario B, resolved with a long recovery epoch.
- TN53 was a real incident lasting < 12 h — basically Scenario A, Leo
  chose a vanilla 12 h recovery epoch.
- TN54 (this run) has been in EFM for ~21.6 days (~43 normal epoch
  lengths). Textbook Scenario B — we need Step 7 (timing-config update)
  and we accept that the on-chain counter won't line up with the
  "original schedule" counter any more.

### Scenario-B timing config update (to pass to `update_epoch_timing_config.cdc`)

Use the util-sealed `targetEndTime` from the recovery args JSON
(**`1,795,723,190`** = 2026-11-26 19:59:50 UTC) as `refTimestamp`. The
10-second shortfall vs. exact noon PT (20:00 UTC) is a cosmetic drift
introduced by the view-rate rounding; it propagates forever through the
on-chain formula `T_n = refTimestamp + (n − refCounter) × duration`. If
cleaner alignment matters, use `refCounter = 3279, refTimestamp =
1,795,766,400` (midnight PT Nov 26/27) instead — the formula will then
place epoch 3280's end at noon PT Nov 27, 3281 at midnight, etc.

| Argument | Preferred value | Alternative (cleaner alignment) |
|---|---|---|
| `duration` | `43200` seconds (12 h) | `43200` |
| `refCounter` | `3278` | `3279` |
| `refTimestamp` | `1,795,723,190` (util-sealed) | `1,795,766,400` (midnight PT Nov 26/27) |

Current `FlowEpoch` timing config to be read via
[`get_epoch_timing_config.cdc`][get-timing] immediately before the update to
confirm `duration` has not changed.

[get-timing]: https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/scripts/get_epoch_timing_config.cdc

### Terraform follow-up (post-recovery, before next spork)

The AWS testnet Terraform module currently has no `epoch_timing_ref_counter`
/ `epoch_timing_ref_timestamp` surface (the convention from the legacy
Ansible stack has not been ported). For the next testnet spork to use the
realigned schedule, one of:

1. Add these fields to the Terraform module and wire into node `user_data`
   / bootstrap generation.
2. Pass the values explicitly to the next `util bootstrap finalize`
   invocation.

Not urgent for this operation; flagging for the next spork handoff.

---

## 3. Pre-conditions (re-verify immediately before Step 5)

| # | Check | Expected |
|---|---|---|
| P1 | No new extension since args were generated | `consensus_compliance_current_epoch_final_view{network="devnet54"}` still = `137,220,565` |
| P2 | ≥ `2 × FinalizationSafetyThreshold` = 2,000 views remaining | `final_view − cur_view ≥ 2000` |
| P3 | Generated `targetEndTime` matches intended Step 7 `refTimestamp` | `jq '.[5].value' devnet54_epoch_recovery.json` returns `"1795723190"` (or whatever value Step 7 plans to use) |
| P4 | Service-account signer available on the submitting machine | Operator's local `flow.json` with the testnet `service_account` alias loaded |

> Note on CN liveness: `util` signs cluster QCs locally from the private
> keys; it does **not** need the live CNs to be reachable at args-generation
> time. CN liveness *does* matter after recovery lands — the new clusters
> need a supermajority of each cluster's internal CNs online to finalize
> blocks in epoch 3278. If any CN is down post-recovery, surface it in
> Step 8 monitoring.

If P1 fails, regenerate args using the fallback target in §1 ("If a new
extension lands before we submit").

---

## 4. Execution plan

### Step 1 — build the `util` binary (ran on operator laptop)

Surveyed devnet54 flow-go versions via SSM `grep FLOW_GO_NODE_VERSION
/var/flow/runtime-conf.env` (2026-11-25):

| Role | Count | Running version |
|---|---|---|
| Consensus (SN) | 7 | **`v0.51.0-rc.1`** |
| Collection (LN) | 14 | `v0.51.0-rc.1` |
| Verification (VN) | 5 | `v0.51.0-rc.1` |
| Execution (EN) | 3 | `v0.51.1-payloadless-4` |
| Access (AN) | 2 | `v0.51.2` |

The `EpochRecover` service event is processed by **consensus nodes**, so
`util` must be built from the version that SNs run →
**`v0.51.0-rc.1`**. This tag lives on the internal `flow-go-internal`
mirror, not on public `flow-go`.

A plain CGO build on the operator laptop produces a working binary — no
Docker, no cross-compile, no SCP to an AN:

```bash
# On a trusted workstation. Run inside the flow-go-internal clone (the
# tag doesn't exist in public flow-go).
cd ~/go/src/github.com/onflow/flow-go-internal
git fetch origin tag v0.51.0-rc.1
git worktree add /tmp/flow-go-util v0.51.0-rc.1
cd /tmp/flow-go-util
CGO_ENABLED=1 go build -o /tmp/util ./cmd/util
/tmp/util epochs efm-recover-tx-args --help | head -5   # sanity
```

### Step 2 — assemble bootstrap tree on the operator laptop

The testnet bootstrap bucket is organized **per-node**:
`s3://<testnet-bootstrap-bucket>/<node>/bootstrap.tar.gz`. Each tar
contains `bootstrap/private-root-information/private-node-info_<id>/`
(with `node-info.priv.json`, `node-machine-account-info.priv.json`,
`random-beacon.priv.json`, `secretsdb-key`) and
`bootstrap/public-root-information/node-infos.pub.json` (the full testnet
identity list, which is the same copy in every node's tar).

`util epochs efm-recover-tx-args` expects two inputs:

- `--internal-priv-dir`: a directory walked recursively for files matching
  `node-info.priv.*`. The per-node tar's
  `private-node-info_<id>/node-info.priv.json` matches this.
- `--config`: a JSON array of `[{Role, Address, Weight}]` for the
  **internal** nodes only — those whose private keys live in
  `--internal-priv-dir`. This is **not** the aggregated
  `node-internal-infos.pub.json` the earlier draft of this plan guessed.
  It's filtered out of `node-infos.pub.json` by the set of node IDs
  present in `--internal-priv-dir`.

Prep layout on the laptop, with tight permissions on anything carrying
private material. **Only collection-node tars are pulled** — `util` uses
private keys exclusively to sign cluster QCs
(`cmd/bootstrap/run/epochs.go:130-175`), and the only internal keys it
reads are the CN staking keys. Keeping SN random-beacon, SN/AN/VN/EN
staking, machine-account, and secretsdb keys off the operator laptop
minimizes blast radius.

```bash
cd ~/testnet_efm_recovery_nov_2026
mkdir -p stage private-root-information
chmod 700 stage private-root-information
export BOOTSTRAP_BUCKET=flow-bootstrap-testnet

# Pull only the 14 internal CN bootstrap tars.
set -eo pipefail
for n in $(AWS_PROFILE=flow-testnet aws s3 ls s3://${BOOTSTRAP_BUCKET}/ \
           | awk '{print $2}' | grep '^devnet54-collection-' | tr -d '/'); do
  AWS_PROFILE=flow-testnet aws s3 cp \
    s3://${BOOTSTRAP_BUCKET}/${n}/bootstrap.tar.gz stage/${n}.tar.gz --quiet
done

# Extract the 14 CN tars into one aggregated tree; copy the
# private-node-info_<id>/ subdirs into the --internal-priv-dir target.
mkdir -p stage/_all
for t in stage/devnet54-collection-*.tar.gz; do tar xzf "$t" -C stage/_all/; done
cp -r stage/_all/bootstrap/private-root-information/* private-root-information/
chmod -R go-rwx private-root-information
ls private-root-information | wc -l    # expect 14 (all internal CNs)

# Build node-config.json by filtering node-infos.pub.json to the same 14
# CN node IDs and keeping only {Role, Address, Weight}.
ls private-root-information | sed 's/^private-node-info_//' \
  > internal-node-ids.txt
jq --slurpfile ids <(jq -R . internal-node-ids.txt | jq -s .) \
   '[ .[] | select(.NodeID as $x | $ids[0] | index($x)) | {Role, Address, Weight} ]' \
   stage/_all/bootstrap/public-root-information/node-infos.pub.json \
   > node-config.json
jq 'length' node-config.json      # expect 14
```

### Step 3 — generate recovery args JSON

```bash
cd ~/testnet_efm_recovery_nov_2026
# Testnet AN network pubkey (published in the Notion runbook):
ACCESS_NETWORK_KEY=ba69f7d2e82b9edf25b103c195cd371cf0cc047ef8884a9bbe331e62982d46daeebf836f7445a2ac16741013b192959d8ad26998aff12f2adc67a99e1eb2988d
/tmp/util epochs efm-recover-tx-args \
  --access-address=access-001.devnet54.nodes.onflow.org:9001 \
  --access-network-key=${ACCESS_NETWORK_KEY} \
  --root-chain-id=flow-testnet \
  --collection-clusters=2 \
  --recovery-epoch-counter=3278 \
  --epoch-length=95100 \
  --epoch-staking-phase-length=83100 \
  --epoch-timing-recovery-duration=47550 \
  --config=./node-config.json \
  --internal-priv-dir=./private-root-information \
  --out=./devnet54_epoch_recovery.json
```

Expected log markers:

```
read 14 internal private node-info files
read internal node configurations  (list of 14 {Role, Address, Weight})
read 14 weights for internal nodes
partitioning 0 partners + 14 internal nodes into 2 collector clusters
constructing root QCs for collection node clusters
producing QC for cluster (index: 0, size: 7) with 7 internal signers
producing QC for cluster (index: 1, size: 7) with 7 internal signers
wrote transaction args to output file ./devnet54_epoch_recovery.json
```

Sanity-check the JSON:

```bash
jq '[.[] | .type]' devnet54_epoch_recovery.json
# Expected 13 types matching recover_epoch.cdc, in order:
# UInt64, UInt64, UInt64, UInt64, UInt64, UInt64,
# Array, Array, Array, String, Dictionary, Array, Bool

# Decode the scalar args:
jq -c '{
  recoveryEpochCounter: .[0].value,
  startView:            .[1].value,
  stakingEndView:       .[2].value,
  endView:              .[3].value,
  targetDuration:       .[4].value,
  targetEndTime:        .[5].value
}' devnet54_epoch_recovery.json
```

Recorded values from the dry-run (2026-11-25 20:03 UTC):

```
recoveryEpochCounter = 3278
startView            = 137,220,566   (LastExtensionFinalView + 1)
stakingEndView       = 137,303,665   (LastExtensionFinalView + StakingPhaseLength)
endView              = 137,315,665   (LastExtensionFinalView + EpochLength)
targetDuration       = 47,550 s
targetEndTime        = 1,795,723,190 (2026-11-26 19:59:50 UTC)   ← use this for Scenario-B refTimestamp
```

Structural counts confirmed against expected topology:
`clusterAssignments = 2 × 7` (14 internal LNs → 2 clusters of 7);
`clusterQCVoteData = 2`; `dkgPubKeys = 7`; `dkgIdMapping = 7` (current
7-SN DKG committee carried forward); `nodeIDs = 120`;
`unsafeAllowOverwrite = false`.

**Step-3 → Step-7 consistency check.** The util computes
`targetEndTime = epoch.TargetEndTime() + recoveryEpochTargetDuration`
rather than taking a wall-clock flag. If Step 7 plans to use the
util-sealed value as `refTimestamp`, confirm equality now:

```bash
PLANNED_REFTIMESTAMP=1795723190     # from §2, Scenario-B timing config
SEALED_TARGETENDTIME=$(jq -r '.[5].value' devnet54_epoch_recovery.json)
[[ "$SEALED_TARGETENDTIME" == "$PLANNED_REFTIMESTAMP" ]] \
  && echo "OK: targetEndTime == refTimestamp" \
  || echo "DIVERGE: sealed=$SEALED_TARGETENDTIME planned=$PLANNED_REFTIMESTAMP"
```

If the Step 7 alternative is chosen (`refCounter=3279,
refTimestamp=1795766400`), skip this check — those variables are
independent of arg 5 by construction.

Wipe local bootstrap scratch once the JSON is captured:

```bash
cd ~/testnet_efm_recovery_nov_2026
rm -rf stage private-root-information internal-node-ids.txt node-config.json
```

Remaining files in the operator directory should be exactly three:

```
devnet54_epoch_recovery.json   (public: cluster QCs + DKG output)
flow.json                      (signer: service_account alias)
recover_epoch.cdc              (from flow-core-contracts master)
```

### Step 4 — re-verify pre-conditions (§3)

Repeat the four checks from §3. Abort if any fail.

### Step 5 — submit the recovery transaction

From the operator laptop, using a local `flow.json` holding the testnet
`service_account` alias and its signing key:

```bash
flow transactions send \
  -n testnet \
  -f <path-to-operator-flow.json> \
  --gas-limit 9999 \
  --signer service_account \
  --args-json "$(jq -c . devnet54_epoch_recovery.json)" \
  ./recover_epoch.cdc
```

`recover_epoch.cdc` is
[`transactions/epoch/admin/recover_epoch.cdc`][recover-cdc] from
`flow-core-contracts`; copy the version at the master commit of
`flow-core-contracts` deployed on testnet (same commit the on-chain
`FlowEpoch` contract was compiled from) into the working directory before
running.

[recover-cdc]: https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/admin/recover_epoch.cdc

Capture:

- Transaction ID
- Sealed block ID + height
- Event index for `A.<flow-epoch-address>.FlowEpoch.EpochRecover`

### Step 6 — observe recovery landing

- **On-chain:** run
  [`get_epoch_metadata.cdc`][get-md] with `epochCounter=3278` and confirm the
  recovery epoch fields match our params.
- **Protocol state:**
  - `consensus_compliance_epoch_fallback_triggered{network="devnet54"}` → 0
  - `consensus_compliance_current_epoch_phase{network="devnet54"}` → 3
    (`EpochPhaseCommitted`)
  - SN logs: `"successfully processed service event" type="recover"`

[get-md]: https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/scripts/get_epoch_metadata.cdc

### Step 7 — Scenario-B epoch-timing-config update

**Only** after Step 6 succeeds. Submit
[`update_epoch_timing_config.cdc`][upd-timing] with the Scenario-B values
from §2. See §2 for the choice between the util-sealed value (preserves
the cosmetic 10-second drift) and the cleaner `refCounter=3279,
refTimestamp=1,795,766,400` alternative that re-aligns all subsequent
epoch ends on exact noon/midnight PT.

[upd-timing]: https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/admin/update_epoch_timing_config.cdc

Validate with
[`get_target_end_time_for_epoch.cdc`][get-ete] for the chosen `refCounter`
→ should return the chosen `refTimestamp`.

[get-ete]: https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/scripts/get_target_end_time_for_epoch.cdc

### Step 8 — post-recovery monitoring

Cruise control is known to over-shoot block rate at the start of a recovery
epoch (TN53: 6.9 blocks/s seen within hours of recovery). Watch
`rate(consensus_compliance_finalized_blocks_total{network="devnet54"}[1m])` for
30 minutes post-recovery. If it climbs above ~3 blocks/s, pre-stage the SN
cruise-control nudge:

```bash
AWS_PROFILE=flow-testnet aws ssm send-command \
  --region us-east-2 \
  --targets "Key=tag:Name,Values=devnet54-consensus-*" \
  --document-name AWS-RunShellScript \
  --comment "EFM recovery: slow cruise ctl" \
  --parameters 'commands=["curl -s localhost:9002/admin/run_command -H '"'"'Content-Type: application/json'"'"' -d '"'"'{\"commandName\":\"set-config\",\"data\":{\"cruise-ctl-min-view-duration\":\"500ms\"}}'"'"'"]'
```

Back off to `400ms` / `300ms` as rate stabilizes, mirroring Leo's TN53 pattern.

### Step 9 — teardown

Scratch cleanup already happened at the end of Step 3 (`rm -rf stage
private-root-information internal-node-ids.txt node-config.json`). After
the recovery epoch completes (Step 8 monitoring window), the
`devnet54_epoch_recovery.json` and `hello.cdc` (if used) can also be
deleted; `flow.json` stays in the secure home for future ops.

### When does the recovery actually occur? (timing semantics)

Four distinct moments, often conflated:

1. **`FlowEpoch` contract transitions** — *immediately* on tx seal
   (~15 s after submit). Rewards for epoch 3277 are paid here.
   `FlowEpoch.currentEpochCounter()` then returns `3278`.
2. **Protocol state exits EFM** — ~1–2 min after tx seal (sealing lag for
   the `EpochRecover` service event). This is when the Grafana
   `..._epoch_fallback_triggered` metric drops 1 → 0 and the epoch phase
   flips 4 → 3.
3. **Recovery epoch (3278) starts running** — when the view counter
   crosses `startView = 137,220,566`. For a submit during the current
   extension, that's approximately when the current extension would have
   ended naturally (~2026-11-26 06:47 UTC).
4. **Network back on normal 12 h cadence** — after the recovery epoch
   ends at view `endView = 137,315,665` (~2026-11-26 19:59:50 UTC =
   noon PT Nov 26), epoch 3279 kicks off and automated transitions
   resume.

A submit inside the current extension clears EFM within minutes, runs
the ~13 h recovery epoch across the day, and returns the network to the
noon/midnight PT cadence by noon PT Nov 26.

---

## 5. Rollback / failure branches

| Failure | Action |
|---|---|
| New extension added before Step 5 (P1 fails) | Re-run §2 with the new `LastExtensionFinalView`; drop in updated params. |
| `util` panics on args generation | Fix the flag per the panic, re-run; do NOT submit a transaction. |
| `recover_epoch.cdc` transaction panics (`FlowEpoch` unchanged) | Correct per the panic, rerun Step 3 → Step 5. *(No rewards concern on testnet.)* |
| Transaction seals but `EpochRecover` service event is rejected | Per runbook Appendix: re-run Steps 1→5 with `--unsafe-overwrite-epoch-data`. *(No rewards concern on testnet; on mainnet, rewards paid in the first attempt are not re-paid.)* |
| View rate spikes post-recovery | Execute Step 8 cruise-control nudge. |
| Transaction never seals | Check `service_account` balance & sequence number; retry. |

---

## 6. Scope statement

- **Touches testnet only.** Mainnet is explicitly out of scope and no mainnet
  resources will be read or modified during this operation.
- **Reads** CN (and other internal node) private-node-info files from the
  testnet bootstrap bucket onto a 700-permed scratch directory on the
  operator laptop for the duration of args generation only, then
  `rm -rf`'d. (Earlier plan said we'd do this in tmpfs on
  `devnet54-access-001`; in practice the laptop already holds the
  `service_account` signer key so the trust boundary is unchanged.)
- **Writes** one testnet transaction (`recover_epoch.cdc`) signed by the
  testnet service account, and (Scenario B) one follow-up
  `update_epoch_timing_config.cdc`.

---

## 7. Open items to confirm before execution

1. ~~`util` release tag to pull~~ — **resolved**: build from internal
   mirror `v0.51.0-rc.1` to match devnet54 SNs. See §4 Step 1.
2. ~~Skip manual reward payouts on testnet?~~ — **resolved**: yes, skipped
   (vishal, 2026-11-25).
3. ~~service_account signer key location~~ — **resolved**: operator-local
   `flow.json`, `service_account` alias.
4. ~~Whether `jq`-concatenated `node-internal-infos.pub.json` is accepted
   by current `util epochs efm-recover-tx-args`~~ — **resolved differently
   than planned**: `--config` doesn't want the aggregated public-info
   file at all. It wants a trimmed `[]{Role, Address, Weight}` filtered
   from `node-infos.pub.json` by the set of node IDs present in
   `--internal-priv-dir`. See §4 Step 2 for the actual recipe; dry-run
   generated a valid 13-arg JSON on 2026-11-25 14:54 UTC.
5. (Nice-to-have, not blocking) decision on whether to also open a
   Terraform PR to add `epoch_timing_ref_*` to the AWS testnet module for
   next-spork parity. Can be a follow-up.

---

## 8. References

- Runbook: *Epoch Fallback Mode (EFM) Recovery* (Notion) —
  mirrored locally at `~/testnet_efm_recovery_nov_2026/Runbook_for_hcu/`.
- Prior execution: *11. Test EFM Recovery on Testnet* (devnet52, 2025-03-07,
  Leo Zhang) — mirrored at
  `~/testnet_efm_recovery_nov_2026/EFM_Recovery_Testnet/`.
- Prior execution: *TN53 EFM recovery* (devnet53, 2026-01-28, Leo Zhang) —
  mirrored at `~/testnet_efm_recovery_nov_2026/Recent_efm_recovery/`.
- flow-go EFM utility:
  https://github.com/onflow/flow-go/blob/master/cmd/util/cmd/epochs/cmd/recover.go
- flow-core-contracts recovery tx:
  https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/admin/recover_epoch.cdc
