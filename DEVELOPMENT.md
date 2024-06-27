# Development

## Feature Release Flow

### Branching Strategy

#### master branch

- The `master` branch is intended to only contain features for the immediately upcoming release, whether it is a [Height Coordinated Upgrade (HCU) or a Spork](https://developers.flow.com/networks/node-ops/node-operation/hcu#hcu-versus-spork).

#### HCU or Spork specific branches

- For every HCU and spork, a branch will be created from master. This branch will be tagged and used to update testnet and then mainnet.
- Only non-breaking changes which do not require an HCU or a spork can be committed to these HCU or spork specific branches.

#### feature branches
- During development, all features should live on a feature branch.
- For small features, this will be a simple working branch. These branches have the naming scheme `<contributor>/<issue #>-<short description>, for example `kan/123-fix-known-issue`
- For larger features, this may be a shared feature branch with other team members. Feature branches have the naming scheme `feature/<name>`.

### Upgrade Path Eligibility

- When a feature branch is ready to be merged, the desired upgrade path onto Mainnet must be determined (if any). The options are:
    - Height Coordinated Upgrade (HCU)
        - No protocol-level breaking changes
        - No state migrations
        - Changes to how Execution state/path are handled are allowed if they are
            - Backwards compatible, or
            - Brand new additions
        - Resource optimizations are okay
        - Cadence upgrades which could cause execution state fork (likely any Cadence upgrade except for trivial changes)
    - Spork
        - Protocol level breaking change
        - State migrations required
- All HCU upgrades can go directly into the `master` branch
- All spork upgrades must live on their own feature branch until the last HCU before the spork has been performed (usually approximately 1 month before the Spork).
    - It is the responsibility of the DRI to keep this feature branch in a mergeable state.
    - If the spork is scheduled to occur within a month, all the feature branches can be merged into `master`. However, if the exact spork date has not been decided then a special spork branch may be created from master to merge all the feature branches. This is to consolidate of all the feature branches while accommodating any additional HCUs that may occur between then and the spork date.
    - Suggestion: once a sprint, merge `master` into the feature branch. More frequent merges are easier, as they avoid complex conflict resolutions


### End of Release Cycle

- At the end of every release cycle, we will tag a commit that includes all desired features to be released
- This commit will be tagged according to [semantic versioning guidelines](https://dapperlabs.notion.site/Changes-to-handling-git-tags-5e39af7c723a428a915bd88901fc1274)
- Release notes must be written up, describing all features included in this tag

### Benchmark Testing

[Benchmarking](https://www.notion.so/Benchmarking-e3d89e3aadb44b0787da9bb7703b0dae?pvs=21)

- All features on the release candidate tag must undergo testing on `benchmarknet`

### Testnet

- The current schedule is the Wednesday two weeks before the corresponding Mainnet spork
- Features should aim to live on Testnet for at least two weeks before making it to Mainnet

### Mainnet

- Features must live on Testnet for two week before making it to Mainnet
- The current schedule is the Wednesday two weeks after the Testnet Spork

## Breaking Change Classifications

### Acceptable Changes for HCU

- All backward compatible changes
- Breaking changes only pertaining to the execution of future transactions
    - Many Cadence related breaking changes would fall in this category
    - FVM changes may also fall here
- Breaking changes only pertaining to the verification of future transactions

### Spork only changes

- Any change that requires a state migration
    - i.e. something changing in how the historical state will be read and interacted with
- Any change that would break the communication contract between nodes
    - e.g. Addition of a new REQUIRED field in a message structure
    - Removal of a REQUIRED channel in libp2p
    - Removal of a REQUIRED field in a message structure
        -  Generally, *all* the fields in our node-to-node messages are required.
        -  For BFT reasons we avoid optional fields, as they add surface for spamming attacks, impose additional consistency requirements, and they often add security vulnerabilities w.r.t. the message hash/ID. 
    - Most changes of the core protocol outside of execution, such as changes in the consensus algorithm, verification-and-sealing pipeline, or collector mechanics.
        - For any protocol-related changes, you need to have a solid argument for why this change is non-breaking (absence of a counter-example is not sufficient).
    - Changes in the [Service Events](https://www.notion.so/Service-Events-54e5edb7515445f293dff36ade910ad7?pvs=21) that are emitted by the Execution environment and ingested by the protocol
    - Change in the reading of Protocol state that is not backwards compatible
        - e.g. If the way the node interprets identities loaded from storage is changed so that identities stored before an upgrade is no longer recognized after the upgrade, this is not acceptable for an HCU
