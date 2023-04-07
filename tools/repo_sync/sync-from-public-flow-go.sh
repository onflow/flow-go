#!/bin/sh
set -ex
# need to set GitHub Actions bot user name and email to avoid "Committer identity unknown" error
# https://github.com/actions/checkout/discussions/479

git config --global user.email "github-actions[bot]@users.noreply.github.com"
git config --global user.name "github-actions"
git config pull.rebase false  # merge

# set up public flow-go as new remote
git remote add public-flow-go https://github.com/onflow/flow-go.git
git remote -v

####################### SYNC public flow-go/master to master-sync branch ################

# will be on default branch so need to switch to master-sync branch
git checkout master-sync

git pull origin

# pull latest commits from public repo
git pull public-flow-go master

# push latest commits from public repo to private repo
git push origin master-sync

# create PR to merge from master-sync => master-private branch
gh pr create --base master-private --title "[Sync] public \`flow-go/master\` → \`master-private\`" --body "Automated PR that merges updates from https://github.com/onflow/flow-go \`master\` branch &rarr; https://github.com/dapperlabs/flow-go \`master-private\` branch."

# create PR to merge from master-sync => to master-public branch
gh pr create --base master-public --title "[Sync] public \`flow-go/master\` → \`master-public\`" --body "Automated PR that merges updates from https://github.com/onflow/flow-go \`master\` branch &rarr; https://github.com/dapperlabs/flow-go \`master-public\` branch."
