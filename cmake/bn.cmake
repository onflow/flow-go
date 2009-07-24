message(STATUS "Multiple precision arithmetic configuration (BN module):\n")

message("   *** Arithmetic precision of the multiple precision module (default = 1024,DOUBLE):")
message("       BN_PRECI=n        The base precision in bits. Let w be n in words.")
message("       BN_MAGNI=DOUBLE   A multiple precision integer can store 2w words.")
message("       BN_MAGNI=CARRY    A multiple precision integer can store w+1 words.")
message("       BN_MAGNI=SINGLE   A multiple precision integer can store w words.\n")

message("   *** Available multiple precision arithmetic methods (default = COMBA;COMBA;MONTY;SLIDE;STEIN;SAFEP):")
message("       BN_METHD=BASIC    Schoolbook multiplication.")
message("       BN_METHD=COMBA    Comba multiplication.")
message("       BN_METHD=KnMUL    Karatsuba for (n > 0) steps and MUL multiplication.\n")

message("       BN_METHD=BASIC    Schoolbook squaring.")
message("       BN_METHD=COMBA    Comba squaring.")
message("       BN_METHD=KnSQR    Karatsuba for (n > 0) steps and SQR squaring.\n")

message("       BN_METHD=BASIC    Division-based modular reduction.")
message("       BN_METHD=BARRT    Barrett modular reduction.")
message("       BN_METHD=MONTY    Montgomery modular reduction.")
message("       BN_METHD=RADIX    Diminished radix modular reduction.\n")

message("       BN_METHD=BASIC    Binary modular exponentiation.")
message("       BN_METHD=CONST    Constant-time modular exponentiation.")
message("       BN_METHD=SLIDE    Sliding window modular exponentiation.\n")

message("       BN_METHD=BASIC    Euclid's standard GCD algorithm.")
message("       BN_METHD=LEHME    Lehmer's fast GCD algorithm.")
message("       BN_METHD=STEIN    Stein's binary GCD algorithm.\n")

message("       BN_METHD=BASIC    Basic prime generation.")
message("       BN_METHD=SAFEP    Safe prime generation.")
message("       BN_METHD=STRON    Strong prime generation.\n")
message("       Note: these methods must be given in order. Ex: BN_METHD=\"K1BASIC;COMBA;BARRT;CONST;SAFEP\"\n")

# Choose the arithmetic precision.
if (NOT BN_PRECI)
	set(BN_PRECI 256)
endif(NOT BN_PRECI)
set(BN_PRECI ${BN_PRECI} CACHE INTEGER "Arithmetic precision in bits")

if (NOT BN_MAGNI)
	set(BN_MAGNI "DOUBLE")
endif(NOT BN_MAGNI)
set(BN_MAGNI ${BN_MAGNI} CACHE STRING "Effective size in words")

# Choose the arithmetic methods.
if (NOT BN_METHD)
	set(BN_METHD "COMBA;COMBA;MONTY;SLIDE;BASIC;SAFEP")
endif(NOT BN_METHD)
list(LENGTH BN_METHD BN_LEN)
if (BN_LEN LESS 6)
	message(FATAL_ERROR "Incomplete BN_METHD specification: ${BN_METHD}")
endif(BN_LEN LESS 6)

list(GET BN_METHD 0 BN_MUL)
list(GET BN_METHD 1 BN_SQR)
list(GET BN_METHD 2 BN_MOD)
list(GET BN_METHD 3 BN_MXP)
list(GET BN_METHD 4 BN_GCD)
list(GET BN_METHD 5 BN_GEN)
set(BN_METHD ${BN_METHD} CACHE STRING "Multiple precision arithmetic method")

# Get the number of Karatsuba steps.
KARAT(${BN_MUL} BN_MUK BN_MUL)
KARAT(${BN_SQR} BN_SQK BN_SQR)
