message(STATUS "Bilinear pairings arithmetic configuration (PB module):\n")

message("   ** Options for the bilinear pairing module (default = on):")
message("      PB_PARAL=[off|on] Parallel implementation.\n")

option(PB_PARAL "Parallel implementation." off)

message("   ** Available bilinear pairing methods (default = ETATS):")
message("      PB_METHD=ETATS    Eta-t pairing in genus 1 with square roots.")
message("      PB_METHD=ETATN    Eta-t pairing in genus 1 without square roots.\n")

# Choose the arithmetic methods.
if (NOT PB_METHD)
	set(PB_METHD "ETATS")
endif(NOT PB_METHD)
list(LENGTH PB_METHD PB_LEN)
if (PB_LEN LESS 1)
	message(FATAL_ERROR "Incomplete PB_METHD specification: ${PB_METHD}")
endif(PB_LEN LESS 1)

list(GET PB_METHD 0 PB_MAP)
set(PB_METHD ${PB_METHD} CACHE STRING "Method for pairings over binary curves.")
