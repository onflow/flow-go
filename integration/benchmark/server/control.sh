#!/bin/bash

# if repo was previously cloned, this will fail cleanly and the script will continue
git clone https://github.com/onflow/flow-go.git

# assumes flow-go was already cloned by user

# need to add this, otherwise will get the following error when systemd executes git commands
# fatal: detected dubious ownership in repository at '/tmp/flow-go'
git config --global --add safe.directory /tmp/flow-go

cd flow-go

git fetch

git log  --merges --first-parent  --format=master:%H origin/master --since '1 week' | sort -R  | tee /tmp/master.recent

date +"Hello. The current date and time is %a %b %d %T %Z %Y" | tee -a /tmp/hello.txt
