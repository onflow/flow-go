# Testnet EFM Recovery — Nov 2026

> **⚠️ DO NOT MERGE.** This document is the operational plan for a one-shot
> EFM recovery on testnet. It is pushed here for review visibility only
> and is not intended to land on `master`. Close the PR after the
> operation completes (or supersede with a trimmed, repo-worthy runbook).

**Author:** vishal (in collaboration with claude)
**Date drafted:** 2026-11-25
**Status:** PLAN — not yet executed
**Scope:** testnet (`devnet54`) only. Mainnet is explicitly out of scope.

This document adapts the [existing EFM Recovery runbook][runbook] and [Leo's TN53
recovery notes][tn53] to the current AWS-hosted testnet (post GCP→AWS fleet
migration, Nov 2026). Links to the Notion playbooks are in the References
section.

[runbook]: # "Runbook: Epoch Fallback Mode (EFM) Recovery (Notion)"
[tn53]: # "TN53 EFM recovery, Jan 2026, Leo Zhang (Notion)"

---

## 1. Current state (re-pulled 2026-11-25 ~13:50 UTC)

Pulled from Grafana Cloud Prometheus (`grafanacloud-flowfoundation-prom`,
`network="devnet54"`). Two readings so far — unchanged values collapsed to
one column, values that moved shown side-by-side.

| Signal | At 05:22 UTC (draft) | **At 13:50 UTC (now)** |
|---|---|---|
| `consensus_compliance_epoch_fallback_triggered` | 1 | **1** (unchanged) |
| `consensus_compliance_current_epoch_counter` | 3277 | **3277** (unchanged) |
| `consensus_compliance_current_epoch_phase` | 4 (`EpochPhaseFallback`) | **4** (unchanged) |
| `consensus_compliance_current_epoch_final_view` | 137,120,565 | **137,120,565** (unchanged — same extension) |
| `consensus_hotstuff_cur_view` | 137,037,426 | **137,098,556** |
| Views remaining in current extension | 83,139 | **22,009** |
| View rate | ~2.00 v/s | **~2.00 v/s** |
| Estimated time to current extension end | ~11.5 h (16:55 UTC) | **~3.06 h** (→ 2026-11-25 16:54 UTC) |
| Views until the next extension gets added | ~82,139 | **~21,009** (at `FinalizationSafetyThreshold = 1,000` before final view) |
| Est. time until next extension add | ~11.4 h | **~2.92 h** (→ ~2026-11-25 16:45 UTC) |

Constants (unchanged — re-stated for completeness):

| Signal | Value |
|---|---|
| `consensus_compliance_current_dkg_phase1_final_view` | 133,414,165 |
| `consensus_compliance_current_dkg_phase2_final_view` | 133,416,165 |
| `consensus_compliance_current_dkg_phase3_final_view` | 133,418,165 |
| Inferred `DKGPhaseLength` | 2,000 views |
| EFM onset | between 2026-11-04 05:26 UTC and 09:21 UTC |
| Elapsed time in EFM (at re-pull) | ~21.3 days |
| Extensions added so far | ~36 (each +100,000 views) |
| `FinalizationSafetyThreshold` (KV-store) | 1,000 views |
| `EpochExtensionViewCount` (KV-store) | 100,000 views |

### What this refresh tells us

- **Recovery parameters are view-based and have not changed.** The last
  extension's final view (137,120,565) is unchanged, and the recovery-epoch
  shape in §2 was computed from that value + the wall-clock target end
  time. Both inputs are still identical to the draft, so the three shape
  flags stay at `--epoch-length=108,000`, `--epoch-staking-phase-length=96,000`,
  `--epoch-timing-recovery-duration=54,000`.
- **The window to submit inside the current extension is now ~3 h.** The
  next extension gets added at `FinalizationSafetyThreshold = 1,000` views
  before the current extension's final view, i.e. at view 137,119,565,
  which is ~2.9 h away at the current rate. See the "if we miss the
  window" section below.
- Pre-condition P2 (≥ `2 × FinalizationSafetyThreshold = 2,000` views
  remaining) still holds comfortably (22,009 views left), but the margin
  is shrinking.

### If a new extension is added before we submit

