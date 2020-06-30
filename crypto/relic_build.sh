#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

mkdir -p $DIR/relic/build
cd $DIR/relic/build

# set RELIC config for Flow

#
GENERAL="-DTIMER=CYCLE -DCHECK=OFF -DVERBS=OFF"
LIBS="-DSHLIB=OFF -DSTLIB=ON"
COMP="-DCOMP=\"-O3 -funroll-loops -fomit-frame-pointer -march=native -mtune=native\""
RAND="-DRAND=HASHD -DSEED="

#
BN_REP="-DALLOC=AUTO -DALIGN=1 -DWSIZE=64 -DBN_PRECI=1024 -DBN_MAGNI=DOUBLE"
ARITH="-DARITH=EASY"  # try GMP or X64-ASM-382
PRIME="-DFP_PRIME=381"

#
#BN_METH="-DBN_KARAT=0 -DBN_METHD=\"COMBA;COMBA;MONTY;SLIDE;STEIN;STRON\""
#FP_METH="-DFP_KARAT=0 -DFP_METHD=\"INTEG;INTEG;INTEG;MONTY;LOWER;SLIDE\""
PRIMES="-DFP_PMERS=OFF -DFP_QNRES=ON -DFP_WIDTH=2"
#FPX_METH="-DFPX_METHD=\"INTEG;INTEG;LAZYR\""
CURVES="-DEP_MIXED=ON -DEP_PLAIN=OFF -DEP_SUPER=OFF -DEP_DEPTH=4 -DEP_WIDTH=2"
#PP_METH="-DPP_METHD=\"LAZYR;OATEP\""


cmake -DCOMP="-O3 -funroll-loops -fomit-frame-pointer -march=native -mtune=native" \
    $GENERAL $LIBS $RAND \
    $BN_REP $ARITH  $PRIME $PRIMES \
    $CURVES \
    -DBN_KARAT=0 -DBN_METHD="COMBA;COMBA;MONTY;SLIDE;STEIN;STRON" \
    -DFP_KARAT=0 -DFP_METHD="INTEG;INTEG;INTEG;MONTY;LOWER;SLIDE" \
    -DFPX_METHD="INTEG;INTEG;LAZYR" \
    -DPP_METHD="LAZYR;OATEP" ..

# compile the static library
make clean
make relic_s -j8
rm -f CMakeCache.txt
cd ../..
