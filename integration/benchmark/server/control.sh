#!/bin/bash

# assumes flow-go was already cloned by user

# need to add this, otherwise will get the following error when systemd executes git commands
# fatal: detected dubious ownership in repository at '/tmp/flow-go'
git config --system --add safe.directory /opt/flow-go

git fetch

# /opt/master.recent stores a list of git merge commit hashes of the master branch that will each be tested via benchmarking
# Sample:
# master:2735ae8dd46ea4d44131284747db849884126712
# master:c93a080ee384ee45e4ad9d414129a88829c26a49
# master:0fc9b0575494258ac3fdcfe00878ee78b2ce0630
# master:cb8564afbe23cffba03b0a40e2737ebe74e76138
# master:991b9a692156aa6258505024e4b18cafeac051de
git log  --merges --first-parent  --format=master:%H origin/master --since '1 week' | sort -R  | tee /opt/master.recent
