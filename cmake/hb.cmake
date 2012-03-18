message(STATUS "Binary hyperelliptic genus 2 curve arithmetic configuration (HB module):\n")

message("   ** Options for the binary elliptic curve module (default = on, w = 4):")
message("      HB_SUPER=[off|on] Support for supersingular curves.")    
message("      HB_PRECO=[off|on] Build precomputation table for generator.")

option(HB_SUPER "Support for supersingular curves" on)
option(HB_PRECO "Build precomputation table for generator" on)

message("      EB_DEPTH=w        Width w in [2,6] of precomputation table for fixed point methods.")
message("      HB_WIDTH=w        Width w in [2,6] of window processing for unknown point methods.\n")

message("   ** Available binary elliptic curve methods (default = PROJC;LWNAF;COMBS;INTER):")
message("      HB_METHD=BASIC    Affine coordinates.")
message("      HB_METHD=PROJC    Projective coordinates (López-Dahab for ordinary curves).\n")

message("      HB_METHD=BASIC    Binary method.")
message("      HB_METHD=OCTUP    Octupling-based windowing method.\n")

message("      HB_METHD=BASIC    Binary method for fixed point multiplication.")
message("      HB_METHD=YAOWI    Yao's windowing method for fixed point multiplication")
message("      HB_METHD=NAFWI    NAF windowing method for fixed point multiplication.")
message("      HB_METHD=COMBS    Single-table Comb method for fixed point multiplication.")
message("      HB_METHD=COMBD    Double-table Comb method for fixed point multiplication.")
message("      HB_METHD=LWNAF    Window NAF with width w (TNAF for Koblitz curves).\n")

message("      HB_METHD=BASIC    Multiplication-and-addition simultaneous multiplication.")
message("      HB_METHD=TRICK    Shamir's trick for simultaneous multiplication.")
message("      HB_METHD=INTER    Interleaving of window NAFs.")
message("      HB_METHD=JOINT    Joint sparse form.\n")

message("      Note: these methods must be given in order. Ex: HB_METHD=\"BASIC;LWNAF;COMBD;TRICK\"\n")

if (NOT HB_DEPTH)
	set(HB_DEPTH 4)
endif(NOT HB_DEPTH)	
if (NOT HB_WIDTH)
	set(HB_WIDTH 4)
endif(NOT HB_WIDTH)	
set(HB_DEPTH "${HB_DEPTH}" CACHE STRING "Width of precomputation table for fixed point methods.")
set(HB_WIDTH "${HB_WIDTH}" CACHE STRING "Width of window processing for unknown point methods.")

option(HB_ORDIN "Support for ordinary curves" on)
option(HB_SUPER "Support for supersingular curves" on)
option(HB_PRECO "Build precomputation table for generator" on)

# Choose the arithmetic methods.
if (NOT HB_METHD)
	set(HB_METHD "BASIC;OCTUP;COMBS;INTER")
endif(NOT HB_METHD)
list(LENGTH HB_METHD HB_LEN)
if (HB_LEN LESS 4)
	message(FATAL_ERROR "Incomplete HB_METHD specification: ${HB_METHD}")
endif(HB_LEN LESS 4)

list(GET HB_METHD 0 HB_ADD)
list(GET HB_METHD 1 HB_MUL)
list(GET HB_METHD 2 HB_FIX)
list(GET HB_METHD 3 HB_SIM)
set(HB_METHD ${HB_METHD} CACHE STRING "Method for binary hyperelliptic curve arithmetic.")