If execution slips past ~2026-11-25 16:45 UTC, the current extension will
auto-renew: `LastExtensionFinalView` jumps from 137,120,565 → **137,220,565**
and the new wall-clock extension end shifts to ~2026-11-26 06:47 UTC. The
target end time (midnight PT Nov 25/26 = 2026-11-26 08:00 UTC) becomes
infeasible — only ~70 min of recovery-epoch runway left, which does not
fit a safe Setup+Commit phase.

Fallback targets if the window is missed:

| New target end | Recovery duration (~) | `--epoch-length` (~) | `--epoch-staking-phase-length` (~) |
|---|---|---|---|
| **2026-11-26 20:00 UTC** (noon PT Nov 26) | 47,500 s (~13.2 h) | 95,000 views | 83,000 views (= 95,000 − 12,000) |
| **2026-11-27 08:00 UTC** (midnight PT Nov 26/27) | 90,700 s (~25.2 h) | 181,000 views | 169,000 views |

If this happens:
1. Pull fresh metrics, confirm new `LastExtensionFinalView = 137,220,565`.
2. Pick one of the two fallback targets above (default: noon PT Nov 26 for
   the shorter realignment).
3. Re-derive `--epoch-timing-recovery-duration`, `--epoch-length`,
   `--epoch-staking-phase-length` with the formulas in §2.
4. Regenerate the args JSON. The Scenario-B timing-config `refTimestamp`
   in §2 also changes to the new target end (`refCounter` stays 3278).

### Scenario classification

#### What the runbook calls Scenario A vs Scenario B

Every testnet epoch has a scheduled wall-clock end-time. When EFM kicks in,
the protocol does **not** cut the current epoch short — it leaves the epoch
counter as is and extends the current epoch's final view by successive
`EpochExtensionViewCount` increments until a recovery transaction lands.
The result: by the time EFM is resolved, the wall-clock "end" of the
extended epoch is far past where the original schedule put it.

The runbook splits the recovery into two shapes based on how far past the
original schedule we are:

- **Scenario A — EFM duration < one normal epoch.** Extensions have pushed
  us past the scheduled end of Epoch N, but not past the scheduled end of
  Epoch N+1. We can simply configure a short recovery epoch that lands on
  the original N+1 boundary and we're back on schedule. No change to the
  on-chain `FlowEpoch` timing config is needed.
- **Scenario B — EFM duration > one normal epoch.** Extensions have pushed
  us past the scheduled end of Epoch N+1 (potentially many epochs). We
  cannot fit a single recovery epoch that returns us to Epoch N+1's original
  boundary. Instead we align the recovery epoch's end with some *later*
  epoch boundary (N+K) and update the `FlowEpoch` timing config so the
  on-chain epoch scheduler uses the recovery epoch as its new reference.
  Rewards for the skipped epochs (N+1 … N+K-1) are **not** paid
  automatically — the runbook asks for a separate manual payout before
  recovery.

#### Where devnet54 sits

- EFM onset: ~2026-11-04 07:00 UTC (step in
  `consensus_compliance_epoch_fallback_triggered`).
- Current time of plan: 2026-11-25 05:22 UTC → **~21 days in EFM**.
- Testnet epoch duration before EFM: **12 h** (43,200 s). So EFM has lasted
  **~42 normal epochs**.
- Original Epoch 3277 was scheduled to end at approximately
  **noon PT 2026-11-04** (= 2026-11-04 20:00 UTC) — derived from: counter
  bumped 3276 → 3277 at ~2026-11-03 19:21 UTC + nominal 12 h epoch.
- Target recovery epoch end: **2026-11-26 08:00 UTC** (= midnight PT
  Nov 25/26). That's ~44 × 12 h after the scheduled end of Epoch 3277 —
  i.e. where original Epoch 3277 + 44 ≈ **Epoch 3321** would have ended.

**→ Scenario B.** We need Step 7 (`update_epoch_timing_config.cdc`) in
addition to the base recovery.

**Manual reward payouts are skipped** on testnet (confirmed by vishal
2026-11-25) — testnet rewards have no economic value.

---

## 2. Target recovery parameters

Target: realign to **midnight PT on Nov 25/26** (= `2026-11-26 08:00:00 UTC` =
Unix `1795680000`). Gives a ~15 h recovery epoch — long enough to observe,
intervene on cruise control if needed, and still realign on the canonical
noon/midnight PT boundary.

Alternate targets considered (reject rationale in parens):

