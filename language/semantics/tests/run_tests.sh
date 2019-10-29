#!/bin/bash
trap 'exit 0' INT

export OPAMROOT=/usr/lib/kframework/lib/opamroot
cd $(dirname $0)
export KRUN_COMPILED_DEF="$(cd ..;pwd)"
echo $KRUN_COMPILED_DEF
test_parse () { kast "$1"; }

if [ -t 1 ]; then
  FANCY=true
else
  FANCY=false
fi

for f in parser/valid/*.bpl; do
  if [ "$FANCY" = true ]; then
    printf "\e[2K\rRUNNING %.$(($(tput cols)-8))s" "$f"
  fi
  if ! test_parse $f &> /dev/null; then
    if [ "$FANCY" = true ]; then printf "\e[2K\r"; fi
    printf "FAIL %s\n" "$f" 
  fi
done

for f in parser/invalid/*.bpl; do
  if [ "$FANCY" = true ]; then
    printf "\e[2K\rRUNNING %.$(($(tput cols)-8))s" "$f"
  fi
  if test_parse $f &> /dev/null; then
    if [ "$FANCY" = true ]; then printf "\e[2K\r"; fi
    printf "FAIL %s\n" "$f" 
  fi
done
if [ "$FANCY" = true ]; then printf "\e[2K\r"; fi
echo DONE
