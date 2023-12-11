# `ADX_SUPPORT` is 1 if ADX instructions are supported and 0 otherwise.
ifeq ($(shell uname -s),Linux)
# detect ADX support on the CURRENT linux machine.
	ADX_SUPPORT := $(shell if ([ -f "/proc/cpuinfo" ] && grep -q -e '^flags.*\badx\b' /proc/cpuinfo); then echo 1; else echo 0; fi)
else
# on non-linux machines, set the flag to 1 by default
	ADX_SUPPORT := 1
endif

# the crypto package uses BLST source files underneath which may use ADX instructions.
ifeq ($(ADX_SUPPORT), 1)
# if ADX instructions are supported, default is to use a fast ADX BLST implementation 
	CRYPTO_FLAG := ""
else
# if ADX instructions aren't supported, this CGO flags uses a slower non-ADX BLST implementation 
	CRYPTO_FLAG := "-O -D__BLST_PORTABLE__"
endif