- Nov 25 20:00 UTC = noon PT Nov 25 → only 3 h recovery epoch (too short; too
  much risk of racing a new extension add).
- Nov 26 20:00 UTC = noon PT Nov 26 → 27 h recovery epoch (fine but longer than
  needed; prefer the shorter realignment).

### How each recovery-epoch flag is derived

The three "shape" flags for `util epochs efm-recover-tx-args` are
`--epoch-length` (views), `--epoch-staking-phase-length` (views), and
`--epoch-timing-recovery-duration` (seconds). The runbook's §1(i) gives the
derivation; this section walks it with devnet54's current-state numbers.

#### Common current-state inputs (from §1)

- `LastExtensionFinalView = 137,120,565`
- `CurrentView` at re-pull = 137,098,556 → views remaining in extension =
  22,009. (Important: the derivation of the three shape flags below does
  not depend on `CurrentView`. It only depends on `LastExtensionFinalView`
  — which has not changed — and the wall-clock extension end time, which is
  likewise view-based and so unchanged.)
- `ViewRate ≈ 2 v/s` → extension end time =
  `LastExtensionFinalView_wall_clock ≈ 2026-11-25 16:54 UTC` (stable across
  both measurements, as long as view rate stays ≈ 2 v/s).
- `FinalizationSafetyThreshold = 1,000 views` (testnet KV-store default).
- `DKGPhaseLength = 2,000 views` (inferred from phase-final-view deltas).
- `EpochExtensionViewCount = 100,000 views` (confirmed from the step pattern
  in `consensus_compliance_current_epoch_final_view`).

#### Step-by-step derivation

**(a) `FinalView` of the recovery epoch.** Runbook §1(i) formula:

> The desired recovery epoch length is the difference between `FinalView`
> for the recovery epoch and the final view of the last extension.
> `EpochLength = FinalView − LastExtensionFinalView`.

We flip this: we pick a target wall-clock end time, convert to views using
the view rate, and derive `FinalView`. With view rate 2 v/s:

```
Δt(seconds between extension end and target end) ≈ 1,795,680,000 − 1,795,625,724 ≈ 54,276 s
EpochLength (views)                              ≈ 54,276 × 2         ≈ 108,552 views
```

We round the length **down** to **108,000 views** to match the
`--epoch-timing-recovery-duration = 54,000 s` we pick next. (Rounding
down avoids a last-view overshoot past the target wall-clock end.)

**(b) `--epoch-length = 108,000`.** Exactly the rounded value from (a).

**(c) `--epoch-timing-recovery-duration = 54,000` seconds.** Runbook §1(i):

> The duration should be the wall-clock time between when the current epoch
> extension is scheduled to end and the desired end time of the recovery
> epoch.

`54,000 s = 108,000 views / 2 v/s` — i.e. we want the recovery epoch to be
exactly 15 wall-clock hours, which lands at Nov 26 08:00 UTC assuming the
view rate stays near 2 v/s. Cruise control will tighten up the view rate
over the course of the epoch to hit this.

**(d) `--epoch-staking-phase-length = 96,000` views.** Runbook §1(i)
constraint:

> The staking phase length should provide enough time for the `EpochSetup`
> and `EpochCommitted` phases.
> `SetupAndCommittedPhasesLength > 3·DKGPhaseLength + 2·FinalizationSafetyThreshold`
> `StakingPhaseLength = EpochLength − SetupAndCommittedPhasesLength`.

Plug in testnet values:

```
lower bound on Setup+Commit  = 3 × 2,000 + 2 × 1,000 = 8,000 views
chosen Setup+Commit margin   = 12,000 views    (matches Leo's TN52 convention)
StakingPhaseLength           = 108,000 − 12,000 = 96,000 views
```

