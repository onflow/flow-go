#!/bin/bash

set -o pipefail

function ctrl_c() {
  kill % &> /dev/null
  printf "\n"
  exit 0
}

trap ctrl_c INT

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

if [ "$FANCY" = true ]; then printf "\e[2K\r"; fi
echo DONE
