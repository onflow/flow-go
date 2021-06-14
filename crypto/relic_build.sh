#!/bin/bash

set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

pushd "$DIR"

# Ensure the directory is writeable
chmod -R +w "$(pwd)"

mkdir -p "$DIR/relic/build"
pushd "$DIR/relic/build"

# Set RELIC config for Flow

#
GENERAL=(-DTIMER=CYCLE -DCHECK=OFF -DVERBS=OFF)
LIBS=(-DSHLIB=OFF -DSTLIB=ON)

# "-march=native" is not supported on ARM while using clang, we use uname
# to record which arch we are running on
ARCH=$(uname -m 2>/dev/null ||true)
ARCH=${ARCH:-x86_64}

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

# "-march=native" is not supported on ARM by clang
# we probe the cmake compiler detection logic by running cmake once with
# the permissive logic, and adding the '-march=native' option only if
# supported
PROBE_COMP=(-DCOMP="-O3 -funroll-loops -fomit-frame-pointer -mtune=native")

# make cmake print its CC interpretation
echo 'message ( STATUS "CC=$ENV{CC}" )' >> ../CMakeLists.txt

# Probe cmake's MakeFile generation
CMAKE_OUTPUT=$(mktemp)
cmake "${PROBE_COMP[@]}" "${GENERAL[@]}" \
    "${LIBS[@]}" "${RAND[@]}" \
    "${BN_REP[@]}" "${ARITH[@]}" \
    "${PRIME[@]}" "${PRIMES[@]}" \
    "${EP_METH[@]}" \
    "${BN_METH[@]}" \
    "${FP_METH[@]}" \
    "${FPX_METH[@]}" \
    "${PP_METH[@]}" .. > $CMAKE_OUTPUT
# de-mangle the CMakeLists file, done with a temp file
# to be portable between GNU / BSD sed
CMAKE_TEMP=$(mktemp)
sed -e '/message ( STATUS "CC=$ENV{CC}" )/d' ../CMakeLists.txt > $CMAKE_TEMP
mv $CMAKE_TEMP ../CMakeLists.txt

# extract what cmake uses as CC and probe its version string
CC_VAL="$(tail -n 5 "$CMAKE_OUTPUT" | grep -oE 'CC=.*$' | sed -re 's/CC=(.*)/\1/g')"
# default to which
CC_VAL=${CC_VAL:-"$(which cc)"}
CC_VERSION_STR=="$($CC_VAL --version)"

if [[ "$ARCH" =~ ^(arm64|armv7|armv7s)$ && "${CC_VERSION_STR[0]}" =~ (clang)  ]]; then
    # clang on ARM => the "-march=native" option is not supported and our
    # probing run contained the correct information, we just display it
    cat $CMAKE_OUTPUT
else
    # we can use "-march=native" and re-run cmake accordingly
    COMP=(-DCOMP="-O3 -funroll-loops -fomit-frame-pointer -march=native -mtune=native")
    cmake "${COMP[@]}" "${GENERAL[@]}" \
          "${LIBS[@]}" "${RAND[@]}" \
          "${BN_REP[@]}" "${ARITH[@]}" \
          "${PRIME[@]}" "${PRIMES[@]}" \
          "${EP_METH[@]}" \
          "${BN_METH[@]}" \
          "${FP_METH[@]}" \
          "${FPX_METH[@]}" \
          "${PP_METH[@]}" ..
fi


# Compile the static library
make clean
make relic_s -j8
rm -f CMakeCache.txt

popd
popd