Why 12,000 and not the bare minimum 8,000? The 4,000-view headroom (~33 min)
gives slack for a) cruise control settling, b) finalization lag at the
phase boundary, and c) a small per-phase safety margin. Leo used 12,000 on
TN52 (also Scenario B); TN53 used only 8,400 because that was a short
Scenario-A-style recovery where he was confident in the DKG timing budget.
For our 15 h epoch there is plenty of room to be generous.

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
| `--access-network-key` | *(see §4 Step 3 — the testnet AN network pubkey; `ba69f7d2…2988d`)* | AN secure-RPC network pubkey |
| `--collection-clusters` | `2` | cluster count — must match current topology |
| `--recovery-epoch-counter` | `3278` | current counter + 1 |
| `--epoch-length` | `108000` views | recovery epoch view span, from (a)/(b) |
| `--epoch-staking-phase-length` | `96000` views | staking phase, from (d) |
| `--epoch-timing-recovery-duration` | `54000` seconds (15 h) | wall-clock epoch duration, from (c) |
| `--config` | `<operator-host>:/mnt/efm-recovery/public-root-information/node-internal-infos.pub.json` | assembled on operator host (see §4) |
| `--internal-priv-dir` | `<operator-host>:/mnt/efm-recovery/private-root-information/` | assembled on operator host |
| `--out` | `<operator-host>:/mnt/efm-recovery/devnet54_epoch_recovery.json` | tx-args output path |

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
| `--epoch-length` | 138,360 views | 86,400 views | **108,000 views** |
| `--epoch-staking-phase-length` | 126,360 views | 78,000 views | **96,000 views** |
| ↳ implied Setup+Commit | 12,000 views | 8,400 views | **12,000 views** |
| `--epoch-timing-recovery-duration` | 69,180 s (~19 h 13 m) | 43,200 s (12 h) | **54,000 s (15 h)** |
| Implied view rate | 2.00 v/s | 2.00 v/s | 2.00 v/s |
| Scenario | B | A-ish (single-day EFM) | **B** |

**What's the same across all three:**
- Chain ID, AN network key, cluster count — these are network-wide
  constants on testnet.
- The derivation formula — `length ÷ duration = 2 v/s` and
  `staking = length − (setup+commit margin)` — is identical.
- Operator pattern: generate args on a trusted operator host with the
  aggregated bootstrap tree, scp the JSON to the laptop, send the tx.

**What differs this time:**

- `--recovery-epoch-counter = 3278`. Trivial bump: current counter is 3277
  vs. 2065/2718 for the prior runs. (Testnet has been resporked between
  each, hence the jumps.)
