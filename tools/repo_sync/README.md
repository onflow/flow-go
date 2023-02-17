# Branches Used for Public-Private Repo Syncing

- `master-sync`
  - branch that is auto synced from https://github.com/onflow/flow-go `master` branch, via `git push`
  - doesn’t contain anything else besides a synced version of the https://github.com/onflow/flow-go `master` branch
  - used as the source branch to sync new commits from https://github.com/onflow/flow-go master to
    - `master-public` (via auto generated PR)
    - `master-private` (via `git push`)

- `master-public`
  - mirror of https://github.com/onflow/flow-go `master` branch + PR merges from https://github.com/dapperlabs/flow-go that are meant for eventual merging to https://github.com/onflow/flow-go
  - this branch will be used to create PRs against https://github.com/onflow/flow-go (via fork of https://github.com/onflow/flow-go as [described here](https://www.notion.so/Synchronizing-Flow-Public-Private-Repos-a0637f89eeed4a80ab91620766d5a58b#fb50ac16e58949a7a618a4afd733a836))
  - has same branch protections as https://github.com/onflow/flow-go `master` branch so that PRs can be fully tested before they are merged
  - doesn’t work with `git push` because of branch protections so a manual PR merge is required (which is auto created via `master-private` branch)

- `master-private`
  - mirror of https://github.com/onflow/flow-go `master` branch + PR merges from https://github.com/dapperlabs/flow-go for permanently private code
  - the **default branch** so that syncs can be run on a schedule in GitHub Actions which only work on default branches
  - contains CI related syncing workflows and scripts used to sync https://github.com/onflow/flow-go `master` branch with https://github.com/dapperlabs/flow-go branches:
    - auto syncs https://github.com/dapperlabs/flow-go `master-sync` branch with https://github.com/onflow/flow-go `master` via `git push`
    - auto merges syncs from https://github.com/dapperlabs/flow-go `master-sync` to https://github.com/dapperlabs/flow-go `master-private`
    - auto creates PRs from https://github.com/dapperlabs/flow-go `master-sync` to https://github.com/dapperlabs/flow-go `master-public` that are manually merged

- `master-old` - former `master` branch of https://github.com/dapperlabs/flow-go which has some extra security scanning workflows

- feature branches for code that will eventually be merged to ‣ master
  - will be branched from and merged to `master-public`
  - will require the same rules to be merged to `master-public` (i.e. 2 approvals, pass all tests) as for https://github.com/onflow/flow-go `master` (to minimize how long PRs against https://github.com/onflow/flow-go `master` stay open, since they will contain vulnerabilities that we want to merge to https://github.com/onflow/flow-go `master` ASAP)

- feature branches for code that will be permanently private
  - will be branched from and merged to `master-private`

Further updates will be in [Notion](https://www.notion.so/dapperlabs/Synchronizing-Flow-Public-Private-Repos-a0637f89eeed4a80ab91620766d5a58b?pvs=4#e8e9a899a8854520a2cdba324d02b97c)
