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


#git log  --merges --first-parent  --format="%S:%H:token-transfer" origin/master --since '1 week' | sort -R  | tee -a $commits_file

# example for all commits on a single branch since master since 1 week ago with the load set to evm
git log   --first-parent  --format="%S:%H:evm-transfer" origin/master..janez/tps-benchmark-evm-load --since '1 week' --author-date-order | head -n 1 | tee -a $commits_file