- `--epoch-length = 108,000` and `--epoch-timing-recovery-duration = 54,000`
  pick a **15 h** recovery epoch. TN52 picked 19 h to span overnight,
  TN53 picked 12 h to match a normal testnet epoch. We picked 15 h as a
  compromise: long enough to absorb the known post-recovery cruise-control
  spike (TN53's 6.9 v/s incident) while still landing cleanly on the
  midnight PT boundary ~15 h after the current extension ends.
- `--epoch-staking-phase-length = 96,000` → Setup+Commit margin = 12,000.
  Same margin TN52 used; we deliberately did **not** copy TN53's tighter
  8,400 margin because our epoch is longer and there's no cost to being
  generous.

**Why the Scenario-B treatment on devnet54 looks similar to TN52 but not
TN53:**

- TN52 was triggered intentionally in a test window, then left in EFM for
  ~29 h (about 2.5× the normal epoch length) — Scenario B, resolved with
  a long recovery epoch.
- TN53 was a real incident lasting < 12 h — basically Scenario A, Leo
  picked a vanilla 12 h recovery epoch.
- TN54 (this one) has been in EFM for ~21 days (~42 normal epoch lengths).
  Textbook Scenario B — we need Step 7 (timing-config update) and we have
  to accept that the on-chain counter won't line up with the "original
  schedule" counter any more.

### Scenario-B timing config update (to pass to `update_epoch_timing_config.cdc`)

| Argument | Value | Derivation |
|---|---|---|
| `duration` | `43200` seconds (12 h) | current testnet epoch duration — unchanged |
| `refCounter` | `3278` | recovery epoch counter |
| `refTimestamp` | `1795680000` (= `2026-11-26 00:00:00 PST`) | target recovery-epoch end time |

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
| P1 | No new extension since this plan was written | `consensus_compliance_current_epoch_final_view{network="devnet54"}` still = `137,120,565` |
| P2 | ≥ `2 × FinalizationSafetyThreshold` = 2,000 views remaining | `final_view − cur_view ≥ 2000` |
| P3 | All 14 CNs active (so `util` can emit a valid root cluster QC) | `up{role="collection", network="devnet54"} == 1` for all |
| P4 | Service-account signer available on the submitting machine | Operator's local `flow.json` with the testnet `service_account` alias loaded |

If P1 fails, re-derive params from the new extension's final view and the
new target end time — see "If a new extension is added before we submit"
in §1.

---

## 4. Execution plan

### Step 1 — pick an operator host & build the `util` binary

Operator host: **`devnet54-access-001`**. It carries the standard Flow
node IAM role for testnet, which already grants `s3:GetObject` on the
testnet bootstrap bucket and `kms:Decrypt` on the bootstrap CMK — the
two things the args-generation step needs. Instance ID can be looked up
at operation time via:

```bash
AWS_PROFILE=flow-testnet aws ec2 describe-instances --region us-east-2 \
  --filters Name=tag:Name,Values=devnet54-access-001 \
  --query 'Reservations[*].Instances[*].InstanceId' --output text
```

#### `util` version pin

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
**`v0.51.0-rc.1`**. This tag is a pre-release that lives on the internal
mirror, not on public `flow-go`. Build locally on a trusted workstation
(not on the AN), then copy the binary onto access-001 via SSM:

```bash
# On a trusted build host (not an AN):
git fetch origin tag v0.51.0-rc.1
git checkout v0.51.0-rc.1
CGO_ENABLED=1 go build -tags="relic,timetravel" -o /tmp/util ./cmd/util
file /tmp/util   # confirm linux/amd64 if target host is x86_64 Amazon Linux

# Push to operator host via SSM port-forward (replace <AN-INSTANCE-ID>):
AWS_PROFILE=flow-testnet aws ssm start-session \
  --region us-east-2 --target <AN-INSTANCE-ID> \
  --document-name AWS-StartPortForwardingSession \
  --parameters portNumber=22,localPortNumber=2222 &
scp -P 2222 /tmp/util ssm-user@localhost:/tmp/util
```

Inside an SSM shell session on access-001:

```bash
# Scratch space on tmpfs so private material never touches the data disk.
sudo mkdir -p /mnt/efm-recovery && sudo mount -t tmpfs -o size=512m tmpfs /mnt/efm-recovery
sudo mv /tmp/util /mnt/efm-recovery/util && sudo chmod +x /mnt/efm-recovery/util
cd /mnt/efm-recovery
./util epochs efm-recover-tx-args --help | head -5   # sanity
```

### Step 2 — assemble bootstrap tree on the operator host

The testnet bootstrap bucket is organized **per-node**:
`s3://<testnet-bootstrap-bucket>/<node>/bootstrap.tar.gz`. There is no
aggregated `node-internal-infos.pub.json` or shared
`private-root-information/` tree in the bucket yet. We synthesize both
from the per-node tars.

```bash
cd /mnt/efm-recovery
mkdir -p stage public-root-information private-root-information
export BOOTSTRAP_BUCKET=<testnet-bootstrap-bucket>   # see Terraform remote state

# Pull every internal (Foundation-run) node's bootstrap tar. Internal roles:
# CN, SN, AN, VN, EN — drop any partner-operated nodes if/when they return to
# testnet. For devnet54 today, all nodes in the bucket are internal.
ROLES="collection consensus access verification execution"
for role in $ROLES; do
  for n in $(aws s3 ls s3://${BOOTSTRAP_BUCKET}/ | awk '{print $2}' \
            | grep "^devnet54-${role}-" | tr -d '/'); do
    aws s3 cp s3://${BOOTSTRAP_BUCKET}/${n}/bootstrap.tar.gz stage/${n}.tar.gz
    mkdir -p stage/${n} && tar xzf stage/${n}.tar.gz -C stage/${n}
    # Each per-node tar typically carries:
    #   private-root-information/private-node-info.<node_id>.json
    #   public-root-information/node-info.pub.<node_id>.json
    cp -n stage/${n}/private-root-information/private-node-info.*.json \
       private-root-information/
    cp -n stage/${n}/public-root-information/node-info.pub.*.json \
       public-root-information/ 2>/dev/null || true
  done
done

# Build node-internal-infos.pub.json by concatenating the per-node pub infos.
# The util tool reads this as a JSON array of {NodeID,Role,Address,
# NetworkPubKey,StakingPubKey,Weight}.
jq -s '[ .[] | .[]? // . ]' public-root-information/node-info.pub.*.json \
   > public-root-information/node-internal-infos.pub.json
wc -l public-root-information/node-internal-infos.pub.json
ls private-root-information/ | wc -l    # expect 31 (14+7+2+5+3)
```

**Open question flagged for review:** the exact shape of
`node-internal-infos.pub.json` emitted by current `util bootstrap` may
differ slightly from a jq-concatenated form. If the generator refuses the
aggregated file, we fall back to downloading the authoritative file from
the previous spork artifact (location TBD — Leo kept his under
`~/bootstrap/public-root-information/` on the legacy GCP mgmt box; AWS
parity asset not yet confirmed). This fallback adds a cross-cloud lookup
step.

### Step 3 — generate recovery args JSON

Still inside SSM session on `devnet54-access-001`:

```bash
cd /mnt/efm-recovery
# Testnet AN network pubkey (published in the Notion runbook):
ACCESS_NETWORK_KEY=ba69f7d2e82b9edf25b103c195cd371cf0cc047ef8884a9bbe331e62982d46daeebf836f7445a2ac16741013b192959d8ad26998aff12f2adc67a99e1eb2988d
./util epochs efm-recover-tx-args \
  --access-address=access-001.devnet54.nodes.onflow.org:9001 \
  --access-network-key=${ACCESS_NETWORK_KEY} \
  --root-chain-id=flow-testnet \
  --collection-clusters=2 \
  --recovery-epoch-counter=3278 \
  --epoch-length=108000 \
  --epoch-staking-phase-length=96000 \
  --epoch-timing-recovery-duration=54000 \
  --config=/mnt/efm-recovery/public-root-information/node-internal-infos.pub.json \
  --internal-priv-dir=/mnt/efm-recovery/private-root-information/ \
  --out=/mnt/efm-recovery/devnet54_epoch_recovery.json

# Sanity: args file should contain non-empty clusterQCs, DKG result, etc.
jq 'keys' devnet54_epoch_recovery.json
```

Copy the JSON off the host (this file contains no secrets — only derived
public cluster QCs + DKG output):

```bash
# From operator laptop (replace <AN-INSTANCE-ID>):
AWS_PROFILE=flow-testnet aws ssm start-session \
  --target <AN-INSTANCE-ID> \
  --document-name AWS-StartPortForwardingSession \
  --parameters portNumber=22,localPortNumber=2222 &
scp -P 2222 ec2-user@localhost:/mnt/efm-recovery/devnet54_epoch_recovery.json ./
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
[`update_epoch_timing_config.cdc`][upd-timing] with the Scenario-B values from
§2.

[upd-timing]: https://github.com/onflow/flow-core-contracts/blob/master/transactions/epoch/admin/update_epoch_timing_config.cdc

Validate with
[`get_target_end_time_for_epoch.cdc`][get-ete] for counter `3278` → should
return `1795680000`.

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

```bash
# Inside SSM on access-001:
sudo umount /mnt/efm-recovery   # wipes tmpfs, including private material
sudo rmdir /mnt/efm-recovery
```

---

## 5. Rollback / failure branches

| Failure | Action |
|---|---|
| New extension added before Step 5 (P1 fails) | Re-run §2 with the new `LastExtensionFinalView`; drop in updated params. |
| `util` panics on args generation | Fix the flag per the panic, re-run; do NOT submit a transaction. |
| `recover_epoch.cdc` transaction panics (`FlowEpoch` unchanged) | Correct per the panic, rerun Step 3 → Step 5. Rewards have NOT been paid. |
| Transaction seals but `EpochRecover` service event is rejected | Per runbook Appendix: re-run Steps 1→5 with `--unsafe-overwrite-epoch-data`. Rewards paid in the first attempt are NOT re-paid. |
| View rate spikes post-recovery | Execute Step 8 cruise-control nudge. |
| Transaction never seals | Check `service_account` balance & sequence number; retry. |

---

## 6. Scope statement

- **Touches testnet only.** Mainnet is explicitly out of scope and no mainnet
  resources will be read or modified during this operation.
- **Reads** CN private-node-info files from the testnet bootstrap bucket into
  tmpfs on `devnet54-access-001` for the duration of args generation only.
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
4. Whether `jq`-concatenated `node-internal-infos.pub.json` is accepted by
   current `util epochs efm-recover-tx-args`. If not, a GCS fallback step is
   needed (see §4 Step 2 note). I will probe this on a dry-run before the
   live submission.
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
