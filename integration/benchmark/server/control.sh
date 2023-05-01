#!/bin/bash

# assumes flow-go was already cloned by user

# need to add this, otherwise will get the following error when systemd executes git commands
# fatal: detected dubious ownership in repository at '/tmp/flow-go'
git config --system --add safe.directory /tmp/flow-go

cd flow-go

git fetch

git log  --merges --first-parent  --format=master:%H origin/master --since '1 week' | sort -R  | tee /opt/master.recent
