#!/bin/bash
#
# Cadence - The resource-oriented smart contract programming language
#
# Copyright 2019-2020 Dapper Labs, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -o pipefail

function ctrl_c() {
  kill % &> /dev/null
  kill %kserver &> /dev/null
  printf "\n"
  exit 0
}

trap ctrl_c INT

kserver &> /dev/null &

export OPAMROOT=/usr/lib/kframework/lib/opamroot
cd $(dirname $0)
export KRUN_COMPILED_DEF="$(cd ..;pwd)"

if [ -t 1 ]; then
  FANCY=true
else
  FANCY=false
fi

test_parse () { kast "$1"; }

for f in parser/valid/*.fpl; do
  if [ "$FANCY" = true ]; then
    printf "\e[2K\rRUNNING %.$(($(tput cols)-8))s" "$f"
  fi
  test_parse $f &> /dev/null &
  if ! wait % ; then
    if [ "$FANCY" = true ]; then printf "\e[2K\r"; fi
    printf "FAIL %s\n" "$f" 
    sleep 1
  fi
done

for f in parser/invalid/*.fpl; do
  if [ "$FANCY" = true ]; then
    printf "\e[2K\rRUNNING %.$(($(tput cols)-8))s" "$f"
  fi
  test_parse $f &> /dev/null &
  if wait % ; then
    if [ "$FANCY" = true ]; then printf "\e[2K\r"; fi
    printf "FAIL %s\n" "$f" 
  fi
done

for f in interpreter/*.fpl; do
  if [ "$FANCY" = true ]; then
    printf "\e[2K\rRUNNING %.$(($(tput cols)-8))s" "$f"
  fi
  krun $f | diff - interpreter/output.txt &
  if ! wait % ; then
    if [ "$FANCY" = true ]; then printf "\e[2K\r"; fi
    printf "FAIL %s\n" "$f" 
  fi
done

for f in interpreter/panic/*.fpl; do
  if [ "$FANCY" = true ]; then
    printf "\e[2K\rRUNNING %.$(($(tput cols)-8))s" "$f"
  fi
  krun $f | diff - interpreter/panic/output.txt &
  if ! wait % ; then
    if [ "$FANCY" = true ]; then printf "\e[2K\r"; fi
    printf "FAIL %s\n" "$f" 
  fi
done

if [ "$FANCY" = true ]; then printf "\e[2K\r"; fi
echo DONE

kill %kserver &> /dev/null
