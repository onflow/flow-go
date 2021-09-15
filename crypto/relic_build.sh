#!/bin/bash

set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

pushd "$DIR"

# Ensure the directory is writeable
chmod -R +w "$(pwd)"

mkdir -p "$DIR/relic/build"
pushd "$DIR/relic/build"


# make cmake print its CC interpretation
CMAKE_FILE="${DIR}/relic/CMakeLists.txt"
# parameter expansion is not suitable here
# shellcheck disable=SC2089
CMAKE_PRINT_CC="message ( STATUS \"CC=\$ENV{CC}\" )"
# Make the cmake run print its interpretation of CC
echo "$CMAKE_PRINT_CC" >> "${CMAKE_FILE}"

# Probe cmake's MakeFile generation and extract the CC version
CMAKE_TEMP=$(mktemp)
cmake .. > "$CMAKE_TEMP"
CC_VAL="$(tail -n 5 "$CMAKE_TEMP" | grep -oE -m 1 'CC=.*$')"
CC_VAL="${CC_VAL:3}"

# de-mangle the CMakeLists file, using a temporary file for BSD compatibility
sed '$d' ../CMakeLists.txt > "$CMAKE_TEMP"
mv "$CMAKE_TEMP" ../CMakeLists.txt

# default to which
CC_VAL=${CC_VAL:-"$(which cc)"}
CC_VERSION_STR="$($CC_VAL --version)"

# we use uname to record which arch we are running on
ARCH=$(uname -m 2>/dev/null ||true)

if [[ "$ARCH" =~ ^(arm64|armv7|armv7s)$ && "${CC_VERSION_STR[0]}" =~ (clang)  ]]; then
    #  the "-march=native" option is not supported with clang on ARM
    MARCH=""
else
    # Target standard x86-64 CPU. Avoids AVX512 SIGILL: illegal instruction
    MARCH="-march=x86-64"
fi

# Set RELIC config for Flow
COMP=(-DCFLAGS="-O3 -funroll-loops -fomit-frame-pointer ${MARCH} -mtune=native")
GENERAL=(-DTIMER=CYCLE -DCHECK=OFF -DVERBS=OFF)
LIBS=(-DSHLIB=OFF -DSTLIB=ON)
RAND=(-DRAND=HASHD -DSEED=)

#
BN_REP=(-DALLOC=AUTO -DALIGN=1 -DWSIZE=64 -DBN_PRECI=1024 -DBN_MAGNI=DOUBLE)
ARITH=(-DARITH=EASY)
PRIME=(-DFP_PRIME=381)

#
BN_METH=(-DBN_KARAT=0 -DBN_METHD="COMBA;COMBA;MONTY;SLIDE;STEIN;STRON")
FP_METH=(-DFP_KARAT=0 -DFP_METHD="INTEG;INTEG;INTEG;MONTY;LOWER;SLIDE")
PRIMES=(-DFP_PMERS=OFF -DFP_QNRES=ON -DFP_WIDTH=2)
FPX_METH=(-DFPX_METHD="INTEG;INTEG;LAZYR")
EP_METH=(-DEP_MIXED=ON -DEP_PLAIN=OFF -DEP_SUPER=OFF -DEP_DEPTH=4 -DEP_WIDTH=2 \
    -DEP_CTMAP=ON -DEP_METHD="JACOB;LWNAF;COMBS;INTER")
PP_METH=(-DPP_METHD="LAZYR;OATEP")

# run cmake
cmake "${COMP[@]}" "${GENERAL[@]}" \
        "${LIBS[@]}" "${RAND[@]}" \
        "${BN_REP[@]}" "${ARITH[@]}" \
        "${PRIME[@]}" "${PRIMES[@]}" \
        "${EP_METH[@]}" \
        "${BN_METH[@]}" \
        "${FP_METH[@]}" \
        "${FPX_METH[@]}" \
        "${PP_METH[@]}" ..


# Compile the static library
make clean
make relic_s -j8
rm -f CMakeCache.txt

popd
popd
