#!/bin/sh

RETVAL=""

# Get the architecture (CPU, OS) of the current system as a string.
# Only MacOS/x86_64 and Linux/x86_64 architectures are supported.
get_architecture() {
    local _ostype _cputype _arch
    _ostype="$(uname -s)"
    _cputype="$(uname -m)"
    if [ "$_ostype" = Darwin ] && [ "$_cputype" = i386 ]; then
        if sysctl hw.optional.x86_64 | grep -q ': 1'; then
            _cputype=x86_64
        fi
    fi
    case "$_ostype" in
        Linux)
            _ostype=linux
            ;;
        Darwin)
            _ostype=darwin
            ;;
        *)
            err "unrecognized OS type: $_ostype"
            ;;
    esac
    case "$_cputype" in
        x86_64 | x86-64 | x64 | amd64)
            _cputype=x86_64
            ;;
        *)
            err "unknown CPU type: $_cputype"
            ;;
    esac
    _arch="${_cputype}-${_ostype}"
    RETVAL="$_arch"
}

# Determine the system architecure, download the appropriate binary, and
# install it in `/usr/local/bin` with executable permission.
main() {
  get_architecture || exit 1
  arch="$RETVAL"

  url="https://storage.googleapis.com/flow-developer-preview-v1/flow-$arch"
  curl "$url" -o flow -s
  chmod +x ./flow
  mv ./flow /usr/local/bin
}

main
