# This script can be imported by Makefiles in order to set the `CRYPTO_FLAG` automatically for 
# a native build (build and run on the same machine NOT for cross-compilation).
#
# The `CRYPTO_FLAG` is a Go command flag that should be used when the target machine's CPU executing 
# the command may not support ADX instructions.
# For new machines that support ADX instructions, the `CRYPTO_FLAG` flag is not needed (or set
# to an empty string).  

# First detect ADX support:
# `ADX_SUPPORT` is 1 if ADX instructions are supported on the current machine and 0 otherwise.
ifeq ($(shell uname -s),Linux)
# detect ADX support on the CURRENT linux machine.
	ADX_SUPPORT := $(shell if ([ -f "/proc/cpuinfo" ] && grep -q -e '^flags.*\badx\b' /proc/cpuinfo); then echo 1; else echo 0; fi)
else
# on non-linux machines, set the flag to 1 by default
	ADX_SUPPORT := 1
endif

DISABLE_ADX := "-O2 -D__BLST_PORTABLE__"

# Then, set `CRYPTO_FLAG`
# the crypto package uses BLST source files underneath which may use ADX instructions.
ifeq ($(ADX_SUPPORT), 1)
# if ADX instructions are supported on the current machine, default is to use a fast ADX implementation 
	CRYPTO_FLAG := ""
else
# if ADX instructions aren't supported, this CGO flags uses a slower non-ADX implementation 
	CRYPTO_FLAG := $(DISABLE_ADX)
endif