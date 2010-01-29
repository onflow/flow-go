message(STATUS "Pairing-based cryptography configuration (PC module):\n")

message("   ** Available pairing computation methods (default = PRIME):")
message("      PC_METHD=PRIME    Use prime (asymmetric) setting.")
message("      PC_METHD=BINAR    Use binary (symmetric) setting.\n")

# Choose the arithmetic methods.
if (NOT PC_METHD)
	set(PC_METHD "PRIME")
endif(NOT PC_METHD)
list(LENGTH PC_METHD PC_LEN)
if (PC_LEN LESS 1)
	message(FATAL_ERROR "Incomplete PC_METHD specification: ${PC_METHD}")
endif(PC_LEN LESS 1)

list(GET PC_METHD 0 PC_CUR )
set(PC_METHD ${PC_METHD} CACHE STRING "Choice of pairing setting.")
