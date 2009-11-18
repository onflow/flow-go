message(STATUS "Cryptographic protocols configuration (CP module):\n")

message("   ** Options for the binary elliptic curve module (default = PKCS1):")
message("      CP_RSAPD=EMPTY    RSA without any padding.")
message("      CP_RSAPD=PKCS1    RSA with PKCS#1 v1.5 padding.\n")

message("   ** Available cryptographic protocols methods (default = QUICK;BASIC):")
message("      CP_METHD=BASIC    Slow RSA decryption/signature.")
message("      CP_METHD=QUICK    Fast RSA decryption/signature using CRT.\n")

message("      Note: these methods must be given in order. Ex: CP_METHD=\"QUICK;QUICK\"\n")

if (NOT CP_RSAPD)
	set(CP_RSAPD "PKCS1")
endif(NOT CP_RSAPD)
set(CP_RSAPD ${CP_RSAPD} CACHE STRING "RSA padding")	

# Choose the methods.
if (NOT CP_METHD)
	set(CP_METHD "QUICK")
endif(NOT CP_METHD)
list(LENGTH CP_METHD CP_LEN)
if (CP_LEN LESS 1)
	message(FATAL_ERROR "Incomplete CP_METHD specification: ${CP_METHD}")
endif(CP_LEN LESS 1)

list(GET CP_METHD 0 CP_RSA)
set(CP_METHD ${CP_METHD} CACHE STRING "Cryptographic protocols methods")
