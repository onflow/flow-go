#!/bin/bash

# assumes flow-go was already cloned by user

# need to add this, otherwise will get the following error when systemd executes git commands
# fatal: detected dubious ownership in repository at '/tmp/flow-go'
# git config --system --add safe.directory /opt/flow-go

git fetch

commits_file="/opt/commits.recent"
# the commits_file stores a list of git merge commit hashes of any branches that will each be tested via benchmarking
# and the load they will be tested with
# Sample:
# master:2735ae8dd46ea4d44131284747db849884126712:token-transfer

# clear the file
: > $commits_file

# the load_types array stores the different types of loads that will be run on the commits
load_types=("token-transfer" "evm-transfer")

# get the merge commits from the last week from master ordered by author date
for commit in $(git log  --merges --first-parent  --format="%S:%H" origin/master --since '1 week' --author-date-order )
do
  for load in "${load_types[@]}"
  do
    echo "$commit:$load" | tee -a $commits_file
  done
done
