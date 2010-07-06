message(STATUS "Prime elliptic curve arithmetic configuration (EP module):\n")

message("   ** Options for the binary elliptic curve module (default = all on):")
message("      EP_ORDIN=[off|on] Support for ordinary curves.")
message("      EP_SUPER=[off|on] Support for supersingular curves.")    
message("      EP_MIXED=[off|on] Use mixed coordinates.\n")
message("      EB_PRECO=[off|on] Build precomputation table for generator.")
message("      EB_DEPTH=w        Width w in [2,6] of precomputation table for fixed point methods.")
message("      EB_WIDTH=w        Width w in [2,6] of window processing for unknown point methods.\n")

message("   ** Available binary elliptic curve methods (default = PROJC;WTNAF;COMBS;INTER):")
message("      EP_METHD=BASIC    Affine coordinates.")
message("      EP_METHD=PROJC    Jacobian projective coordinates.\n")
 
message("      EB_METHD=BASIC    Binary method.")
message("      EB_METHD=WTNAF    Window (T)NAF method.\n")

message("      EB_METHD=BASIC    Binary method for fixed point multiplication.")
message("      EB_METHD=YAOWI    Yao's windowing method for fixed point multiplication")
message("      EB_METHD=NAFWI    NAF windowing method for fixed point multiplication.")
message("      EB_METHD=COMBS    Single-table Comb method for fixed point multiplication.")
message("      EB_METHD=COMBD    Double-table Comb method for fixed point multiplication.")
message("      EB_METHD=WTNAF    Window NAF with width w (TNAF for Koblitz curves).\n")

message("      EB_METHD=BASIC    Multiplication-and-addition simultaneous multiplication.")
message("      EB_METHD=TRICK    Shamir's trick for simultaneous multiplication.")
message("      EB_METHD=INTER    Interleaving of w-(T)NAFs.")
message("      EB_METHD=JOINT    Joint sparse form.\n")

message("      Note: these methods must be given in order. Ex: EB_METHD=\"BASIC;WTNAF;COMBD;TRICK\"\n")

if (NOT EP_DEPTH)
	set(EP_DEPTH 4)
endif(NOT EP_DEPTH)	
if (NOT EP_WIDTH)
	set(EP_WIDTH 4)
endif(NOT EP_WIDTH)	
set(EP_DEPTH "${EP_DEPTH}" CACHE STRING "Width of precomputation table for fixed point methods.")
set(EP_WIDTH "${EP_WIDTH}" CACHE STRING "Width of window processing for unknown point methods.")

option(EP_ORDIN "Support for ordinary curves" on)
option(EP_SUPER "Support for supersingular curves" on)
option(EP_MIXED "Use mixed coordinates" on)
option(EP_PRECO "Build precomputation table for generator" on)

# Choose the arithmetic methods.
if (NOT EP_METHD)
	set(EP_METHD "PROJC;WTNAF;COMBS;INTER")
endif(NOT EP_METHD)
list(LENGTH EP_METHD EP_LEN)
if (EP_LEN LESS 4)
	message(FATAL_ERROR "Incomplete EP_METHD specification: ${EP_METHD}")
endif(EP_LEN LESS 4)

list(GET EP_METHD 0 EP_ADD)
list(GET EP_METHD 1 EP_MUL)
list(GET EP_METHD 2 EP_FIX)
list(GET EP_METHD 3 EP_SIM)
set(EP_METHD ${EP_METHD} CACHE STRING "Prime elliptic curve arithmetic method.")
