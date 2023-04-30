#!/bin/bash

# assumes flow-go was already cloned and git was configured to allow systemd to issue git commands with
# git config --global --add safe.directory /tmp/flow-go

cd flow-go
git fetch
git log  --merges --first-parent  --format=master:%H origin/master --since '1 week' | sort -R  | tee /tmp/master.recent

echo "Hello. The current date and time is " | tee -a /tmp/hello.txt
date | tee -a /tmp/hello.txt
