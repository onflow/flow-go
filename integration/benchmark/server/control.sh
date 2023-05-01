#!/bin/bash

# if repo was previously cloned, this will fail cleanly and the script will continue
git clone https://github.com/onflow/flow-go.git

cd flow-go

# temporary - need to use this branch while changes not yet merged to master
git checkout misha/4015-move-tps-to-new-vm

git fetch
git log  --merges --first-parent  --format=master:%H origin/master --since '1 week' | sort -R  | tee /tmp/master.recent

date +"Hello. The current date and time is %a %b %d %T %Z %Y" | tee -a /tmp/hello.txt
