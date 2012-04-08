message(STATUS "Bilinear pairings arithmetic configuration (PP module):\n")

message("   ** Options for the bilinear pairing module (default = on):")
message("      PP_PARAL=[off|on] Parallel implementation.\n")

option(PP_PARAL "Parallel implementation." off)

message("   ** Available bilinear pairing methods (default = BASIC;BASIC;BASIC;OATEP):")

message("      PP_METHD=BASIC    Basic quadratic extension field arithmetic.")    
message("      PP_METHD=INTEG    Quadratic extension field arithmetic with embedded modular reduction.\n")

message("      PP_METHD=BASIC    Basic cubic extension field arithmetic.")    
message("      PP_METHD=INTEG    Cubic extension field arithmetic with embedded modular reduction.\n")

message("      PP_METHD=BASIC    Basic extension field arithmetic.")    
message("      PP_METHD=LAZYR    Lazy reduced extension field arithmetic.\n")

message("      PP_METHD=TATEP    Tate pairing.")
message("      PP_METHD=WEILP    Weil pairing.")
message("      PP_METHD=OATEP    Optimal ate pairing.\n")

# Choose the arithmetic methods.
if (NOT PP_METHD)
	set(PP_METHD "BASIC;BASIC;BASIC;OATEP")
endif(NOT PP_METHD)
list(LENGTH PP_METHD PP_LEN)
if (PP_LEN LESS 4)
	message(FATAL_ERROR "Incomplete PP_METHD specification: ${PP_METHD}")
endif(PP_LEN LESS 4)

list(GET PP_METHD 0 PP_QDR)
list(GET PP_METHD 1 PP_CBC)
list(GET PP_METHD 2 PP_EXT)
list(GET PP_METHD 3 PP_MAP)
set(PP_METHD ${PP_METHD} CACHE STRING "Method for pairing over prime curves.")
