message(STATUS "Prime elliptic curve arithmetic configuration (EP module):\n")

message("   *** Options for the binary elliptic curve module (default = all on):")
message("       EP_ORDIN=[off|on] Support for ordinary curves.")
message("       EP_SUPER=[off|on] Support for supersingular curves.")    
message("       EP_STAND=[off|on] Support for standardized curves.")
message("       EP_MIXED=[off|on] Use mixed coordinates.\n")

option(EP_ORDIN "Support for ordinary curves" on)
option(EP_SUPER "Support for supersingular curves" on)
option(EP_STAND "Support for NIST standardized curves" on)
option(EP_MIXED "Use mixed coordinates" on)

message("   *** Available binary elliptic curve methods (default = PROJC;W4NAF):")
message("       EP_METHD=BASIC    Affine coordinates.")
message("       EP_METHD=PROJC    López-Dahab Projective coordinates.\n") 
message("       EP_METHD=BASIC    Binary method.")
message("       Note: these methods must be given in order. Ex: EP_METHDD=\"BASIC;BASIC\"\n")

# Choose the arithmetic methods.
if (NOT EP_METHD)
	set(EP_METHD "BASIC;BASIC")
endif(NOT EP_METHD)
list(LENGTH EP_METHD EP_LEN)
if (EP_LEN LESS 2)
	message(FATAL_ERROR "Incomplete EP_METHD specification: ${EP_METHD}")
endif(EP_LEN LESS 2)

list(GET EP_METHD 0 EP_ADD)
list(GET EP_METHD 1 EP_MUL)
set(EP_METHD ${EP_METHD} CACHE STRING "Prime elliptic curve arithmetic method.")
