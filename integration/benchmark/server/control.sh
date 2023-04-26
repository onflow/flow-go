#!/bin/bash

while :; do
  git fetch; git log  --merges --first-parent  --format=master:%H origin/master --since '1 week' | sort -R  | tee ~/master.recent
  sleep 86400;
done
