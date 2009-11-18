message(STATUS "Bilinear pairings arithmetic configuration (PP module):\n")

message("   ** Options for the bilinear pairing module (default = on):")
message("      PP_PARAL=[off|on] Parallel implementation.\n")

option(PP_PARAL "Parallel implementation." on)

message("   ** Available bilinear pairing methods (default = X-ATE):")
#message("       PP_METHD=WEILP    Weil pairing.")
#message("       PP_METHD=TATEP    Tate pairing.")
#message("       PP_METHD=ATE-I    Ate-i pairng.")
#message("       PP_METHD=ATE-O    Optimal Ate pairing.")
message("      PP_METHD=R-ATE    R-ate pairing.")
message("      PP_METHD=X-ATE    X-ate pairing.\n")

# Choose the arithmetic methods.
if (NOT PP_METHD)
	set(PP_METHD "")
endif(NOT PP_METHD)
list(LENGTH PP_METHD PP_LEN)
if (PP_LEN LESS 0)
	message(FATAL_ERROR "Incomplete PP_METHD specification: ${PP_METHD}")
endif(PP_LEN LESS 0)

#list(GET PP_METHD 0 PP_PAIR)
set(PP_METHD ${PP_METHD} CACHE STRING "Bilinear pairings arithmetic method.")
